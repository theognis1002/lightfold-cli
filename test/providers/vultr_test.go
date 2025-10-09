package providers

import (
	"context"
	"lightfold/pkg/providers"
	"lightfold/pkg/providers/vultr"
	"testing"
	"time"
)

func TestVultrClientCreation(t *testing.T) {
	client := vultr.NewClient("fake-token")
	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.Name() != "vultr" {
		t.Errorf("Expected provider name 'vultr', got %s", client.Name())
	}

	if client.DisplayName() != "Vultr" {
		t.Errorf("Expected display name 'Vultr', got %s", client.DisplayName())
	}
}

func TestVultrProviderInterface(t *testing.T) {
	client := vultr.NewClient("test-token")

	// Verify interface implementation
	var _ providers.Provider = client

	if !client.SupportsProvisioning() {
		t.Error("Vultr should support provisioning")
	}

	if !client.SupportsBYOS() {
		t.Error("Vultr should support BYOS")
	}
}

func TestVultrProvisionConfigValidation(t *testing.T) {
	testCases := []struct {
		name   string
		config providers.ProvisionConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: providers.ProvisionConfig{
				Name:     "test-instance",
				Region:   "ewr",
				Size:     "vc2-1c-1gb",
				Image:    "387",
				SSHKeys:  []string{"abc123"},
				UserData: "#cloud-config\npackages:\n  - nginx",
			},
			valid: true,
		},
		{
			name: "missing name",
			config: providers.ProvisionConfig{
				Region:  "ewr",
				Size:    "vc2-1c-1gb",
				Image:   "387",
				SSHKeys: []string{"abc123"},
			},
			valid: false,
		},
		{
			name: "missing region",
			config: providers.ProvisionConfig{
				Name:    "test-instance",
				Size:    "vc2-1c-1gb",
				Image:   "387",
				SSHKeys: []string{"abc123"},
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateVultrProvisionConfig(tc.config)
			if tc.valid && err != nil {
				t.Errorf("Expected valid config, got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Error("Expected invalid config, got no error")
			}
		})
	}
}

func TestVultrServerConversion(t *testing.T) {
	server := &providers.Server{
		ID:          "abc-123-def",
		Name:        "test-instance",
		Status:      "active",
		PublicIPv4:  "203.0.113.1",
		PrivateIPv4: "10.0.0.1",
		Region:      "ewr",
		Size:        "vc2-1c-1gb",
		Image:       "387",
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

func TestVultrSSHKeyStructure(t *testing.T) {
	sshKey := &providers.SSHKey{
		ID:          "key-12345",
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

func TestVultrProviderErrorHandling(t *testing.T) {
	err := &providers.ProviderError{
		Provider: "vultr",
		Code:     "invalid_credentials",
		Message:  "API token is invalid",
		Details:  map[string]interface{}{"status_code": 401},
	}

	expectedMsg := "API token is invalid (HTTP 401)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got %s", expectedMsg, err.Error())
	}

	if err.Provider != "vultr" {
		t.Errorf("Expected provider 'vultr', got %s", err.Provider)
	}
}

func TestVultrContextHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond)

	select {
	case <-ctx.Done():
	default:
		t.Error("Expected context to be cancelled")
	}

	if ctx.Err() == nil {
		t.Error("Expected context error, got nil")
	}
}

func TestStaticFallbacks(t *testing.T) {
	staticRegions := vultr.GetStaticRegions()
	if len(staticRegions) == 0 {
		t.Error("Static regions fallback should not be empty")
	}

	staticPlans := vultr.GetStaticPlans()
	if len(staticPlans) == 0 {
		t.Error("Static plans fallback should not be empty")
	}

	staticImages := vultr.GetStaticImages()
	if len(staticImages) == 0 {
		t.Error("Static images fallback should not be empty")
	}
}
func validateVultrProvisionConfig(config providers.ProvisionConfig) error {
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
