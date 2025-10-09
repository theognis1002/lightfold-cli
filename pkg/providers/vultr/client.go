package vultr

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vultr/govultr/v3"
	"golang.org/x/oauth2"
)

// Register the Vultr provider with the global registry
func init() {
	providers.Register("vultr", func(token string) providers.Provider {
		return NewClient(token)
	})
}

type Client struct {
	client *govultr.Client
	token  string
}

func NewClient(token string) *Client {
	config := &oauth2.Config{}
	ctx := context.Background()
	ts := config.TokenSource(ctx, &oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	client := govultr.NewClient(httpClient)

	return &Client{
		client: client,
		token:  token,
	}
}

func (c *Client) Name() string {
	return "vultr"
}

func (c *Client) DisplayName() string {
	return "Vultr"
}

func (c *Client) SupportsProvisioning() bool {
	return true
}

func (c *Client) SupportsBYOS() bool {
	return true
}

func (c *Client) SupportsSSH() bool {
	return true
}

func (c *Client) ValidateCredentials(ctx context.Context) error {
	_, _, err := c.client.Account.Get(ctx)
	if err != nil {
		providerErr := &providers.ProviderError{
			Provider: "vultr",
			Code:     "invalid_credentials",
			Message:  "Invalid Vultr API token",
			Details:  map[string]interface{}{"error": err.Error()},
		}
		return providerErr
	}
	return nil
}

func (c *Client) GetRegions(ctx context.Context) ([]providers.Region, error) {
	vultrRegions, _, _, err := c.client.Region.List(ctx, nil)
	if err != nil {
		// Return static fallback on error
		return getStaticRegions(), nil
	}

	var regions []providers.Region
	for _, region := range vultrRegions {
		regions = append(regions, providers.Region{
			ID:       region.ID,
			Name:     region.City,
			Location: fmt.Sprintf("%s, %s", region.City, region.Country),
		})
	}

	if len(regions) == 0 {
		return getStaticRegions(), nil
	}

	return regions, nil
}

func (c *Client) GetSizes(ctx context.Context, region string) ([]providers.Size, error) {
	vultrPlans, _, _, err := c.client.Plan.List(ctx, "", nil)
	if err != nil {
		return getStaticPlans(), nil
	}

	var sizes []providers.Size
	for _, plan := range vultrPlans {
		// Filter minimum 512MB RAM (same as DO/Hetzner)
		if plan.RAM >= 512 {
			sizes = append(sizes, providers.Size{
				ID:           plan.ID,
				Name:         fmt.Sprintf("%s (%d MB RAM, %d vCPUs, %d GB disk)", plan.ID, plan.RAM, plan.VCPUCount, plan.Disk),
				Memory:       plan.RAM,
				VCPUs:        plan.VCPUCount,
				Disk:         plan.Disk,
				PriceMonthly: float64(plan.MonthlyCost),
				PriceHourly:  0, // Vultr doesn't provide hourly in plan list
			})
		}
	}

	if len(sizes) == 0 {
		return getStaticPlans(), nil
	}

	// Sort by memory (smallest first)
	sort.Slice(sizes, func(i, j int) bool {
		return sizes[i].Memory < sizes[j].Memory
	})

	return sizes, nil
}

func (c *Client) GetImages(ctx context.Context) ([]providers.Image, error) {
	vultrOSList, _, _, err := c.client.OS.List(ctx, nil)
	if err != nil {
		return getStaticImages(), nil
	}

	var images []providers.Image
	for _, os := range vultrOSList {
		if strings.Contains(strings.ToLower(os.Name), "ubuntu") {
			images = append(images, providers.Image{
				ID:           strconv.Itoa(os.ID),
				Name:         os.Name,
				Distribution: "Ubuntu",
				Version:      extractVersionFromImageName(os.Name),
			})
		}
	}

	if len(images) == 0 {
		return getStaticImages(), nil
	}

	return images, nil
}

func (c *Client) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) {
	keyReq := &govultr.SSHKeyReq{
		Name:   name,
		SSHKey: publicKey,
	}

	key, _, err := c.client.SSHKey.Create(ctx, keyReq)
	if err != nil {
		// Check if key already exists
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate") {
			keys, _, _, listErr := c.client.SSHKey.List(ctx, nil)
			if listErr == nil {
				for _, k := range keys {
					if k.Name == name {
						return &providers.SSHKey{
							ID:          k.ID,
							Name:        k.Name,
							Fingerprint: k.SSHKey, // Vultr doesn't provide fingerprint separately
							PublicKey:   k.SSHKey,
						}, nil
					}
				}
			}
		}

		return nil, &providers.ProviderError{
			Provider: "vultr",
			Code:     "upload_ssh_key_failed",
			Message:  fmt.Sprintf("Failed to upload SSH key to Vultr: %v", err),
			Details:  map[string]interface{}{"error": err.Error(), "name": name},
		}
	}

	return &providers.SSHKey{
		ID:          key.ID,
		Name:        key.Name,
		Fingerprint: key.SSHKey,
		PublicKey:   key.SSHKey,
	}, nil
}

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
	instanceReq := &govultr.InstanceCreateReq{
		Region:   config.Region,
		Plan:     config.Size,
		OsID:     0, // Will be set below
		Label:    config.Name,
		SSHKeys:  config.SSHKeys,
		UserData: config.UserData,
		Tags:     config.Tags,
		Backups:  "disabled",
	}

	if config.BackupsEnabled {
		instanceReq.Backups = "enabled"
	}

	// Convert image ID string to int
	osID, err := strconv.Atoi(config.Image)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "vultr",
			Code:     "invalid_image",
			Message:  fmt.Sprintf("Invalid image ID: %s", config.Image),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}
	instanceReq.OsID = osID

	instance, _, err := c.client.Instance.Create(ctx, instanceReq)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "vultr",
			Code:     "create_instance_failed",
			Message:  "Failed to create Vultr instance",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return convertInstanceToServer(instance), nil
}

func (c *Client) GetServer(ctx context.Context, serverID string) (*providers.Server, error) {
	instance, _, err := c.client.Instance.Get(ctx, serverID)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "vultr",
			Code:     "get_instance_failed",
			Message:  "Failed to get Vultr instance",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return convertInstanceToServer(instance), nil
}

func (c *Client) Destroy(ctx context.Context, serverID string) error {
	err := c.client.Instance.Delete(ctx, serverID)
	if err != nil {
		return &providers.ProviderError{
			Provider: "vultr",
			Code:     "destroy_instance_failed",
			Message:  "Failed to destroy Vultr instance",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return nil
}

func (c *Client) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*providers.Server, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		instance, _, err := c.client.Instance.Get(ctx, serverID)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "vultr",
				Code:     "poll_instance_failed",
				Message:  "Failed to poll Vultr instance status",
				Details:  map[string]interface{}{"error": err.Error()},
			}
		}

		if instance.Status == "active" {
			return convertInstanceToServer(instance), nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			continue
		}
	}

	return nil, &providers.ProviderError{
		Provider: "vultr",
		Code:     "timeout",
		Message:  fmt.Sprintf("Timeout waiting for instance to become active (waited %s)", timeout.String()),
		Details:  map[string]interface{}{"timeout": timeout.String()},
	}
}

func convertInstanceToServer(instance *govultr.Instance) *providers.Server {
	var publicIPv4, privateIPv4 string

	if instance.MainIP != "" {
		publicIPv4 = instance.MainIP
	}

	if instance.InternalIP != "" {
		privateIPv4 = instance.InternalIP
	}

	metadata := map[string]string{
		"vcpus":  strconv.Itoa(instance.VCPUCount),
		"memory": strconv.Itoa(instance.RAM),
		"disk":   strconv.Itoa(instance.Disk),
		"locked": strconv.FormatBool(false), // Vultr doesn't have a locked field
	}

	if instance.Features != nil {
		metadata["features"] = strings.Join(instance.Features, ",")
	}

	// Parse DateCreated string to time.Time
	createdAt, err := time.Parse(time.RFC3339, instance.DateCreated)
	if err != nil {
		createdAt = time.Now()
	}

	return &providers.Server{
		ID:          instance.ID,
		Name:        instance.Label,
		Status:      instance.Status,
		PublicIPv4:  publicIPv4,
		PrivateIPv4: privateIPv4,
		Region:      instance.Region,
		Size:        instance.Plan,
		Image:       strconv.Itoa(instance.OsID),
		Tags:        instance.Tags,
		CreatedAt:   createdAt,
		Metadata:    metadata,
	}
}

func extractVersionFromImageName(imageName string) string {
	parts := strings.Fields(imageName)
	for _, part := range parts {
		if strings.Contains(part, ".") && len(part) <= 6 {
			return part
		}
	}
	return ""
}

// getStaticRegions returns fallback regions when API is unavailable
func getStaticRegions() []providers.Region {
	return []providers.Region{
		{ID: "ewr", Name: "New Jersey", Location: "New York (EWR)"},
		{ID: "ord", Name: "Chicago", Location: "Chicago (ORD)"},
		{ID: "dfw", Name: "Dallas", Location: "Dallas (DFW)"},
		{ID: "sea", Name: "Seattle", Location: "Seattle (SEA)"},
		{ID: "lax", Name: "Los Angeles", Location: "Los Angeles (LAX)"},
		{ID: "atl", Name: "Atlanta", Location: "Atlanta (ATL)"},
		{ID: "ams", Name: "Amsterdam", Location: "Amsterdam (AMS)"},
		{ID: "lhr", Name: "London", Location: "London (LHR)"},
		{ID: "fra", Name: "Frankfurt", Location: "Frankfurt (FRA)"},
		{ID: "sjc", Name: "Silicon Valley", Location: "Silicon Valley (SJC)"},
		{ID: "syd", Name: "Sydney", Location: "Sydney (SYD)"},
		{ID: "nrt", Name: "Tokyo", Location: "Tokyo (NRT)"},
		{ID: "sgp", Name: "Singapore", Location: "Singapore (SGP)"},
	}
}

// getStaticPlans returns fallback plans when API is unavailable
func getStaticPlans() []providers.Size {
	return []providers.Size{
		{ID: "vc2-1c-1gb", Name: "1 vCPU, 1 GB RAM, 25 GB SSD", Memory: 1024, VCPUs: 1, Disk: 25, PriceMonthly: 6.0, PriceHourly: 0.009},
		{ID: "vc2-1c-2gb", Name: "1 vCPU, 2 GB RAM, 55 GB SSD", Memory: 2048, VCPUs: 1, Disk: 55, PriceMonthly: 12.0, PriceHourly: 0.018},
		{ID: "vc2-2c-4gb", Name: "2 vCPUs, 4 GB RAM, 80 GB SSD", Memory: 4096, VCPUs: 2, Disk: 80, PriceMonthly: 24.0, PriceHourly: 0.036},
		{ID: "vc2-4c-8gb", Name: "4 vCPUs, 8 GB RAM, 160 GB SSD", Memory: 8192, VCPUs: 4, Disk: 160, PriceMonthly: 48.0, PriceHourly: 0.071},
		{ID: "vc2-6c-16gb", Name: "6 vCPUs, 16 GB RAM, 320 GB SSD", Memory: 16384, VCPUs: 6, Disk: 320, PriceMonthly: 96.0, PriceHourly: 0.143},
	}
}

// getStaticImages returns fallback images when API is unavailable
func getStaticImages() []providers.Image {
	return []providers.Image{
		{ID: "1743", Name: "Ubuntu 22.04 LTS x64", Distribution: "Ubuntu", Version: "22.04"},
		{ID: "387", Name: "Ubuntu 20.04 LTS x64", Distribution: "Ubuntu", Version: "20.04"},
	}
}

// GetStaticRegions exports static regions for testing
func GetStaticRegions() []providers.Region {
	return getStaticRegions()
}

// GetStaticPlans exports static plans for testing
func GetStaticPlans() []providers.Size {
	return getStaticPlans()
}

// GetStaticImages exports static images for testing
func GetStaticImages() []providers.Image {
	return getStaticImages()
}
