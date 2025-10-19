package linode

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"strconv"
	"strings"
	"time"

	"github.com/linode/linodego"
	"golang.org/x/oauth2"
)

// Register the Linode provider with the global registry
func init() {
	providers.Register("linode", func(token string) providers.Provider {
		return NewClient(token)
	})
}

type Client struct {
	client *linodego.Client
}

func NewClient(token string) *Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	oauth2Client := oauth2.NewClient(context.Background(), tokenSource)

	linodeClient := linodego.NewClient(oauth2Client)

	return &Client{
		client: &linodeClient,
	}
}

func (c *Client) Name() string {
	return "linode"
}

func (c *Client) DisplayName() string {
	return "Linode"
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
	_, err := c.client.ListRegions(ctx, &linodego.ListOptions{})
	if err != nil {
		return &providers.ProviderError{
			Provider: "linode",
			Code:     "invalid_credentials",
			Message:  "Invalid Linode API token",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}
	return nil
}

func (c *Client) GetRegions(ctx context.Context) ([]providers.Region, error) {
	linodeRegions, err := c.client.ListRegions(ctx, &linodego.ListOptions{})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "linode",
			Code:     "list_regions_failed",
			Message:  "Failed to list Linode regions",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var regions []providers.Region
	for _, region := range linodeRegions {
		if region.Status == "ok" {
			regions = append(regions, providers.Region{
				ID:       region.ID,
				Name:     region.Label,
				Location: fmt.Sprintf("%s (%s)", region.Label, region.Country),
			})
		}
	}

	return regions, nil
}

func (c *Client) GetSizes(ctx context.Context, _ string) ([]providers.Size, error) {
	linodeTypes, err := c.client.ListTypes(ctx, &linodego.ListOptions{})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "linode",
			Code:     "list_sizes_failed",
			Message:  "Failed to list Linode types",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var sizes []providers.Size
	for _, t := range linodeTypes {
		if t.Memory >= 512 {
			sizes = append(sizes, providers.Size{
				ID:           t.ID,
				Name:         fmt.Sprintf("%s (%d MB RAM, %d vCPUs, %d GB disk)", t.Label, t.Memory, t.VCPUs, t.Disk),
				Memory:       t.Memory,
				VCPUs:        t.VCPUs,
				Disk:         t.Disk,
				PriceMonthly: float64(t.Price.Monthly),
				PriceHourly:  float64(t.Price.Hourly),
			})
		}
	}

	return sizes, nil
}

func (c *Client) GetImages(ctx context.Context) ([]providers.Image, error) {
	images, err := c.client.ListImages(ctx, &linodego.ListOptions{})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "linode",
			Code:     "list_images_failed",
			Message:  "Failed to list Linode images",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var providerImages []providers.Image
	for _, image := range images {
		label := strings.ToLower(image.Label)
		if strings.Contains(label, "ubuntu") &&
			image.IsPublic &&
			image.Status == "available" &&
			strings.HasPrefix(image.ID, "linode/") {
			providerImages = append(providerImages, providers.Image{
				ID:           image.ID,
				Name:         image.Label,
				Distribution: "Ubuntu",
				Version:      extractVersionFromLabel(image.Label),
			})
		}
	}

	// Fallback to known stable Ubuntu LTS images if none found
	if len(providerImages) == 0 {
		providerImages = []providers.Image{
			{
				ID:           "linode/ubuntu22.04",
				Name:         "Ubuntu 22.04 LTS",
				Distribution: "Ubuntu",
				Version:      "22.04",
			},
			{
				ID:           "linode/ubuntu24.04",
				Name:         "Ubuntu 24.04 LTS",
				Distribution: "Ubuntu",
				Version:      "24.04",
			},
		}
	}

	return providerImages, nil
}

func (c *Client) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) {
	key, err := c.client.CreateSSHKey(ctx, linodego.SSHKeyCreateOptions{
		Label:  name,
		SSHKey: publicKey,
	})

	if err != nil {
		if strings.Contains(err.Error(), "already exists") ||
			strings.Contains(err.Error(), "duplicate") ||
			strings.Contains(err.Error(), "Label must be unique") {
			keys, listErr := c.client.ListSSHKeys(ctx, &linodego.ListOptions{})
			if listErr == nil {
				for _, k := range keys {
					if k.Label == name {
						return &providers.SSHKey{
							ID:          intToString(k.ID),
							Name:        k.Label,
							Fingerprint: "", // Linode doesn't expose fingerprint in API
							PublicKey:   k.SSHKey,
						}, nil
					}
				}
			}
		}

		return nil, &providers.ProviderError{
			Provider: "linode",
			Code:     "upload_ssh_key_failed",
			Message:  fmt.Sprintf("Failed to upload SSH key to Linode: %v", err),
			Details:  map[string]interface{}{"error": err.Error(), "name": name},
		}
	}

	return &providers.SSHKey{
		ID:          intToString(key.ID),
		Name:        key.Label,
		Fingerprint: "", // Linode doesn't expose SSH key fingerprint in API
		PublicKey:   key.SSHKey,
	}, nil
}

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
	// Linode's AuthorizedKeys expects raw public keys, not key IDs
	var authorizedKeys []string
	for _, keyIDStr := range config.SSHKeys {
		keyID, err := stringToInt(keyIDStr)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "linode",
				Code:     "invalid_ssh_key",
				Message:  fmt.Sprintf("Invalid SSH key ID: %s", keyIDStr),
				Details:  map[string]interface{}{"error": err.Error()},
			}
		}

		key, err := c.client.GetSSHKey(ctx, keyID)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "linode",
				Code:     "ssh_key_not_found",
				Message:  fmt.Sprintf("SSH key not found: %s", keyIDStr),
				Details:  map[string]interface{}{"error": err.Error()},
			}
		}
		authorizedKeys = append(authorizedKeys, key.SSHKey)
	}

	booted := true
	createOpts := linodego.InstanceCreateOptions{
		Label:          config.Name,
		Region:         config.Region,
		Type:           config.Size,
		Image:          config.Image,
		AuthorizedKeys: authorizedKeys,
		BackupsEnabled: config.BackupsEnabled,
		Booted:         &booted,
	}

	if config.UserData != "" {
		createOpts.Metadata = &linodego.InstanceMetadataOptions{
			UserData: config.UserData,
		}
	}

	if len(config.Tags) > 0 {
		createOpts.Tags = config.Tags
	}

	instance, err := c.client.CreateInstance(ctx, createOpts)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "linode",
			Code:     "create_instance_failed",
			Message:  "Failed to create Linode instance",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return convertInstanceToServer(instance), nil
}

func (c *Client) GetServer(ctx context.Context, serverID string) (*providers.Server, error) {
	instanceID, err := stringToInt(serverID)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "linode",
			Code:     "invalid_server_id",
			Message:  fmt.Sprintf("Invalid server ID: %s", serverID),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	instance, err := c.client.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "linode",
			Code:     "get_instance_failed",
			Message:  "Failed to get Linode instance",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return convertInstanceToServer(instance), nil
}

func (c *Client) Destroy(ctx context.Context, serverID string) error {
	instanceID, err := stringToInt(serverID)
	if err != nil {
		return &providers.ProviderError{
			Provider: "linode",
			Code:     "invalid_server_id",
			Message:  fmt.Sprintf("Invalid server ID: %s", serverID),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	err = c.client.DeleteInstance(ctx, instanceID)
	if err != nil {
		return &providers.ProviderError{
			Provider: "linode",
			Code:     "destroy_instance_failed",
			Message:  "Failed to destroy Linode instance",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return nil
}

func (c *Client) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*providers.Server, error) {
	instanceID, err := stringToInt(serverID)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "linode",
			Code:     "invalid_server_id",
			Message:  fmt.Sprintf("Invalid server ID: %s", serverID),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		instance, err := c.client.GetInstance(ctx, instanceID)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "linode",
				Code:     "poll_instance_failed",
				Message:  "Failed to poll Linode instance status",
				Details:  map[string]interface{}{"error": err.Error()},
			}
		}

		if instance.Status == linodego.InstanceRunning {
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
		Provider: "linode",
		Code:     "timeout",
		Message:  fmt.Sprintf("Timeout waiting for instance to become active (waited %s)", timeout.String()),
		Details:  map[string]interface{}{"timeout": timeout.String()},
	}
}

func convertInstanceToServer(instance *linodego.Instance) *providers.Server {
	var publicIPv4 string

	if len(instance.IPv4) > 0 {
		publicIPv4 = instance.IPv4[0].String()
	}

	metadata := map[string]string{
		"hypervisor": instance.Hypervisor,
	}

	// Add specs to metadata if available
	if instance.Specs != nil {
		metadata["vcpus"] = fmt.Sprintf("%d", instance.Specs.VCPUs)
		metadata["memory"] = fmt.Sprintf("%d", instance.Specs.Memory)
		metadata["disk"] = fmt.Sprintf("%d", instance.Specs.Disk)

		if instance.Specs.Transfer > 0 {
			metadata["transfer"] = fmt.Sprintf("%d", instance.Specs.Transfer)
		}
	}

	return &providers.Server{
		ID:          intToString(instance.ID),
		Name:        instance.Label,
		Status:      string(instance.Status),
		PublicIPv4:  publicIPv4,
		PrivateIPv4: "",
		Region:      instance.Region,
		Size:        instance.Type,
		Image:       instance.Image,
		Tags:        instance.Tags,
		CreatedAt:   *instance.Created,
		Metadata:    metadata,
	}
}

func extractVersionFromLabel(label string) string {
	parts := strings.Fields(label)
	for _, part := range parts {
		if strings.Contains(part, ".") && len(part) <= 6 {
			return strings.TrimSuffix(part, ",")
		}
	}
	return ""
}

func stringToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func intToString(i int) string {
	return strconv.Itoa(i)
}
