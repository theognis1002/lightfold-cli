package cmd

import (
	"context"
	"lightfold/pkg/config"
	"lightfold/pkg/providers"
	"lightfold/pkg/state"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockProvider for testing destroy functionality without actual API calls
type MockProvider struct {
	destroyCalled bool
	destroyError  error
	destroyedID   string
}

func (m *MockProvider) Name() string                      { return "mock" }
func (m *MockProvider) DisplayName() string               { return "Mock Provider" }
func (m *MockProvider) SupportsProvisioning() bool        { return true }
func (m *MockProvider) SupportsBYOS() bool                { return true }
func (m *MockProvider) ValidateCredentials(ctx context.Context) error { return nil }
func (m *MockProvider) GetRegions(ctx context.Context) ([]providers.Region, error) { return nil, nil }
func (m *MockProvider) GetSizes(ctx context.Context, region string) ([]providers.Size, error) { return nil, nil }
func (m *MockProvider) GetImages(ctx context.Context) ([]providers.Image, error) { return nil, nil }
func (m *MockProvider) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) { return nil, nil }
func (m *MockProvider) GetServer(ctx context.Context, serverID string) (*providers.Server, error) { return nil, nil }
func (m *MockProvider) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*providers.Server, error) { return nil, nil }
func (m *MockProvider) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) { return nil, nil }

func (m *MockProvider) Destroy(ctx context.Context, serverID string) error {
	m.destroyCalled = true
	m.destroyedID = serverID
	return m.destroyError
}

// setupTestEnv creates a temporary environment for testing
func setupTestEnv(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "lightfold-destroy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Override HOME environment variable
	originalHome := os.Getenv("HOME")
	testHome := filepath.Join(tmpDir, "home")
	os.Setenv("HOME", testHome)

	cleanup := func() {
		os.Setenv("HOME", originalHome)
		os.RemoveAll(tmpDir)
	}

	return testHome, cleanup
}

func TestDestroyStateOnly(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a target with state but no config
	targetName := "orphaned-target"

	if err := state.MarkCreated(targetName, ""); err != nil {
		t.Fatalf("Failed to create state: %v", err)
	}

	// Verify state exists
	if !state.IsCreated(targetName) {
		t.Fatal("State should exist")
	}

	// Delete state (simulating destroy for orphaned target)
	if err := state.DeleteState(targetName); err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	// Verify state is deleted
	if state.IsCreated(targetName) {
		t.Error("State should be deleted")
	}
}

func TestDestroyBYOSTarget(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	targetName := "byos-target"

	// Create a BYOS target
	cfg, _ := config.LoadConfig()
	target := config.TargetConfig{
		ProjectPath: "/path/to/project",
		Framework:   "Next.js",
		Provider:    "digitalocean",
	}

	// BYOS config (no provisioned ID)
	byosConfig := &config.DigitalOceanConfig{
		IP:          "192.168.1.100",
		Username:    "deploy",
		SSHKey:      "~/.ssh/id_rsa",
		Provisioned: false,
	}
	target.SetProviderConfig("digitalocean", byosConfig)
	cfg.SetTarget(targetName, target)
	cfg.SaveConfig()

	// Create state
	state.MarkCreated(targetName, "") // No provisioned ID for BYOS

	// Verify target exists
	_, exists := cfg.GetTarget(targetName)
	if !exists {
		t.Fatal("Target should exist")
	}

	// Destroy target (clean up config and state)
	if err := state.DeleteState(targetName); err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	if err := cfg.DeleteTarget(targetName); err != nil {
		t.Fatalf("Failed to delete target: %v", err)
	}

	// Verify target is deleted
	_, exists = cfg.GetTarget(targetName)
	if exists {
		t.Error("Target should be deleted")
	}

	if state.IsCreated(targetName) {
		t.Error("State should be deleted")
	}
}

func TestDestroyProvisionedTarget(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	targetName := "provisioned-target"
	provisionedID := "droplet-12345"

	// Create a provisioned target
	cfg, _ := config.LoadConfig()
	target := config.TargetConfig{
		ProjectPath: "/path/to/project",
		Framework:   "Next.js",
		Provider:    "digitalocean",
	}

	doConfig := &config.DigitalOceanConfig{
		IP:          "192.168.1.100",
		Username:    "deploy",
		SSHKey:      "~/.ssh/id_rsa",
		DropletID:   provisionedID,
		Region:      "nyc1",
		Provisioned: true,
	}
	target.SetProviderConfig("digitalocean", doConfig)
	cfg.SetTarget(targetName, target)
	cfg.SaveConfig()

	// Create state with provisioned ID
	state.MarkCreated(targetName, provisionedID)

	// Verify provisioned ID is stored
	if id := state.GetProvisionedID(targetName); id != provisionedID {
		t.Errorf("Expected provisioned ID '%s', got '%s'", provisionedID, id)
	}

	// Mock provider would destroy the VM here
	// In real destroy command, this would call provider.Destroy()

	// Clean up state and config
	if err := state.DeleteState(targetName); err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	if err := cfg.DeleteTarget(targetName); err != nil {
		t.Fatalf("Failed to delete target: %v", err)
	}

	// Verify everything is deleted
	_, exists := cfg.GetTarget(targetName)
	if exists {
		t.Error("Target should be deleted")
	}

	if state.IsCreated(targetName) {
		t.Error("State should be deleted")
	}

	if id := state.GetProvisionedID(targetName); id != "" {
		t.Error("Provisioned ID should be empty after deletion")
	}
}

func TestDestroyIdempotent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	targetName := "test-target"

	cfg, _ := config.LoadConfig()

	// Delete non-existent target (should not error)
	if err := cfg.DeleteTarget(targetName); err != nil {
		t.Errorf("DeleteTarget should be idempotent, got error: %v", err)
	}

	if err := state.DeleteState(targetName); err != nil {
		t.Errorf("DeleteState should be idempotent, got error: %v", err)
	}

	// Create target
	target := config.TargetConfig{
		ProjectPath: "/path/to/project",
		Framework:   "Next.js",
		Provider:    "digitalocean",
	}
	cfg.SetTarget(targetName, target)
	cfg.SaveConfig()
	state.MarkCreated(targetName, "server-123")

	// Delete once
	cfg.DeleteTarget(targetName)
	state.DeleteState(targetName)

	// Delete again (should be idempotent)
	if err := cfg.DeleteTarget(targetName); err != nil {
		t.Errorf("DeleteTarget should be idempotent on second call, got error: %v", err)
	}

	if err := state.DeleteState(targetName); err != nil {
		t.Errorf("DeleteState should be idempotent on second call, got error: %v", err)
	}
}

func TestDestroyMultipleTargets(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg, _ := config.LoadConfig()

	// Create multiple targets
	targets := []string{"app1", "app2", "app3"}
	for _, name := range targets {
		target := config.TargetConfig{
			ProjectPath: "/path/to/" + name,
			Framework:   "Next.js",
			Provider:    "digitalocean",
		}
		cfg.SetTarget(name, target)
		state.MarkCreated(name, "server-"+name)
	}
	cfg.SaveConfig()

	// Verify all exist
	for _, name := range targets {
		if _, exists := cfg.GetTarget(name); !exists {
			t.Errorf("Target '%s' should exist", name)
		}
		if !state.IsCreated(name) {
			t.Errorf("State for '%s' should exist", name)
		}
	}

	// Destroy app2
	cfg.DeleteTarget("app2")
	state.DeleteState("app2")

	// Verify app2 is gone, others remain
	if _, exists := cfg.GetTarget("app2"); exists {
		t.Error("app2 should be deleted")
	}
	if state.IsCreated("app2") {
		t.Error("app2 state should be deleted")
	}

	// Verify app1 and app3 still exist
	for _, name := range []string{"app1", "app3"} {
		if _, exists := cfg.GetTarget(name); !exists {
			t.Errorf("Target '%s' should still exist", name)
		}
		if !state.IsCreated(name) {
			t.Errorf("State for '%s' should still exist", name)
		}
	}
}

func TestGetProvisionedIDFromState(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	targetName := "test-target"

	// No state initially
	if id := state.GetProvisionedID(targetName); id != "" {
		t.Errorf("Expected empty ID for non-existent state, got '%s'", id)
	}

	// Create state without provisioned ID (BYOS)
	state.MarkCreated(targetName, "")
	if id := state.GetProvisionedID(targetName); id != "" {
		t.Errorf("Expected empty ID for BYOS, got '%s'", id)
	}

	// Create state with provisioned ID
	provisionedID := "droplet-67890"
	state.MarkCreated(targetName, provisionedID)
	if id := state.GetProvisionedID(targetName); id != provisionedID {
		t.Errorf("Expected ID '%s', got '%s'", provisionedID, id)
	}

	// Delete state
	state.DeleteState(targetName)
	if id := state.GetProvisionedID(targetName); id != "" {
		t.Errorf("Expected empty ID after deletion, got '%s'", id)
	}
}

func TestDestroyWithDifferentProviders(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cfg, _ := config.LoadConfig()

	// DigitalOcean target
	doTarget := config.TargetConfig{
		ProjectPath: "/path/to/do-app",
		Framework:   "Next.js",
		Provider:    "digitalocean",
	}
	doTarget.SetProviderConfig("digitalocean", &config.DigitalOceanConfig{
		IP:          "1.2.3.4",
		Username:    "deploy",
		DropletID:   "do-123",
		Provisioned: true,
	})

	// Hetzner target
	hetznerTarget := config.TargetConfig{
		ProjectPath: "/path/to/hetzner-app",
		Framework:   "Django",
		Provider:    "hetzner",
	}
	hetznerTarget.SetProviderConfig("hetzner", &config.HetznerConfig{
		IP:          "5.6.7.8",
		Username:    "deploy",
		ServerID:    "hetzner-456",
		Provisioned: true,
	})

	// Save targets
	cfg.SetTarget("do-app", doTarget)
	cfg.SetTarget("hetzner-app", hetznerTarget)
	cfg.SaveConfig()

	// Create state
	state.MarkCreated("do-app", "do-123")
	state.MarkCreated("hetzner-app", "hetzner-456")

	// Destroy DigitalOcean target
	cfg.DeleteTarget("do-app")
	state.DeleteState("do-app")

	// Verify DO target is gone
	if _, exists := cfg.GetTarget("do-app"); exists {
		t.Error("do-app should be deleted")
	}

	// Verify Hetzner target still exists
	if _, exists := cfg.GetTarget("hetzner-app"); !exists {
		t.Error("hetzner-app should still exist")
	}

	if id := state.GetProvisionedID("hetzner-app"); id != "hetzner-456" {
		t.Errorf("Hetzner provisioned ID should still be 'hetzner-456', got '%s'", id)
	}
}
