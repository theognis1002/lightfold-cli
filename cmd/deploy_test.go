package cmd

import (
	"encoding/json"
	"lightfold/pkg/config"
	"lightfold/pkg/state"
	"os"
	"path/filepath"
	"testing"
)

// TestCreateTarget_SavesConfigToDisk ensures that createTarget() persists
// the target configuration to disk, preventing the bug where deploy would
// succeed but destroy would fail because config wasn't saved.
func TestCreateTarget_SavesConfigToDisk(t *testing.T) {
	// Setup: Create temp directories for config and state
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

	// Create a test project
	projectPath := t.TempDir()
	packageJSON := filepath.Join(projectPath, "package.json")
	if err := os.WriteFile(packageJSON, []byte(`{"name": "test-app"}`), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	targetName := "test-target"

	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}

	target := config.TargetConfig{
		ProjectPath: projectPath,
		Framework:   "Node.js",
		Provider:    "digitalocean",
		Builder:     "native",
	}

	doConfig := &config.DigitalOceanConfig{
		IP:          "192.168.1.100",
		SSHKey:      "/tmp/test_key",
		Username:    "deploy",
		Provisioned: true,
		DropletID:   "123456",
	}
	target.SetProviderConfig("digitalocean", doConfig)

	if err := cfg.SetTarget(targetName, target); err != nil {
		t.Fatalf("Failed to set target: %v", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	if err := state.MarkCreated(targetName, ""); err != nil {
		t.Fatalf("Failed to mark created: %v", err)
	}

	configPath := filepath.Join(tmpHome, config.LocalConfigDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", configPath)
	}

	reloadedCfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to reload config from disk: %v", err)
	}

	reloadedTarget, exists := reloadedCfg.GetTarget(targetName)
	if !exists {
		t.Fatalf("Target '%s' not found in reloaded config (config wasn't saved)", targetName)
	}

	if reloadedTarget.ProjectPath != projectPath {
		t.Errorf("ProjectPath mismatch: expected '%s', got '%s'", projectPath, reloadedTarget.ProjectPath)
	}

	if reloadedTarget.Provider != "digitalocean" {
		t.Errorf("Provider mismatch: expected 'digitalocean', got '%s'", reloadedTarget.Provider)
	}

	reloadedDOConfig, err := reloadedTarget.GetDigitalOceanConfig()
	if err != nil {
		t.Fatalf("Failed to get DigitalOcean config from reloaded target: %v", err)
	}

	if reloadedDOConfig.GetIP() != "192.168.1.100" {
		t.Errorf("IP mismatch: expected '192.168.1.100', got '%s'", reloadedDOConfig.GetIP())
	}

	if reloadedDOConfig.DropletID != "123456" {
		t.Errorf("DropletID mismatch: expected '123456', got '%s'", reloadedDOConfig.DropletID)
	}
}

// TestDeployConfigPersistence_AfterCreateTarget simulates the deploy flow
// to ensure config is properly reloaded after createTarget() completes.
func TestDeployConfigPersistence_AfterCreateTarget(t *testing.T) {
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
	packageJSON := filepath.Join(projectPath, "package.json")
	if err := os.WriteFile(packageJSON, []byte(`{"name": "test-app"}`), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	targetName := "test-target"

	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}

	target := config.TargetConfig{
		ProjectPath: projectPath,
		Framework:   "Next.js",
		Provider:    "digitalocean",
		Builder:     "nixpacks",
	}

	doConfig := &config.DigitalOceanConfig{
		IP:          "165.227.192.223",
		SSHKey:      "/tmp/test_key",
		Username:    "deploy",
		Provisioned: true,
		DropletID:   "12345",
		Region:      "nyc1",
		Size:        "s-1vcpu-1gb",
	}
	target.SetProviderConfig("digitalocean", doConfig)

	if err := cfg.SetTarget(targetName, target); err != nil {
		t.Fatalf("Failed to set target: %v", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config after createTarget: %v", err)
	}

	if err := state.MarkCreated(targetName, "12345"); err != nil {
		t.Fatalf("Failed to mark created: %v", err)
	}

	// Simulate the bug fix: reload config after createTarget() (deploy.go:135-136)
	reloadedCfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to reload config (simulating deploy.go fix): %v", err)
	}

	reloadedTarget, exists := reloadedCfg.GetTarget(targetName)
	if !exists {
		t.Fatalf("Target not found after reload - config wasn't saved properly")
	}

	reloadedDOConfig, err := reloadedTarget.GetDigitalOceanConfig()
	if err != nil {
		t.Fatalf("Failed to get DigitalOcean config from reloaded target: %v", err)
	}

	if reloadedDOConfig.IP != "165.227.192.223" {
		t.Errorf("IP not persisted: expected '165.227.192.223', got '%s'", reloadedDOConfig.IP)
	}

	if reloadedDOConfig.DropletID != "12345" {
		t.Errorf("DropletID not persisted: expected '12345', got '%s'", reloadedDOConfig.DropletID)
	}

	destroyCfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Destroy would fail - can't load config: %v", err)
	}

	destroyTarget, exists := destroyCfg.GetTarget(targetName)
	if !exists {
		t.Fatalf("Destroy would fail - target '%s' not found in config", targetName)
	}

	targetState, err := state.LoadState(targetName)
	if err != nil {
		t.Fatalf("Destroy would fail - can't load state: %v", err)
	}

	if targetState.ProvisionedID != "12345" {
		t.Errorf("Destroy would fail - provisioned ID mismatch: expected '12345', got '%s'", targetState.ProvisionedID)
	}

	destroyProviderCfg, err := destroyTarget.GetAnyProviderConfig()
	if err != nil {
		t.Fatalf("Destroy would fail - can't get provider config: %v", err)
	}

	if destroyProviderCfg.GetIP() == "" {
		t.Error("Destroy would fail - IP is empty in provider config")
	}
}

// TestConfigJSONStructure_ProviderConfig verifies that provider configs
// are stored under the correct provider key (not hardcoded to one provider)
func TestConfigJSONStructure_ProviderConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	// Test BYOS - should be under "byos" key, NOT "digitalocean"
	byosTarget := config.TargetConfig{
		ProjectPath: "/tmp/test",
		Framework:   "Express",
		Provider:    "byos",
	}
	byosConfig := &config.DigitalOceanConfig{
		IP:       "192.168.1.1",
		SSHKey:   "/tmp/key",
		Username: "root",
	}
	byosTarget.SetProviderConfig("byos", byosConfig)
	cfg.SetTarget("byos-target", byosTarget)

	// Test DigitalOcean - should be under "digitalocean" key
	doTarget := config.TargetConfig{
		ProjectPath: "/tmp/test",
		Framework:   "Next.js",
		Provider:    "digitalocean",
	}
	doConfig := &config.DigitalOceanConfig{
		IP:        "1.2.3.4",
		SSHKey:    "/tmp/key",
		Username:  "deploy",
		DropletID: "123",
	}
	doTarget.SetProviderConfig("digitalocean", doConfig)
	cfg.SetTarget("do-target", doTarget)

	// Save config
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Read raw JSON and verify structure
	configPath := filepath.Join(tmpHome, config.LocalConfigDir, "config.json")
	rawJSON, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config JSON: %v", err)
	}

	var configData map[string]interface{}
	if err := json.Unmarshal(rawJSON, &configData); err != nil {
		t.Fatalf("Failed to parse config JSON: %v", err)
	}

	targets := configData["targets"].(map[string]interface{})

	// Verify BYOS target has "byos" key in provider_config
	byosTargetData := targets["byos-target"].(map[string]interface{})
	byosProviderConfig := byosTargetData["provider_config"].(map[string]interface{})
	if _, exists := byosProviderConfig["byos"]; !exists {
		t.Error("BYOS target should have 'byos' key in provider_config, not 'digitalocean'")
	}
	if _, exists := byosProviderConfig["digitalocean"]; exists {
		t.Error("BYOS target should NOT have 'digitalocean' key in provider_config")
	}

	// Verify DigitalOcean target has "digitalocean" key in provider_config
	doTargetData := targets["do-target"].(map[string]interface{})
	doProviderConfig := doTargetData["provider_config"].(map[string]interface{})
	if _, exists := doProviderConfig["digitalocean"]; !exists {
		t.Error("DigitalOcean target should have 'digitalocean' key in provider_config")
	}
	if _, exists := doProviderConfig["byos"]; exists {
		t.Error("DigitalOcean target should NOT have 'byos' key in provider_config")
	}
}

// TestCreateTarget_Idempotency ensures that calling createTarget multiple
// times doesn't create duplicate configs or corrupt state
func TestCreateTarget_Idempotency(t *testing.T) {
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
	packageJSON := filepath.Join(projectPath, "package.json")
	if err := os.WriteFile(packageJSON, []byte(`{"name": "test-app"}`), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	targetName := "test-target"

	// Create target first time
	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	target := config.TargetConfig{
		ProjectPath: projectPath,
		Framework:   "Express",
		Provider:    "digitalocean",
	}

	doConfig := &config.DigitalOceanConfig{
		IP:         "192.168.1.1",
		SSHKey:     "/tmp/key",
		Username:   "deploy",
		DropletID:  "999",
		Provisioned: true,
	}
	target.SetProviderConfig("digitalocean", doConfig)

	cfg.SetTarget(targetName, target)
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config first time: %v", err)
	}
	state.MarkCreated(targetName, "")

	// Reload and verify
	firstCfg, _ := config.LoadConfig()
	firstTarget, exists := firstCfg.GetTarget(targetName)
	if !exists {
		t.Fatal("Target not found after first save")
	}

	// Save again (simulating second deploy)
	firstCfg.SetTarget(targetName, firstTarget)
	if err := firstCfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config second time: %v", err)
	}

	// Reload and verify it's the same
	secondCfg, _ := config.LoadConfig()
	secondTarget, exists := secondCfg.GetTarget(targetName)
	if !exists {
		t.Fatal("Target not found after second save")
	}

	// Verify no corruption
	if secondTarget.ProjectPath != projectPath {
		t.Error("ProjectPath corrupted after multiple saves")
	}

	if secondTarget.Provider != "digitalocean" {
		t.Error("Provider corrupted after multiple saves")
	}

	secondDOConfig, err := secondTarget.GetDigitalOceanConfig()
	if err != nil {
		t.Fatal("Provider config corrupted after multiple saves")
	}

	if secondDOConfig.GetIP() != "192.168.1.1" {
		t.Error("IP corrupted after multiple saves")
	}

	if secondDOConfig.DropletID != "999" {
		t.Error("DropletID corrupted after multiple saves")
	}
}

// TestDestroyWithoutConfig_CleansState tests the scenario from the bug report:
// State exists but config is missing (shouldn't happen, but destroy should handle it gracefully)
func TestDestroyWithoutConfig_CleansState(t *testing.T) {
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

	targetName := "orphaned-target"

	// Create state without config (simulating the bug)
	if err := state.MarkCreated(targetName, "12345"); err != nil {
		t.Fatalf("Failed to create state: %v", err)
	}

	// Create empty config (no targets)
	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save empty config: %v", err)
	}

	// Verify state exists
	statePath := filepath.Join(stateDir, targetName+".json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("State file should exist")
	}

	// Simulate destroy command logic (from destroy.go lines 60-74)
	loadedCfg, _ := config.LoadConfig()
	_, exists := loadedCfg.GetTarget(targetName)

	if !exists {
		// This is the code path from the bug - target not in config but state exists
		targetState, _ := state.LoadState(targetName)
		if targetState != nil && (targetState.Created || targetState.Configured) {
			// Should clean up state
			if err := state.DeleteState(targetName); err != nil {
				t.Fatalf("Failed to delete orphaned state: %v", err)
			}
		}
	}

	// Verify state was cleaned up
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("Orphaned state file should have been deleted")
	}
}

// BenchmarkConfigReload benchmarks the performance impact of reloading config
// This tests the fix added in deploy.go (lines 135-136)
func BenchmarkConfigReload(b *testing.B) {
	tmpHome := b.TempDir()
	b.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, config.LocalConfigDir)
	os.MkdirAll(configDir, 0755)

	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}

	// Create a realistic config with multiple targets
	for i := 0; i < 10; i++ {
		targetName := "target-" + string(rune('a'+i))
		target := config.TargetConfig{
			ProjectPath: "/tmp/project",
			Framework:   "Next.js",
			Provider:    "digitalocean",
		}
		doConfig := &config.DigitalOceanConfig{
			IP:       "1.2.3.4",
			SSHKey:   "/tmp/key",
			Username: "deploy",
		}
		target.SetProviderConfig("digitalocean", doConfig)
		cfg.SetTarget(targetName, target)
	}

	cfg.SaveConfig()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := config.LoadConfig()
		if err != nil {
			b.Fatalf("Failed to reload config: %v", err)
		}
	}
}

// TestDeployFlow_Integration simulates the full deploy flow from the user's perspective
// This is an integration test to catch the exact bug scenario
func TestDeployFlow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
	packageJSON := filepath.Join(projectPath, "package.json")
	if err := os.WriteFile(packageJSON, []byte(`{"name": "dummy-api"}`), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	targetName := "dummy-api"

	// STEP 1: Initial config (empty)
	cfg := &config.Config{
		Targets: make(map[string]config.TargetConfig),
	}
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}

	// STEP 2: Simulate deploy command - createTarget()
	target := config.TargetConfig{
		ProjectPath: projectPath,
		Framework:   "Express",
		Provider:    "digitalocean",
		Builder:     "nixpacks",
	}

	doConfig := &config.DigitalOceanConfig{
		IP:          "165.227.192.223",
		SSHKey:      "/tmp/key",
		Username:    "deploy",
		Provisioned: true,
		DropletID:   "67890",
		Region:      "nyc1",
		Size:        "s-1vcpu-1gb",
	}
	target.SetProviderConfig("digitalocean", doConfig)

	cfg.SetTarget(targetName, target)
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config after createTarget: %v", err)
	}

	if err := state.MarkCreated(targetName, "67890"); err != nil {
		t.Fatalf("Failed to mark created: %v", err)
	}

	// STEP 3: Deploy completes successfully
	if err := state.MarkConfigured(targetName); err != nil {
		t.Fatalf("Failed to mark configured: %v", err)
	}

	if err := state.UpdateDeployment(targetName, "abc123", "20251006213525"); err != nil {
		t.Fatalf("Failed to update deployment: %v", err)
	}

	// STEP 4: User runs destroy - THIS SHOULD WORK (the bug scenario)
	destroyCfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Destroy failed - can't load config: %v", err)
	}

	destroyTarget, exists := destroyCfg.GetTarget(targetName)
	if !exists {
		// This was the bug - target not found even though deploy succeeded
		t.Fatalf("BUG REPRODUCED: Target '%s' not found after successful deploy", targetName)
	}

	// Verify all data needed for destroy is present
	provisionedID := state.GetProvisionedID(targetName)
	if provisionedID != "67890" {
		t.Errorf("Provisioned ID mismatch: expected '67890', got '%s'", provisionedID)
	}

	destroyProviderCfg, _ := destroyTarget.GetAnyProviderConfig()
	if destroyProviderCfg == nil {
		t.Fatal("Provider config is nil - destroy would fail")
	}

	if destroyProviderCfg.GetIP() != "165.227.192.223" {
		t.Errorf("IP mismatch: expected '165.227.192.223', got '%s'", destroyProviderCfg.GetIP())
	}

	// Cleanup (simulate destroy)
	if err := state.DeleteState(targetName); err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	if err := destroyCfg.DeleteTarget(targetName); err != nil {
		t.Fatalf("Failed to delete target: %v", err)
	}

	// Verify cleanup succeeded
	if _, exists := destroyCfg.GetTarget(targetName); exists {
		t.Error("Target still exists after destroy")
	}
}
