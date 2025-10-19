package flyio

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"strings"
	"time"

	"github.com/superfly/fly-go"
	"github.com/superfly/fly-go/flaps"
	"github.com/superfly/fly-go/tokens"
)

// Register the fly.io provider with the global registry
func init() {
	providers.Register("flyio", func(token string) providers.Provider {
		return NewClient(token)
	})
}

type Client struct {
	token       string
	apiClient   *fly.Client
	flapsClient *flaps.Client
}

func NewClient(token string) *Client {
	apiClient := fly.NewClientFromOptions(fly.ClientOptions{
		AccessToken: token,
		BaseURL:     "https://api.fly.io",
	})

	return &Client{
		token:     token,
		apiClient: apiClient,
	}
}

func (c *Client) Name() string {
	return "flyio"
}

func (c *Client) DisplayName() string {
	return "fly.io"
}

func (c *Client) SupportsProvisioning() bool {
	return true
}

func (c *Client) SupportsBYOS() bool {
	return false
}

func (c *Client) SupportsSSH() bool {
	return false // fly.io uses container-based deployments, not SSH
}

func (c *Client) ValidateCredentials(ctx context.Context) error {
	_, err := c.apiClient.GetCurrentUser(ctx)
	if err != nil {
		return &providers.ProviderError{
			Provider: "flyio",
			Code:     "invalid_credentials",
			Message:  "Invalid fly.io API token",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}
	return nil
}

func (c *Client) GetRegions(ctx context.Context) ([]providers.Region, error) {
	regions, _, err := c.apiClient.PlatformRegions(ctx)
	if err != nil {
		// Fallback to static regions on API error
		return getStaticRegions(), nil
	}

	var result []providers.Region
	// Filter out deprecated regions
	deprecatedRegions := map[string]bool{
		"bos": true, "scl": true, "gru": true, "den": true,
	}

	for _, region := range regions {
		if deprecatedRegions[region.Code] || !region.GatewayAvailable {
			continue
		}

		result = append(result, providers.Region{
			ID:       region.Code,
			Name:     region.Name,
			Location: fmt.Sprintf("%s (%s)", region.Name, region.Code),
		})
	}

	if len(result) == 0 {
		return getStaticRegions(), nil
	}

	return result, nil
}

func (c *Client) GetSizes(ctx context.Context, region string) ([]providers.Size, error) {
	// fly.io SDK doesn't expose VM sizes via API - use static fallback
	return getStaticSizes(), nil
}

func (c *Client) GetImages(ctx context.Context) ([]providers.Image, error) {
	// fly.io uses Docker images, not traditional VPS images
	return getStaticImages(), nil
}

func (c *Client) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) {
	// fly.io doesn't use SSH keys for deployment
	// Return a dummy key for compatibility
	return &providers.SSHKey{
		ID:          "embedded",
		Name:        name,
		Fingerprint: "",
		PublicKey:   publicKey,
	}, nil
}

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
	// Get or create organization
	orgs, err := c.apiClient.GetOrganizations(ctx)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "flyio",
			Code:     "get_org_failed",
			Message:  "Failed to fetch organizations",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var orgID string
	for _, org := range orgs {
		if org.Type == "PERSONAL" {
			orgID = org.ID
			break
		}
	}

	if orgID == "" && len(orgs) > 0 {
		orgID = orgs[0].ID
	}

	if orgID == "" {
		return nil, &providers.ProviderError{
			Provider: "flyio",
			Code:     "no_org_found",
			Message:  "No organizations found for this account",
		}
	}

	// Get app name from metadata or generate one
	var appName string
	if appNameFromMeta, exists := config.Metadata["app_name"]; exists && appNameFromMeta != "" {
		appName = appNameFromMeta
	} else {
		appName = c.generateAppName(config.Name)
	}

	// Check if app already exists
	existingApp, err := c.apiClient.GetAppCompact(ctx, appName)
	if err == nil && existingApp != nil {
		// App exists, get its machine
		return c.getExistingServerInfo(ctx, appName)
	}

	// Create app
	input := fly.CreateAppInput{
		Name:           appName,
		OrganizationID: orgID,
	}

	_, err = c.apiClient.CreateApp(ctx, input)
	if err != nil {
		// Try with a new name if name collision
		if strings.Contains(err.Error(), "already been taken") {
			appName = c.generateAppName(config.Name)
			input.Name = appName
			_, err = c.apiClient.CreateApp(ctx, input)
			if err != nil {
				return nil, &providers.ProviderError{
					Provider: "flyio",
					Code:     "create_app_failed",
					Message:  fmt.Sprintf("Failed to create fly.io app after retry: %s", err.Error()),
					Details:  map[string]interface{}{"app_name": appName},
				}
			}
		} else {
			return nil, &providers.ProviderError{
				Provider: "flyio",
				Code:     "create_app_failed",
				Message:  fmt.Sprintf("Failed to create fly.io app: %s", err.Error()),
				Details:  map[string]interface{}{"app_name": appName},
			}
		}
	}

	// Allocate shared IPv4
	ipAddress, err := c.apiClient.AllocateSharedIPAddress(ctx, appName)
	if err != nil {
		// Non-fatal - we can get IP later
		ipAddress = nil
	}

	// For now, return server info without machine (machine will be created during deployment)
	server := &providers.Server{
		ID:         fmt.Sprintf("%s:pending", appName),
		Name:       appName,
		Status:     "created",
		PublicIPv4: "",
		Region:     config.Region,
		Size:       config.Size,
		Image:      config.Image,
		CreatedAt:  time.Now(),
		Metadata: map[string]string{
			"organization_id": orgID,
			"app_name":        appName,
		},
	}

	if ipAddress != nil {
		server.PublicIPv4 = ipAddress.String()
	}

	return server, nil
}

func (c *Client) GetServer(ctx context.Context, serverID string) (*providers.Server, error) {
	// Parse app name and machine ID from serverID (format: "app-name:machine-id" or "app-name:pending")
	parts := strings.Split(serverID, ":")
	if len(parts) != 2 {
		return nil, &providers.ProviderError{
			Provider: "flyio",
			Code:     "invalid_server_id",
			Message:  "Invalid server ID format. Expected 'app-name:machine-id'",
			Details:  map[string]interface{}{"server_id": serverID},
		}
	}

	appName := parts[0]
	machineID := parts[1]

	// If machine ID is "pending", just return app info
	if machineID == "pending" {
		_, err := c.apiClient.GetAppCompact(ctx, appName)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "flyio",
				Code:     "get_app_failed",
				Message:  fmt.Sprintf("Failed to get app '%s': %s", appName, err.Error()),
			}
		}

		return &providers.Server{
			ID:        serverID,
			Name:      appName,
			Status:    "pending",
			CreatedAt: time.Now(),
			Metadata:  map[string]string{"app_name": appName},
		}, nil
	}

	// Get machine info via flaps client
	flapsClient, err := flaps.NewWithOptions(ctx, flaps.NewClientOpts{
		AppName: appName,
		Tokens:  tokens.Parse(c.token),
	})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "flyio",
			Code:     "flaps_client_failed",
			Message:  fmt.Sprintf("Failed to create flaps client: %s", err.Error()),
		}
	}

	machine, err := flapsClient.Get(ctx, machineID)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "flyio",
			Code:     "get_machine_failed",
			Message:  fmt.Sprintf("Failed to get machine: %s", err.Error()),
		}
	}

	// Parse CreatedAt string to time.Time
	createdAt, _ := time.Parse(time.RFC3339, machine.CreatedAt)

	return &providers.Server{
		ID:          serverID,
		Name:        machine.Name,
		Status:      machine.State,
		PrivateIPv4: machine.PrivateIP,
		Region:      machine.Region,
		Image:       machine.Config.Image,
		CreatedAt:   createdAt,
		Metadata:    map[string]string{"app_name": appName},
	}, nil
}

func (c *Client) Destroy(ctx context.Context, serverID string) error {
	// Parse app name and machine ID
	parts := strings.Split(serverID, ":")
	if len(parts) != 2 {
		return &providers.ProviderError{
			Provider: "flyio",
			Code:     "invalid_server_id",
			Message:  "Invalid server ID format. Expected 'app-name:machine-id'",
			Details:  map[string]interface{}{"server_id": serverID},
		}
	}

	appName := parts[0]
	machineID := parts[1]

	// Create flaps client
	flapsClient, err := flaps.NewWithOptions(ctx, flaps.NewClientOpts{
		AppName: appName,
		Tokens:  tokens.Parse(c.token),
	})
	if err != nil {
		// If flaps client fails, try to delete app directly
		return c.deleteApp(ctx, appName)
	}

	// Step 1: Stop machine if it exists
	if machineID != "pending" {
		_ = flapsClient.Stop(ctx, fly.StopMachineInput{ID: machineID}, "")

		// Step 2: Delete machine
		err = flapsClient.Destroy(ctx, fly.RemoveMachineInput{ID: machineID}, "")
		if err != nil && !strings.Contains(err.Error(), "not found") {
			// Non-fatal - continue to delete app
		}
	}

	// Step 3: Release IPs (best effort)
	_ = c.releaseAppIPs(ctx, appName)

	// Step 4: Delete app
	return c.deleteApp(ctx, appName)
}

func (c *Client) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*providers.Server, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		server, err := c.GetServer(ctx, serverID)
		if err != nil {
			return nil, err
		}

		if server.Status == "started" || server.Status == "running" {
			return server, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			continue
		}
	}

	return nil, &providers.ProviderError{
		Provider: "flyio",
		Code:     "timeout",
		Message:  fmt.Sprintf("Timeout waiting for machine to become active (waited %s)", timeout.String()),
		Details:  map[string]interface{}{"timeout": timeout.String()},
	}
}

// Helper functions

func (c *Client) generateAppName(projectName string) string {
	name := strings.ToLower(projectName)

	// Replace invalid characters with hyphens
	validChars := "abcdefghijklmnopqrstuvwxyz0123456789-"
	var sanitized strings.Builder
	for _, char := range name {
		if strings.ContainsRune(validChars, char) {
			sanitized.WriteRune(char)
		} else {
			sanitized.WriteRune('-')
		}
	}

	name = sanitized.String()
	name = strings.Trim(name, "-")

	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	if name == "" {
		name = "lightfold-app"
	}

	// Add timestamp suffix for uniqueness
	now := time.Now()
	suffix := fmt.Sprintf("-%d%03d", now.Unix()%100000, now.Nanosecond()/1000000)

	maxBaseLen := 30 - len(suffix)
	if len(name) > maxBaseLen {
		name = name[:maxBaseLen]
	}

	return name + suffix
}

func (c *Client) getExistingServerInfo(ctx context.Context, appName string) (*providers.Server, error) {
	_, err := c.apiClient.GetAppCompact(ctx, appName)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "flyio",
			Code:     "get_app_failed",
			Message:  fmt.Sprintf("Failed to get app: %s", err.Error()),
		}
	}

	// Try to get IP
	ipAddresses, _ := c.apiClient.GetIPAddresses(ctx, appName)
	var publicIP string
	if len(ipAddresses) > 0 {
		for _, ip := range ipAddresses {
			if ip.Type == "v4" || ip.Type == "shared_v4" {
				publicIP = ip.Address
				break
			}
		}
	}

	return &providers.Server{
		ID:         fmt.Sprintf("%s:pending", appName),
		Name:       appName,
		Status:     "created",
		PublicIPv4: publicIP,
		CreatedAt:  time.Now(),
		Metadata:   map[string]string{"app_name": appName},
	}, nil
}

func (c *Client) releaseAppIPs(ctx context.Context, appName string) error {
	ips, err := c.apiClient.GetIPAddresses(ctx, appName)
	if err != nil {
		// Ignore "not found" errors - app may already be deleted
		if strings.Contains(err.Error(), "Could not find App") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "404") {
			return nil
		}
		return err
	}

	for _, ip := range ips {
		_ = c.apiClient.ReleaseIPAddress(ctx, appName, ip.ID)
	}

	return nil
}

func (c *Client) deleteApp(ctx context.Context, appName string) error {
	err := c.apiClient.DeleteApp(ctx, appName)
	if err != nil {
		// Return "not found" error instead of nil so destroy command can show proper message
		if strings.Contains(err.Error(), "Could not find App") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "404") {
			return &providers.ProviderError{
				Provider: "flyio",
				Code:     "not_found",
				Message:  "App not found (may have been deleted manually)",
				Details:  map[string]interface{}{"app_name": appName},
			}
		}
		return &providers.ProviderError{
			Provider: "flyio",
			Code:     "delete_app_failed",
			Message:  fmt.Sprintf("Failed to delete app '%s': %s", appName, err.Error()),
			Details:  map[string]interface{}{"app_name": appName},
		}
	}
	return nil
}

// GetAppIP retrieves the IP address for an app (used for backward compatibility)
func (c *Client) GetAppIP(ctx context.Context, appName string) (string, error) {
	ipAddresses, err := c.apiClient.GetIPAddresses(ctx, appName)
	if err != nil {
		return "", &providers.ProviderError{
			Provider: "flyio",
			Code:     "get_ip_failed",
			Message:  fmt.Sprintf("Failed to get IP addresses: %s", err.Error()),
		}
	}

	for _, ip := range ipAddresses {
		if ip.Type == "v4" || ip.Type == "shared_v4" {
			return ip.Address, nil
		}
	}

	return "", &providers.ProviderError{
		Provider: "flyio",
		Code:     "no_ip_found",
		Message:  "No IPv4 address found for app",
	}
}

// Static fallback data

func getStaticRegions() []providers.Region {
	return []providers.Region{
		{ID: "sjc", Name: "San Jose, California (US)", Location: "San Jose (SJC)"},
		{ID: "iad", Name: "Ashburn, Virginia (US)", Location: "Ashburn (IAD)"},
		{ID: "ord", Name: "Chicago, Illinois (US)", Location: "Chicago (ORD)"},
		{ID: "dfw", Name: "Dallas, Texas (US)", Location: "Dallas (DFW)"},
		{ID: "sea", Name: "Seattle, Washington (US)", Location: "Seattle (SEA)"},
		{ID: "lax", Name: "Los Angeles, California (US)", Location: "Los Angeles (LAX)"},
		{ID: "ewr", Name: "Secaucus, NJ (US)", Location: "Secaucus (EWR)"},
		{ID: "lhr", Name: "London, United Kingdom", Location: "London (LHR)"},
		{ID: "fra", Name: "Frankfurt, Germany", Location: "Frankfurt (FRA)"},
		{ID: "ams", Name: "Amsterdam, Netherlands", Location: "Amsterdam (AMS)"},
		{ID: "nrt", Name: "Tokyo, Japan", Location: "Tokyo (NRT)"},
		{ID: "hkg", Name: "Hong Kong", Location: "Hong Kong (HKG)"},
		{ID: "syd", Name: "Sydney, Australia", Location: "Sydney (SYD)"},
		{ID: "sin", Name: "Singapore", Location: "Singapore (SIN)"},
	}
}

func getStaticSizes() []providers.Size {
	return []providers.Size{
		{ID: "shared-cpu-1x", Name: "Shared CPU 1x (256 MB RAM, 1 vCPU)", Memory: 256, VCPUs: 1, Disk: 3, PriceMonthly: 5.0, PriceHourly: 0.007},
		{ID: "shared-cpu-2x", Name: "Shared CPU 2x (512 MB RAM, 1 vCPU)", Memory: 512, VCPUs: 1, Disk: 5, PriceMonthly: 10.0, PriceHourly: 0.014},
		{ID: "shared-cpu-4x", Name: "Shared CPU 4x (1 GB RAM, 2 vCPUs)", Memory: 1024, VCPUs: 2, Disk: 10, PriceMonthly: 20.0, PriceHourly: 0.027},
		{ID: "shared-cpu-8x", Name: "Shared CPU 8x (2 GB RAM, 4 vCPUs)", Memory: 2048, VCPUs: 4, Disk: 20, PriceMonthly: 40.0, PriceHourly: 0.055},
		{ID: "performance-1x", Name: "Performance 1x (2 GB RAM, 1 vCPU)", Memory: 2048, VCPUs: 1, Disk: 50, PriceMonthly: 60.0, PriceHourly: 0.082},
		{ID: "performance-2x", Name: "Performance 2x (4 GB RAM, 2 vCPUs)", Memory: 4096, VCPUs: 2, Disk: 100, PriceMonthly: 120.0, PriceHourly: 0.164},
		{ID: "performance-4x", Name: "Performance 4x (8 GB RAM, 4 vCPUs)", Memory: 8192, VCPUs: 4, Disk: 200, PriceMonthly: 240.0, PriceHourly: 0.329},
	}
}

func getStaticImages() []providers.Image {
	return []providers.Image{
		{ID: providers.GetDefaultImage("flyio"), Name: "Ubuntu 22.04 LTS", Distribution: "Ubuntu", Version: "22.04"},
		{ID: "ubuntu:20.04", Name: "Ubuntu 20.04 LTS", Distribution: "Ubuntu", Version: "20.04"},
		{ID: "ubuntu:24.04", Name: "Ubuntu 24.04 LTS", Distribution: "Ubuntu", Version: "24.04"},
	}
}
