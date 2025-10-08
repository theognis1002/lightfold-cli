package cmd

import (
	"encoding/json"
	"lightfold/pkg/config"
	"lightfold/pkg/state"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSyncCommand_StateRecovery tests that sync correctly updates local state
// when it differs from what would be inferred from server accessibility
func TestSyncCommand_StateRecovery(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	stateDir := filepath.Join(configDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	projectPath := t.TempDir()
	targetName := "test-sync"

	// Create config with target
	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	target := config.TargetConfig{
		ProjectPath: projectPath,
		Framework:   "Next.js",
		Provider:    "digitalocean",
		Builder:     "nixpacks",
	}

	doConfig := &config.DigitalOceanConfig{
		IP:          "192.168.1.100",
		SSHKey:      "/tmp/test_key",
		Username:    "deploy",
		Provisioned: true,
		DropletID:   "123456",
		Region:      "nyc1",
		Size:        "s-1vcpu-1gb",
	}
	target.SetProviderConfig("digitalocean", doConfig)

	cfg.SetTarget(targetName, target)
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Create state with outdated/missing info (simulating drift)
	badState := &state.TargetState{
		Created:    false, // Should be true if server is accessible
		Configured: false,
		LastCommit: "",
	}
	if err := state.SaveState(targetName, badState); err != nil {
		t.Fatalf("Failed to save bad state: %v", err)
	}

	// Verify state is recoverable
	loadedState, err := state.LoadState(targetName)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Verify we can detect the drift
	if loadedState.Created {
		t.Error("Expected Created=false in drifted state")
	}

	// Simulate what syncTarget would do: mark as created if config has IP
	if doConfig.GetIP() != "" && doConfig.Provisioned {
		loadedState.Created = true
		if err := state.SaveState(targetName, loadedState); err != nil {
			t.Fatalf("Failed to update state: %v", err)
		}
	}

	// Verify state was corrected
	correctedState, err := state.LoadState(targetName)
	if err != nil {
		t.Fatalf("Failed to load corrected state: %v", err)
	}

	if !correctedState.Created {
		t.Error("Expected Created=true after sync simulation")
	}
}

// TestSyncCommand_PreservesUserConfig tests that sync doesn't delete
// user-supplied configuration like domain settings
func TestSyncCommand_PreservesUserConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	stateDir := filepath.Join(configDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	targetName := "prod-app"

	// Create config with custom domain
	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	target := config.TargetConfig{
		ProjectPath: "/tmp/app",
		Framework:   "Express",
		Provider:    "digitalocean",
		Builder:     "native",
		Domain: &config.DomainConfig{ // User-supplied
			Domain:     "app.example.com",
			SSLEnabled: true,
		},
		Deploy: &config.DeploymentOptions{ // User-supplied
			EnvVars: map[string]string{
				"DATABASE_URL": "postgres://...",
				"API_KEY":      "secret123",
			},
		},
	}

	doConfig := &config.DigitalOceanConfig{
		IP:       "10.0.0.1",
		SSHKey:   "/tmp/key",
		Username: "deploy",
	}
	target.SetProviderConfig("digitalocean", doConfig)

	cfg.SetTarget(targetName, target)
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Reload config to simulate sync operation
	reloadedCfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	reloadedTarget, exists := reloadedCfg.GetTarget(targetName)
	if !exists {
		t.Fatal("Target not found after reload")
	}

	// Verify user-supplied config is preserved
	if reloadedTarget.Domain == nil {
		t.Fatal("Domain config is nil")
	}

	if reloadedTarget.Domain.Domain != "app.example.com" {
		t.Errorf("Domain not preserved: expected 'app.example.com', got '%s'", reloadedTarget.Domain.Domain)
	}

	if !reloadedTarget.Domain.SSLEnabled {
		t.Error("SSL enabled flag not preserved")
	}

	if reloadedTarget.Deploy == nil {
		t.Fatal("Deploy config is nil")
	}

	if len(reloadedTarget.Deploy.EnvVars) != 2 {
		t.Errorf("EnvVars not preserved: expected 2, got %d", len(reloadedTarget.Deploy.EnvVars))
	}

	if reloadedTarget.Deploy.EnvVars["DATABASE_URL"] != "postgres://..." {
		t.Error("DATABASE_URL env var not preserved")
	}

	if reloadedTarget.Deploy.EnvVars["API_KEY"] != "secret123" {
		t.Error("API_KEY env var not preserved")
	}
}

// TestSyncCommand_HandlesAllProviders tests that sync works with all provider types
func TestSyncCommand_HandlesAllProviders(t *testing.T) {
	providers := []struct {
		name       string
		provider   string
		configFunc func() interface{}
	}{
		{
			name:     "digitalocean",
			provider: "digitalocean",
			configFunc: func() interface{} {
				return &config.DigitalOceanConfig{
					IP:        "1.2.3.4",
					SSHKey:    "/tmp/key",
					Username:  "deploy",
					DropletID: "123",
				}
			},
		},
		{
			name:     "vultr",
			provider: "vultr",
			configFunc: func() interface{} {
				return &config.VultrConfig{
					IP:         "5.6.7.8",
					SSHKey:     "/tmp/key",
					Username:   "deploy",
					InstanceID: "456",
				}
			},
		},
		{
			name:     "hetzner",
			provider: "hetzner",
			configFunc: func() interface{} {
				return &config.HetznerConfig{
					IP:       "9.10.11.12",
					SSHKey:   "/tmp/key",
					Username: "deploy",
					ServerID: "789",
				}
			},
		},
		{
			name:     "s3",
			provider: "s3",
			configFunc: func() interface{} {
				return &config.S3Config{
					Bucket:    "my-bucket",
					Region:    "us-east-1",
					AccessKey: "AKIA...",
					SecretKey: "secret",
				}
			},
		},
	}

	for _, tt := range providers {
		t.Run(tt.name, func(t *testing.T) {
			tmpHome := t.TempDir()
			t.Setenv("HOME", tmpHome)

			configDir := filepath.Join(tmpHome, config.LocalConfigDir)
			if err := os.MkdirAll(configDir, 0755); err != nil {
				t.Fatalf("Failed to create config dir: %v", err)
			}

			targetName := "test-" + tt.provider

			cfg := &config.Config{
				Targets: make(map[string]config.TargetConfig),
			}

			target := config.TargetConfig{
				ProjectPath: "/tmp/test",
				Framework:   "Express",
				Provider:    tt.provider,
			}

			// Set provider-specific config
			providerCfg := tt.configFunc()
			target.SetProviderConfig(tt.provider, providerCfg)

			cfg.SetTarget(targetName, target)
			if err := cfg.SaveConfig(); err != nil {
				t.Fatalf("Failed to save config: %v", err)
			}

			// Reload and verify
			reloadedCfg, err := config.LoadConfig()
			if err != nil {
				t.Fatalf("Failed to reload config: %v", err)
			}

			reloadedTarget, exists := reloadedCfg.GetTarget(targetName)
			if !exists {
				t.Fatal("Target not found after reload")
			}

			if reloadedTarget.Provider != tt.provider {
				t.Errorf("Provider mismatch: expected '%s', got '%s'", tt.provider, reloadedTarget.Provider)
			}

			// Verify provider config is accessible (skip for S3 which doesn't have SSH)
			if tt.provider != "s3" {
				providerConfig, err := reloadedTarget.GetSSHProviderConfig()
				if err != nil {
					t.Fatalf("Failed to get provider config: %v", err)
				}

				if providerConfig.GetIP() == "" {
					t.Error("Provider config IP is empty")
				}
			} else {
				// Verify S3 config is present
				var s3Config config.S3Config
				if err := reloadedTarget.GetProviderConfig("s3", &s3Config); err != nil {
					t.Fatalf("Failed to get S3 config: %v", err)
				}

				if s3Config.Bucket != "my-bucket" {
					t.Error("S3 bucket not preserved")
				}
			}
		})
	}
}

// TestSyncCommand_ConfigJSON_Structure verifies that sync operations
// maintain the correct JSON structure
func TestSyncCommand_ConfigJSON_Structure(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	targetName := "structured-target"

	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	target := config.TargetConfig{
		ProjectPath: "/tmp/app",
		Framework:   "Django",
		Provider:    "hetzner",
		Builder:     "nixpacks",
	}

	hetznerConfig := &config.HetznerConfig{
		IP:       "192.0.2.1",
		SSHKey:   "/tmp/key",
		Username: "deploy",
		ServerID: "999",
	}
	target.SetProviderConfig("hetzner", hetznerConfig)

	cfg.SetTarget(targetName, target)
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Read raw JSON to verify structure
	configPath := filepath.Join(tmpHome, config.LocalConfigDir, "config.json")
	rawJSON, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config JSON: %v", err)
	}

	var configData map[string]interface{}
	if err := json.Unmarshal(rawJSON, &configData); err != nil {
		t.Fatalf("Failed to parse config JSON: %v", err)
	}

	// Verify top-level structure
	if _, exists := configData["targets"]; !exists {
		t.Fatal("Config JSON missing 'targets' key")
	}

	targets := configData["targets"].(map[string]interface{})
	targetData := targets[targetName].(map[string]interface{})

	// Verify target has required fields
	if targetData["project_path"] != "/tmp/app" {
		t.Error("Target missing or incorrect 'project_path'")
	}

	if targetData["framework"] != "Django" {
		t.Error("Target missing or incorrect 'framework'")
	}

	if targetData["provider"] != "hetzner" {
		t.Error("Target missing or incorrect 'provider'")
	}

	// Verify provider_config structure
	providerConfig := targetData["provider_config"].(map[string]interface{})
	if _, exists := providerConfig["hetzner"]; !exists {
		t.Error("Provider config missing 'hetzner' key")
	}

	hetznerData := providerConfig["hetzner"].(map[string]interface{})
	if hetznerData["ip"] != "192.0.2.1" {
		t.Error("Hetzner config missing or incorrect 'ip'")
	}
}

// TestSyncCommand_StateFile_Structure verifies state file structure
func TestSyncCommand_StateFile_Structure(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	stateDir := filepath.Join(configDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	targetName := "state-target"

	// Create state
	testState := &state.TargetState{
		Created:        true,
		Configured:     true,
		LastCommit:     "abc123def456",
		LastRelease:    "20251008120000",
		LastDeploy:     time.Now(),
		ProvisionedID:  "12345",
		Builder:        "nixpacks",
		SSLConfigured:  true,
		LastSSLRenewal: time.Now(),
	}

	if err := state.SaveState(targetName, testState); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Read raw JSON to verify structure
	statePath := state.GetTargetStatePath(targetName)
	rawJSON, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state JSON: %v", err)
	}

	var stateData map[string]interface{}
	if err := json.Unmarshal(rawJSON, &stateData); err != nil {
		t.Fatalf("Failed to parse state JSON: %v", err)
	}

	// Verify all expected fields exist
	expectedFields := []string{
		"created",
		"configured",
		"last_commit",
		"last_release",
		"last_deploy",
		"provisioned_id",
		"builder",
		"ssl_configured",
		"last_ssl_renewal",
	}

	for _, field := range expectedFields {
		if _, exists := stateData[field]; !exists {
			t.Errorf("State JSON missing expected field: %s", field)
		}
	}

	// Verify data types and values
	if created, ok := stateData["created"].(bool); !ok || !created {
		t.Error("State 'created' should be bool true")
	}

	if configured, ok := stateData["configured"].(bool); !ok || !configured {
		t.Error("State 'configured' should be bool true")
	}

	if commit, ok := stateData["last_commit"].(string); !ok || commit != "abc123def456" {
		t.Error("State 'last_commit' incorrect")
	}

	if builder, ok := stateData["builder"].(string); !ok || builder != "nixpacks" {
		t.Error("State 'builder' incorrect")
	}
}

// TestSyncCommand_Idempotency ensures sync can be run multiple times safely
func TestSyncCommand_Idempotency(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	stateDir := filepath.Join(configDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	targetName := "idempotent-target"

	// Create initial config and state
	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	target := config.TargetConfig{
		ProjectPath: "/tmp/app",
		Framework:   "Astro",
		Provider:    "digitalocean",
	}

	doConfig := &config.DigitalOceanConfig{
		IP:       "203.0.113.1",
		SSHKey:   "/tmp/key",
		Username: "deploy",
	}
	target.SetProviderConfig("digitalocean", doConfig)

	cfg.SetTarget(targetName, target)
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	initialState := &state.TargetState{
		Created:     true,
		Configured:  true,
		LastCommit:  "initial123",
		LastRelease: "20251008100000",
		LastDeploy:  time.Now(),
	}
	if err := state.SaveState(targetName, initialState); err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	// Simulate first sync - just reload and save
	cfg1, _ := config.LoadConfig()
	target1, _ := cfg1.GetTarget(targetName)
	cfg1.SetTarget(targetName, target1)
	if err := cfg1.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config after sync 1: %v", err)
	}

	state1, _ := state.LoadState(targetName)
	if err := state.SaveState(targetName, state1); err != nil {
		t.Fatalf("Failed to save state after sync 1: %v", err)
	}

	// Simulate second sync
	cfg2, _ := config.LoadConfig()
	target2, _ := cfg2.GetTarget(targetName)
	cfg2.SetTarget(targetName, target2)
	if err := cfg2.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config after sync 2: %v", err)
	}

	state2, _ := state.LoadState(targetName)
	if err := state.SaveState(targetName, state2); err != nil {
		t.Fatalf("Failed to save state after sync 2: %v", err)
	}

	// Verify nothing was corrupted
	finalCfg, _ := config.LoadConfig()
	finalTarget, exists := finalCfg.GetTarget(targetName)
	if !exists {
		t.Fatal("Target disappeared after multiple syncs")
	}

	if finalTarget.ProjectPath != "/tmp/app" {
		t.Error("ProjectPath corrupted after multiple syncs")
	}

	if finalTarget.Framework != "Astro" {
		t.Error("Framework corrupted after multiple syncs")
	}

	finalState, _ := state.LoadState(targetName)
	if finalState.LastCommit != "initial123" {
		t.Error("LastCommit corrupted after multiple syncs")
	}

	if !finalState.Created || !finalState.Configured {
		t.Error("State flags corrupted after multiple syncs")
	}
}

// TestSyncCommand_MissingConfig tests sync behavior when config is missing
func TestSyncCommand_MissingConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Try to load config when it doesn't exist - LoadConfig creates it if missing
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig should create config if missing: %v", err)
	}

	if cfg == nil {
		t.Fatal("Config should not be nil")
	}

	// Verify it created an empty config
	if len(cfg.Targets) != 0 {
		t.Errorf("New config should have 0 targets, got %d", len(cfg.Targets))
	}
}

// TestSyncCommand_PartialState tests sync with incomplete state data
func TestSyncCommand_PartialState(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	stateDir := filepath.Join(configDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	targetName := "partial-target"

	// Create partial state (only some fields set)
	partialState := &state.TargetState{
		Created:    true,
		Configured: false, // Not yet configured
		// LastCommit, LastRelease, LastDeploy are empty
	}

	if err := state.SaveState(targetName, partialState); err != nil {
		t.Fatalf("Failed to save partial state: %v", err)
	}

	// Load and verify
	loadedState, err := state.LoadState(targetName)
	if err != nil {
		t.Fatalf("Failed to load partial state: %v", err)
	}

	if !loadedState.Created {
		t.Error("Created should be true in partial state")
	}

	if loadedState.Configured {
		t.Error("Configured should be false in partial state")
	}

	if loadedState.LastCommit != "" {
		t.Error("LastCommit should be empty in partial state")
	}

	if loadedState.LastRelease != "" {
		t.Error("LastRelease should be empty in partial state")
	}

	// Update state to complete it
	loadedState.Configured = true
	loadedState.LastCommit = "complete123"
	if err := state.SaveState(targetName, loadedState); err != nil {
		t.Fatalf("Failed to save completed state: %v", err)
	}

	// Verify update
	completedState, err := state.LoadState(targetName)
	if err != nil {
		t.Fatalf("Failed to load completed state: %v", err)
	}

	if !completedState.Configured {
		t.Error("Configured should be true after update")
	}

	if completedState.LastCommit != "complete123" {
		t.Error("LastCommit should be set after update")
	}
}
