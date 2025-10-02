package hetzner

import (
	"context"
	"lightfold/pkg/providers"
	"time"
)

// Register the Hetzner provider with the global registry
func init() {
	providers.Register("hetzner", func(token string) providers.Provider {
		return NewClient(token)
	})
}

// Client implements the Provider interface for Hetzner Cloud
type Client struct {
	token string
}

// NewClient creates a new Hetzner Cloud client
func NewClient(token string) *Client {
	return &Client{
		token: token,
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

func (c *Client) ValidateCredentials(ctx context.Context) error {
	// TODO: Implement Hetzner API credential validation
	// This would typically make an API call to verify the token
	if c.token == "" {
		return &providers.ProviderError{
			Provider: "hetzner",
			Code:     "invalid_credentials",
			Message:  "Hetzner Cloud API token is required",
			Details:  map[string]interface{}{},
		}
	}
	return nil
}

func (c *Client) GetRegions(ctx context.Context) ([]providers.Region, error) {
	// TODO: Implement actual Hetzner Cloud API call
	// For now, return common Hetzner locations
	return []providers.Region{
		{ID: "nbg1", Name: "Nuremberg", Location: "Nuremberg, Germany"},
		{ID: "fsn1", Name: "Falkenstein", Location: "Falkenstein, Germany"},
		{ID: "hel1", Name: "Helsinki", Location: "Helsinki, Finland"},
		{ID: "ash", Name: "Ashburn", Location: "Ashburn, VA, USA"},
		{ID: "hil", Name: "Hillsboro", Location: "Hillsboro, OR, USA"},
	}, nil
}

func (c *Client) GetSizes(ctx context.Context, region string) ([]providers.Size, error) {
	// TODO: Implement actual Hetzner Cloud API call
	// For now, return common Hetzner server types
	return []providers.Size{
		{
			ID:           "cx11",
			Name:         "CX11 (1 vCPU, 2 GB RAM, 20 GB disk)",
			Memory:       2048,
			VCPUs:        1,
			Disk:         20,
			PriceMonthly: 3.79,
			PriceHourly:  0.005,
		},
		{
			ID:           "cx21",
			Name:         "CX21 (2 vCPUs, 4 GB RAM, 40 GB disk)",
			Memory:       4096,
			VCPUs:        2,
			Disk:         40,
			PriceMonthly: 5.83,
			PriceHourly:  0.009,
		},
		{
			ID:           "cx31",
			Name:         "CX31 (2 vCPUs, 8 GB RAM, 80 GB disk)",
			Memory:       8192,
			VCPUs:        2,
			Disk:         80,
			PriceMonthly: 11.05,
			PriceHourly:  0.016,
		},
	}, nil
}

func (c *Client) GetImages(ctx context.Context) ([]providers.Image, error) {
	// TODO: Implement actual Hetzner Cloud API call
	return []providers.Image{
		{
			ID:           "ubuntu-22.04",
			Name:         "Ubuntu 22.04 LTS",
			Distribution: "Ubuntu",
			Version:      "22.04",
		},
		{
			ID:           "ubuntu-20.04",
			Name:         "Ubuntu 20.04 LTS",
			Distribution: "Ubuntu",
			Version:      "20.04",
		},
	}, nil
}

func (c *Client) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) {
	// TODO: Implement actual Hetzner Cloud API call to upload SSH key
	// For now, return a stub
	return &providers.SSHKey{
		ID:          "123456",
		Name:        name,
		Fingerprint: "stub-fingerprint",
		PublicKey:   publicKey,
	}, nil
}

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
	// TODO: Implement actual Hetzner Cloud API call to provision server
	return nil, &providers.ProviderError{
		Provider: "hetzner",
		Code:     "not_implemented",
		Message:  "Hetzner Cloud provisioning not yet implemented",
		Details:  map[string]interface{}{},
	}
}

func (c *Client) GetServer(ctx context.Context, serverID string) (*providers.Server, error) {
	// TODO: Implement actual Hetzner Cloud API call
	return nil, &providers.ProviderError{
		Provider: "hetzner",
		Code:     "not_implemented",
		Message:  "Hetzner Cloud server retrieval not yet implemented",
		Details:  map[string]interface{}{},
	}
}

func (c *Client) Destroy(ctx context.Context, serverID string) error {
	// TODO: Implement actual Hetzner Cloud API call
	return &providers.ProviderError{
		Provider: "hetzner",
		Code:     "not_implemented",
		Message:  "Hetzner Cloud server destruction not yet implemented",
		Details:  map[string]interface{}{},
	}
}

func (c *Client) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*providers.Server, error) {
	// TODO: Implement actual Hetzner Cloud API polling
	return nil, &providers.ProviderError{
		Provider: "hetzner",
		Code:     "not_implemented",
		Message:  "Hetzner Cloud server polling not yet implemented",
		Details:  map[string]interface{}{},
	}
}
