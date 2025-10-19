package hetzner

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"strconv"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func init() {
	providers.Register("hetzner", func(token string) providers.Provider {
		return NewClient(token)
	})
}

type Client struct {
	client *hcloud.Client
	token  string
}

func NewClient(token string) *Client {
	client := hcloud.NewClient(hcloud.WithToken(token))
	return &Client{
		client: client,
		token:  token,
	}
}

func (c *Client) Name() string {
	return "hetzner"
}

func (c *Client) DisplayName() string {
	return "Hetzner Cloud"
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
	if c.token == "" {
		return &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_credentials",
			Message:  "Hetzner Cloud API token is required",
			Details:  map[string]interface{}{},
		}
	}

	_, _, err := c.client.Location.List(ctx, hcloud.LocationListOpts{})
	if err != nil {
		return &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_credentials",
			Message:  "Invalid Hetzner Cloud API token",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return nil
}

func (c *Client) GetRegions(ctx context.Context) ([]providers.Region, error) {
	locations, _, err := c.client.Location.List(ctx, hcloud.LocationListOpts{})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "list_regions_failed",
			Message:  "Failed to list Hetzner Cloud locations",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var regions []providers.Region
	for _, location := range locations {
		regions = append(regions, providers.Region{
			ID:       location.Name,
			Name:     location.City,
			Location: fmt.Sprintf("%s, %s", location.City, location.Country),
		})
	}

	return regions, nil
}

func (c *Client) GetSizes(ctx context.Context, region string) ([]providers.Size, error) {
	serverTypes, _, err := c.client.ServerType.List(ctx, hcloud.ServerTypeListOpts{})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "list_sizes_failed",
			Message:  "Failed to list Hetzner Cloud server types",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var sizes []providers.Size
	for _, st := range serverTypes {
		memoryMB := int(st.Memory * 1024)
		if memoryMB >= 512 {
			priceMonthly := 0.0
			priceHourly := 0.0
			if len(st.Pricings) > 0 {
				pricing := st.Pricings[0]
				if region != "" {
					for _, p := range st.Pricings {
						if p.Location.Name == region {
							pricing = p
							break
						}
					}
				}
				priceMonthly, _ = strconv.ParseFloat(pricing.Monthly.Net, 64)
				priceHourly, _ = strconv.ParseFloat(pricing.Hourly.Net, 64)
			}

			sizes = append(sizes, providers.Size{
				ID:           st.Name,
				Name:         fmt.Sprintf("%s (%d GB RAM, %d vCPUs, %d GB disk)", st.Name, int(st.Memory), st.Cores, st.Disk),
				Memory:       memoryMB,
				VCPUs:        st.Cores,
				Disk:         st.Disk,
				PriceMonthly: priceMonthly,
				PriceHourly:  priceHourly,
			})
		}
	}

	return sizes, nil
}

func (c *Client) GetImages(ctx context.Context) ([]providers.Image, error) {
	images, _, err := c.client.Image.List(ctx, hcloud.ImageListOpts{
		Type:   []hcloud.ImageType{hcloud.ImageTypeSystem},
		Status: []hcloud.ImageStatus{hcloud.ImageStatusAvailable},
	})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "list_images_failed",
			Message:  "Failed to list Hetzner Cloud images",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var providerImages []providers.Image
	for _, image := range images {
		if strings.Contains(strings.ToLower(image.Description), "ubuntu") {
			providerImages = append(providerImages, providers.Image{
				ID:           image.Name,
				Name:         image.Description,
				Distribution: "Ubuntu",
				Version:      extractVersionFromImageName(image.Description),
			})
		}
	}

	if len(providerImages) == 0 {
		providerImages = append(providerImages, providers.Image{
			ID:           providers.GetDefaultImage("hetzner"),
			Name:         "Ubuntu 22.04 LTS",
			Distribution: "Ubuntu",
			Version:      "22.04",
		})
	}

	return providerImages, nil
}

func (c *Client) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) {
	key, _, err := c.client.SSHKey.Create(ctx, hcloud.SSHKeyCreateOpts{
		Name:      name,
		PublicKey: publicKey,
	})

	if err != nil {
		if strings.Contains(err.Error(), "uniqueness_error") || strings.Contains(err.Error(), "already exists") {
			keys, _, listErr := c.client.SSHKey.List(ctx, hcloud.SSHKeyListOpts{})
			if listErr == nil {
				for _, k := range keys {
					if k.Name == name {
						return &providers.SSHKey{
							ID:          strconv.FormatInt(k.ID, 10),
							Name:        k.Name,
							Fingerprint: k.Fingerprint,
							PublicKey:   k.PublicKey,
						}, nil
					}
				}
			}
		}

		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "upload_ssh_key_failed",
			Message:  fmt.Sprintf("Failed to upload SSH key to Hetzner Cloud: %v", err),
			Details:  map[string]interface{}{"error": err.Error(), "name": name},
		}
	}

	return &providers.SSHKey{
		ID:          strconv.FormatInt(key.ID, 10),
		Name:        key.Name,
		Fingerprint: key.Fingerprint,
		PublicKey:   key.PublicKey,
	}, nil
}

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
	// Fetch server type
	serverType, _, err := c.client.ServerType.GetByName(ctx, config.Size)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_server_type",
			Message:  fmt.Sprintf("Invalid server type: %s", config.Size),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}
	if serverType == nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "server_type_not_found",
			Message:  fmt.Sprintf("Server type not found: %s", config.Size),
			Details:  map[string]interface{}{},
		}
	}

	// Fetch location
	location, _, err := c.client.Location.GetByName(ctx, config.Region)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_location",
			Message:  fmt.Sprintf("Invalid location: %s", config.Region),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}
	if location == nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "location_not_found",
			Message:  fmt.Sprintf("Location not found: %s", config.Region),
			Details:  map[string]interface{}{},
		}
	}

	// Fetch image
	image, _, err := c.client.Image.GetByName(ctx, config.Image)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_image",
			Message:  fmt.Sprintf("Invalid image: %s", config.Image),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}
	if image == nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "image_not_found",
			Message:  fmt.Sprintf("Image not found: %s", config.Image),
			Details:  map[string]interface{}{},
		}
	}

	var sshKeys []*hcloud.SSHKey
	for _, keyID := range config.SSHKeys {
		keyIDInt, err := strconv.ParseInt(keyID, 10, 64)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "hetzner",
				Code:     "invalid_ssh_key",
				Message:  fmt.Sprintf("Invalid SSH key ID: %s", keyID),
				Details:  map[string]interface{}{"error": err.Error()},
			}
		}

		key, _, err := c.client.SSHKey.GetByID(ctx, keyIDInt)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "hetzner",
				Code:     "ssh_key_not_found",
				Message:  fmt.Sprintf("SSH key not found: %s", keyID),
				Details:  map[string]interface{}{"error": err.Error()},
			}
		}
		sshKeys = append(sshKeys, key)
	}

	result, _, err := c.client.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:       config.Name,
		ServerType: serverType,
		Location:   location,
		Image:      image,
		SSHKeys:    sshKeys,
		UserData:   config.UserData,
		Labels:     convertTagsToLabels(config.Tags),
	})

	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "create_server_failed",
			Message:  "Failed to create Hetzner Cloud server",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return convertServerToProvider(result.Server), nil
}

func (c *Client) GetServer(ctx context.Context, serverID string) (*providers.Server, error) {
	id, err := strconv.ParseInt(serverID, 10, 64)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_server_id",
			Message:  fmt.Sprintf("Invalid server ID: %s", serverID),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	server, _, err := c.client.Server.GetByID(ctx, id)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "get_server_failed",
			Message:  "Failed to get Hetzner Cloud server",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	if server == nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "server_not_found",
			Message:  fmt.Sprintf("Server not found: %s", serverID),
			Details:  map[string]interface{}{},
		}
	}

	return convertServerToProvider(server), nil
}

func (c *Client) Destroy(ctx context.Context, serverID string) error {
	id, err := strconv.ParseInt(serverID, 10, 64)
	if err != nil {
		return &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_server_id",
			Message:  fmt.Sprintf("Invalid server ID: %s", serverID),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	server := &hcloud.Server{ID: id}
	_, _, err = c.client.Server.DeleteWithResult(ctx, server)
	if err != nil {
		return &providers.ProviderError{
			Provider: "hetzner",
			Code:     "destroy_server_failed",
			Message:  "Failed to destroy Hetzner Cloud server",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return nil
}

func (c *Client) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*providers.Server, error) {
	id, err := strconv.ParseInt(serverID, 10, 64)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_server_id",
			Message:  fmt.Sprintf("Invalid server ID: %s", serverID),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		server, _, err := c.client.Server.GetByID(ctx, id)
		if err != nil {
			return nil, &providers.ProviderError{
				Provider: "hetzner",
				Code:     "poll_server_failed",
				Message:  "Failed to poll Hetzner Cloud server status",
				Details:  map[string]interface{}{"error": err.Error()},
			}
		}

		if server.Status == hcloud.ServerStatusRunning {
			return convertServerToProvider(server), nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			continue
		}
	}

	return nil, &providers.ProviderError{
		Provider: "hetzner",
		Code:     "timeout",
		Message:  fmt.Sprintf("Timeout waiting for server to become active (waited %s)", timeout.String()),
		Details:  map[string]interface{}{"timeout": timeout.String()},
	}
}

func convertServerToProvider(server *hcloud.Server) *providers.Server {
	var publicIPv4, privateIPv4 string

	if server.PublicNet.IPv4.IP != nil {
		publicIPv4 = server.PublicNet.IPv4.IP.String()
	}

	for _, privateNet := range server.PrivateNet {
		if privateNet.IP != nil {
			privateIPv4 = privateNet.IP.String()
			break
		}
	}

	metadata := map[string]string{
		"vcpus":  fmt.Sprintf("%d", server.ServerType.Cores),
		"memory": fmt.Sprintf("%.0f", server.ServerType.Memory),
		"disk":   fmt.Sprintf("%d", server.ServerType.Disk),
		"locked": fmt.Sprintf("%t", server.Locked),
	}

	if server.Datacenter != nil && server.Datacenter.Location != nil {
		metadata["datacenter"] = server.Datacenter.Name
		metadata["location"] = server.Datacenter.Location.City
	}

	var tags []string
	for key, value := range server.Labels {
		tags = append(tags, fmt.Sprintf("%s:%s", key, value))
	}

	regionName := ""
	if server.Datacenter != nil && server.Datacenter.Location != nil {
		regionName = server.Datacenter.Location.Name
	}

	imageID := ""
	if server.Image != nil {
		imageID = server.Image.Name
	}

	return &providers.Server{
		ID:          strconv.FormatInt(server.ID, 10),
		Name:        server.Name,
		Status:      string(server.Status),
		PublicIPv4:  publicIPv4,
		PrivateIPv4: privateIPv4,
		Region:      regionName,
		Size:        server.ServerType.Name,
		Image:       imageID,
		Tags:        tags,
		CreatedAt:   server.Created,
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

func convertTagsToLabels(tags []string) map[string]string {
	labels := make(map[string]string)
	for _, tag := range tags {
		parts := strings.SplitN(tag, ":", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		} else {
			labels[tag] = "true"
		}
	}
	return labels
}
