package digitalocean

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

// Client implements the Provider interface for DigitalOcean
type Client struct {
	client *godo.Client
	token  string
}

// TokenSource implements oauth2.TokenSource for DigitalOcean API
type TokenSource struct {
	AccessToken string
}

// Token returns the oauth2 token
func (t *TokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: t.AccessToken,
	}, nil
}

// NewClient creates a new DigitalOcean provider client
func NewClient(token string) *Client {
	tokenSource := &TokenSource{AccessToken: token}
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	client := godo.NewClient(oauthClient)

	return &Client{
		client: client,
		token:  token,
	}
}

// Name returns the provider name
func (c *Client) Name() string {
	return "digitalocean"
}

// ValidateCredentials validates the API token by making a simple API call
func (c *Client) ValidateCredentials(ctx context.Context) error {
	_, _, err := c.client.Account.Get(ctx)
	if err != nil {
		return &providers.ProviderError{
			Provider: "digitalocean",
			Code:     "invalid_credentials",
			Message:  "Invalid DigitalOcean API token",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}
	return nil
}

// GetRegions returns available DigitalOcean regions
func (c *Client) GetRegions(ctx context.Context) ([]providers.Region, error) {
	doRegions, _, err := c.client.Regions.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "digitalocean",
			Code:     "list_regions_failed",
			Message:  "Failed to list DigitalOcean regions",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var regions []providers.Region
	for _, region := range doRegions {
		if region.Available {
			regions = append(regions, providers.Region{
				ID:       region.Slug,
				Name:     region.Name,
				Location: fmt.Sprintf("%s, %s", region.Name, region.Slug),
			})
		}
	}

	return regions, nil
}

// GetSizes returns available droplet sizes for a region
func (c *Client) GetSizes(ctx context.Context, region string) ([]providers.Size, error) {
	doSizes, _, err := c.client.Sizes.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "digitalocean",
			Code:     "list_sizes_failed",
			Message:  "Failed to list DigitalOcean sizes",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var sizes []providers.Size
	for _, size := range doSizes {
		// Filter out sizes that aren't available or suitable for web apps
		if size.Available && size.Memory >= 512 {
			sizes = append(sizes, providers.Size{
				ID:           size.Slug,
				Name:         fmt.Sprintf("%s (%d MB RAM, %d vCPUs, %d GB disk)", size.Slug, size.Memory, size.Vcpus, size.Disk),
				Memory:       size.Memory,
				VCPUs:        size.Vcpus,
				Disk:         size.Disk,
				PriceMonthly: size.PriceMonthly,
				PriceHourly:  size.PriceHourly,
			})
		}
	}

	return sizes, nil
}

// GetImages returns available OS images
func (c *Client) GetImages(ctx context.Context) ([]providers.Image, error) {
	// Get official distribution images
	doImages, _, err := c.client.Images.ListDistribution(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "digitalocean",
			Code:     "list_images_failed",
			Message:  "Failed to list DigitalOcean images",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var images []providers.Image
	for _, image := range doImages {
		// Filter for Ubuntu images (most common for web apps)
		if image.Distribution == "Ubuntu" && image.Public {
			images = append(images, providers.Image{
				ID:           image.Slug,
				Name:         fmt.Sprintf("%s %s", image.Distribution, image.Name),
				Distribution: image.Distribution,
				Version:      image.Name,
			})
		}
	}

	// Add default Ubuntu 22.04 if not found
	if len(images) == 0 {
		images = append(images, providers.Image{
			ID:           "ubuntu-22-04-x64",
			Name:         "Ubuntu 22.04 LTS",
			Distribution: "Ubuntu",
			Version:      "22.04",
		})
	}

	return images, nil
}

func (c *Client) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) {
	keyRequest := &godo.KeyCreateRequest{
		Name:      name,
		PublicKey: publicKey,
	}

	key, resp, err := c.client.Keys.Create(ctx, keyRequest)
	if err != nil {
		// If key already exists, try to find it
		if resp != nil && (resp.StatusCode == 422 || strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate")) {
			// List all keys and find the matching one
			keys, _, listErr := c.client.Keys.List(ctx, &godo.ListOptions{})
			if listErr == nil {
				for _, k := range keys {
					if k.Name == name {
						return &providers.SSHKey{
							ID:          fmt.Sprintf("%d", k.ID),
							Name:        k.Name,
							Fingerprint: k.Fingerprint,
							PublicKey:   k.PublicKey,
						}, nil
					}
				}
			}
		}

		errDetails := map[string]interface{}{
			"error": err.Error(),
			"name":  name,
		}
		if resp != nil {
			errDetails["status_code"] = resp.StatusCode
		}
		return nil, &providers.ProviderError{
			Provider: "digitalocean",
			Code:     "upload_ssh_key_failed",
			Message:  fmt.Sprintf("Failed to upload SSH key to DigitalOcean: %v", err),
			Details:  errDetails,
		}
	}

	return &providers.SSHKey{
		ID:          fmt.Sprintf("%d", key.ID),
		Name:        key.Name,
		Fingerprint: key.Fingerprint,
		PublicKey:   key.PublicKey,
	}, nil
}

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
	var sshKeys []godo.DropletCreateSSHKey
	for _, keyID := range config.SSHKeys {
		keyIDInt := getKeyID(keyID)
		sshKeys = append(sshKeys, godo.DropletCreateSSHKey{
			ID: keyIDInt,
		})
	}

	dropletRequest := &godo.DropletCreateRequest{
		Name:              config.Name,
		Region:            config.Region,
		Size:              config.Size,
		Image:             godo.DropletCreateImage{Slug: config.Image},
		SSHKeys:           sshKeys,
		UserData:          config.UserData,
		Tags:              config.Tags,
		Backups:           config.BackupsEnabled,
		Monitoring:        config.MonitoringEnabled,
	}

	droplet, _, err := c.client.Droplets.Create(ctx, dropletRequest)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "digitalocean",
			Code:     "create_droplet_failed",
			Message:  "Failed to create DigitalOcean droplet",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return convertDropletToServer(droplet), nil
}

func (c *Client) GetServer(ctx context.Context, serverID string) (*providers.Server, error) {
	dropletID := getDropletID(serverID)

	droplet, _, err := c.client.Droplets.Get(ctx, dropletID)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "digitalocean",
			Code:     "get_droplet_failed",
			Message:  "Failed to get DigitalOcean droplet",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return convertDropletToServer(droplet), nil
}

func (c *Client) Destroy(ctx context.Context, serverID string) error {
	dropletID := getDropletID(serverID)

	_, err := c.client.Droplets.Delete(ctx, dropletID)
	if err != nil {
		return &providers.ProviderError{
			Provider: "digitalocean",
			Code:     "destroy_droplet_failed",
			Message:  "Failed to destroy DigitalOcean droplet",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return nil
}

func (c *Client) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*providers.Server, error) {
	dropletID := getDropletID(serverID)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		droplet, _, err := c.client.Droplets.Get(ctx, dropletID)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "digitalocean",
				Code:     "poll_droplet_failed",
				Message:  "Failed to poll DigitalOcean droplet status",
				Details:  map[string]interface{}{"error": err.Error()},
			}
		}

		if droplet.Status == "active" {
			return convertDropletToServer(droplet), nil
		}

		// Wait before next poll
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			continue
		}
	}

	return nil, &providers.ProviderError{
		Provider: "digitalocean",
		Code:     "timeout",
		Message:  "Timeout waiting for droplet to become active",
		Details:  map[string]interface{}{"timeout": timeout.String()},
	}
}

// Helper functions

func convertDropletToServer(droplet *godo.Droplet) *providers.Server {
	var publicIPv4, privateIPv4 string

	for _, network := range droplet.Networks.V4 {
		if network.Type == "public" {
			publicIPv4 = network.IPAddress
		} else if network.Type == "private" {
			privateIPv4 = network.IPAddress
		}
	}

	createdAt, err := time.Parse(time.RFC3339, droplet.Created)
	if err != nil {
		createdAt = time.Now()
	}

	metadata := map[string]string{
		"vcpus":        fmt.Sprintf("%d", droplet.Vcpus),
		"memory":       fmt.Sprintf("%d", droplet.Memory),
		"disk":         fmt.Sprintf("%d", droplet.Disk),
		"locked":       fmt.Sprintf("%t", droplet.Locked),
		"backup_ids":   fmt.Sprintf("%v", droplet.BackupIDs),
		"snapshot_ids": fmt.Sprintf("%v", droplet.SnapshotIDs),
	}

	// Kernel may be nil for some droplets
	if droplet.Kernel != nil {
		metadata["kernel"] = droplet.Kernel.Name
	}

	// Region may also be nil
	regionSlug := ""
	if droplet.Region != nil {
		regionSlug = droplet.Region.Slug
	}

	// Size may also be nil
	sizeSlug := ""
	if droplet.Size != nil {
		sizeSlug = droplet.Size.Slug
	}

	// Image may also be nil
	imageSlug := ""
	if droplet.Image != nil {
		imageSlug = droplet.Image.Slug
	}

	return &providers.Server{
		ID:          fmt.Sprintf("%d", droplet.ID),
		Name:        droplet.Name,
		Status:      droplet.Status,
		PublicIPv4:  publicIPv4,
		PrivateIPv4: privateIPv4,
		Region:      regionSlug,
		Size:        sizeSlug,
		Image:       imageSlug,
		Tags:        droplet.Tags,
		CreatedAt:   createdAt,
		Metadata:    metadata,
	}
}

func getDropletID(serverID string) int {
	id, err := strconv.Atoi(serverID)
	if err != nil {
		return 0
	}
	return id
}

func getKeyID(keyIDStr string) int {
	id, err := strconv.Atoi(keyIDStr)
	if err != nil {
		return 0
	}
	return id
}