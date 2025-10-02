package providers

import (
	"context"
	"lightfold/pkg/providers"
	"lightfold/pkg/providers/digitalocean"
	"testing"
	"time"
)

func TestDigitalOceanClientCreation(t *testing.T) {
	client := digitalocean.NewClient("fake-token")
	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.Name() != "digitalocean" {
		t.Errorf("Expected provider name 'digitalocean', got %s", client.Name())
	}
}

func TestProvisionConfigValidation(t *testing.T) {
	testCases := []struct {
		name   string
		config providers.ProvisionConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: providers.ProvisionConfig{
				Name:     "test-droplet",
				Region:   "nyc1",
				Size:     "s-1vcpu-1gb",
				Image:    "ubuntu-22-04-x64",
				SSHKeys:  []string{"12345"},
				UserData: "#cloud-config\npackages:\n  - nginx",
			},
			valid: true,
		},
		{
			name: "missing name",
			config: providers.ProvisionConfig{
				Region:  "nyc1",
				Size:    "s-1vcpu-1gb",
				Image:   "ubuntu-22-04-x64",
				SSHKeys: []string{"12345"},
			},
			valid: false,
		},
		{
			name: "missing region",
			config: providers.ProvisionConfig{
				Name:    "test-droplet",
				Size:    "s-1vcpu-1gb",
				Image:   "ubuntu-22-04-x64",
				SSHKeys: []string{"12345"},
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateProvisionConfig(tc.config)
			if tc.valid && err != nil {
				t.Errorf("Expected valid config, got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Error("Expected invalid config, got no error")
			}
		})
	}
}

func TestServerConversion(t *testing.T) {
	// Test that we can create a server object with required fields
	server := &providers.Server{
		ID:          "123456",
		Name:        "test-droplet",
		Status:      "active",
		PublicIPv4:  "203.0.113.1",
		PrivateIPv4: "10.0.0.1",
		Region:      "nyc1",
		Size:        "s-1vcpu-1gb",
		Image:       "ubuntu-22-04-x64",
		CreatedAt:   time.Now(),
		Tags:        []string{"lightfold", "auto-provisioned"},
	}

	if server.ID == "" {
		t.Error("Server ID should not be empty")
	}

	if server.Status != "active" {
		t.Errorf("Expected status 'active', got %s", server.Status)
	}

	if server.PublicIPv4 == "" {
		t.Error("Public IP should not be empty")
	}
}

func TestSSHKeyStructure(t *testing.T) {
	sshKey := &providers.SSHKey{
		ID:          "67890",
		Name:        "lightfold-test-key",
		Fingerprint: "SHA256:test-fingerprint",
		PublicKey:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
	}

	if sshKey.ID == "" {
		t.Error("SSH key ID should not be empty")
	}

	if sshKey.Name == "" {
		t.Error("SSH key name should not be empty")
	}

	if sshKey.PublicKey == "" {
		t.Error("SSH key public key should not be empty")
	}
}

// validateProvisionConfig validates provision configuration (test helper)
func validateProvisionConfig(config providers.ProvisionConfig) error {
	if config.Name == "" {
		return &providers.ProviderError{
			Provider: "test",
			Code:     "validation_error",
			Message:  "name is required",
		}
	}

	if config.Region == "" {
		return &providers.ProviderError{
			Provider: "test",
			Code:     "validation_error",
			Message:  "region is required",
		}
	}

	return nil
}

func TestProviderErrorHandling(t *testing.T) {
	err := &providers.ProviderError{
		Provider: "digitalocean",
		Code:     "invalid_credentials",
		Message:  "API token is invalid",
		Details:  map[string]interface{}{"status_code": 401},
	}

	if err.Error() != "API token is invalid" {
		t.Errorf("Expected error message 'API token is invalid', got %s", err.Error())
	}

	if err.Provider != "digitalocean" {
		t.Errorf("Expected provider 'digitalocean', got %s", err.Provider)
	}
}

func TestContextHandling(t *testing.T) {
	// Test context cancellation handling
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond) // Ensure context times out

	select {
	case <-ctx.Done():
		// Expected - context should be cancelled
	default:
		t.Error("Expected context to be cancelled")
	}

	if ctx.Err() == nil {
		t.Error("Expected context error, got nil")
	}
}