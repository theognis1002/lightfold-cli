package deploy

import (
	"context"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"testing"
)

func TestCheckExistingServer_DigitalOcean_AlreadyProvisioned(t *testing.T) {
	// Test case: Prevent duplicate provisioning when server already exists
	projectConfig := config.TargetConfig{
		Framework: "FastAPI",
		Provider:  "digitalocean",
	}
	projectConfig.SetProviderConfig("digitalocean", config.DigitalOceanConfig{
		DropletID:   "522181726",
		IP:          "68.183.110.156",
		SSHKey:      "/Users/test/.lightfold/keys/test_key",
		SSHKeyName:  "test_key",
		Username:    "deploy",
		Region:      "nyc1",
		Size:        "s-1vcpu-512mb-10gb",
		Provisioned: true,
	})

	orchestrator, err := deploy.GetOrchestrator(projectConfig, "/tmp/test-project", "test-project")
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Attempt deployment - should fail with existing server error
	_, err = orchestrator.Deploy(context.Background())
	if err == nil {
		t.Fatal("Expected error when deploying to already provisioned server, got nil")
	}

	expectedMsg := "server already provisioned"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %s", expectedMsg, err.Error())
	}
}

func TestCheckExistingServer_DigitalOcean_NewProvisioning(t *testing.T) {
	// Test case: Allow provisioning when no server exists yet
	projectConfig := config.TargetConfig{
		Framework: "FastAPI",
		Provider:  "digitalocean",
	}
	projectConfig.SetProviderConfig("digitalocean", config.DigitalOceanConfig{
		DropletID:   "",
		IP:          "",
		SSHKey:      "/Users/test/.lightfold/keys/test_key",
		SSHKeyName:  "test_key",
		Username:    "deploy",
		Region:      "nyc1",
		Size:        "s-1vcpu-512mb-10gb",
		Provisioned: true,
	})

	orchestrator, err := deploy.GetOrchestrator(projectConfig, "/tmp/test-project", "test-project")
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// This should pass the checkExistingServer validation
	// (Will fail later due to missing API token, but that's expected)
	_, err = orchestrator.Deploy(context.Background())

	// Should NOT be the "already provisioned" error
	if err != nil && contains(err.Error(), "server already provisioned") {
		t.Errorf("Should not block provisioning when no server exists, got: %s", err.Error())
	}
}

func TestCheckExistingServer_DigitalOcean_BYOS(t *testing.T) {
	// Test case: Allow deployment to BYOS (Bring Your Own Server)
	projectConfig := config.TargetConfig{
		Framework: "FastAPI",
		Provider:  "digitalocean",
	}
	projectConfig.SetProviderConfig("digitalocean", config.DigitalOceanConfig{
		IP:          "192.168.1.100",
		SSHKey:      "/Users/test/.ssh/id_rsa",
		Username:    "root",
		Provisioned: false, // BYOS, not provisioned by us
	})

	orchestrator, err := deploy.GetOrchestrator(projectConfig, "/tmp/test-project", "test-project")
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// This should pass the checkExistingServer validation (BYOS is allowed)
	_, err = orchestrator.Deploy(context.Background())

	// Should NOT be the "already provisioned" error
	if err != nil && contains(err.Error(), "server already provisioned") {
		t.Errorf("Should not block BYOS deployments, got: %s", err.Error())
	}
}

func TestCheckExistingServer_S3_NoCheck(t *testing.T) {
	// Test case: S3 deployments don't provision servers, so no check needed
	projectConfig := config.TargetConfig{
		Framework: "React",
		Provider:  "s3",
	}
	projectConfig.SetProviderConfig("s3", config.S3Config{
		Bucket: "my-bucket",
		Region: "us-east-1",
	})

	orchestrator, err := deploy.GetOrchestrator(projectConfig, "/tmp/test-project", "test-project")
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// S3 deployments should never trigger the "already provisioned" check
	_, err = orchestrator.Deploy(context.Background())

	// Should NOT be the "already provisioned" error
	if err != nil && contains(err.Error(), "server already provisioned") {
		t.Errorf("S3 deployments should not check for existing servers, got: %s", err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
