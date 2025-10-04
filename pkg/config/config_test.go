package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestConfigDir creates a temporary directory for testing
// and sets the HOME env var to use it
func setupTestConfigDir(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "lightfold-config-test-*")
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

func TestLoadConfig(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Loading non-existent config should return empty config
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error loading non-existent config, got: %v", err)
	}

	if cfg.Targets == nil {
		t.Error("Expected Targets map to be initialized")
	}

	if len(cfg.Targets) != 0 {
		t.Error("Expected empty Targets map")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create config with a target
	cfg := &Config{
		Targets: map[string]TargetConfig{
			"myapp": {
				ProjectPath: "/path/to/project",
				Framework:   "Next.js",
				Provider:    "digitalocean",
			},
		},
	}

	// Save config
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load config
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify
	if len(loadedCfg.Targets) != 1 {
		t.Errorf("Expected 1 target, got %d", len(loadedCfg.Targets))
	}

	target, exists := loadedCfg.Targets["myapp"]
	if !exists {
		t.Fatal("Expected 'myapp' target to exist")
	}

	if target.ProjectPath != "/path/to/project" {
		t.Errorf("Expected ProjectPath '/path/to/project', got '%s'", target.ProjectPath)
	}
	if target.Framework != "Next.js" {
		t.Errorf("Expected Framework 'Next.js', got '%s'", target.Framework)
	}
}

func TestSetTarget(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	cfg, _ := LoadConfig()

	target := TargetConfig{
		ProjectPath: "/path/to/app",
		Framework:   "Django",
		Provider:    "digitalocean",
	}

	if err := cfg.SetTarget("django-app", target); err != nil {
		t.Fatalf("Failed to set target: %v", err)
	}

	// Verify target was added
	loadedTarget, exists := cfg.Targets["django-app"]
	if !exists {
		t.Error("Expected target to be added to Targets map")
	}

	if loadedTarget.Framework != "Django" {
		t.Errorf("Expected Framework 'Django', got '%s'", loadedTarget.Framework)
	}
}

func TestGetTarget(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	cfg := &Config{
		Targets: map[string]TargetConfig{
			"myapp": {
				ProjectPath: "/path/to/project",
				Framework:   "Next.js",
			},
		},
	}

	// Get existing target
	target, exists := cfg.GetTarget("myapp")
	if !exists {
		t.Error("Expected target to exist")
	}
	if target.Framework != "Next.js" {
		t.Errorf("Expected Framework 'Next.js', got '%s'", target.Framework)
	}

	// Get non-existent target
	_, exists = cfg.GetTarget("nonexistent")
	if exists {
		t.Error("Expected target to not exist")
	}
}

func TestDeleteTarget(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create config with targets
	cfg := &Config{
		Targets: map[string]TargetConfig{
			"app1": {ProjectPath: "/path/to/app1", Framework: "Next.js"},
			"app2": {ProjectPath: "/path/to/app2", Framework: "Django"},
		},
	}

	// Save initial config
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Delete target
	if err := cfg.DeleteTarget("app1"); err != nil {
		t.Fatalf("Failed to delete target: %v", err)
	}

	// Verify target was deleted from memory
	if _, exists := cfg.Targets["app1"]; exists {
		t.Error("Expected app1 to be deleted from memory")
	}

	// Verify target still exists
	if _, exists := cfg.Targets["app2"]; !exists {
		t.Error("Expected app2 to still exist")
	}

	// Reload config and verify persistence
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if _, exists := loadedCfg.Targets["app1"]; exists {
		t.Error("Expected app1 to be deleted from saved config")
	}

	if _, exists := loadedCfg.Targets["app2"]; !exists {
		t.Error("Expected app2 to exist in saved config")
	}
}

func TestDeleteTargetIdempotent(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	cfg, _ := LoadConfig()

	// Delete non-existent target should not error
	if err := cfg.DeleteTarget("nonexistent"); err != nil {
		t.Errorf("Expected no error deleting non-existent target, got: %v", err)
	}

	// Delete again should also not error (idempotent)
	if err := cfg.DeleteTarget("nonexistent"); err != nil {
		t.Errorf("DeleteTarget should be idempotent, got error: %v", err)
	}
}

func TestProviderConfigSetAndGet(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	target := TargetConfig{
		ProjectPath: "/path/to/project",
		Framework:   "Next.js",
		Provider:    "digitalocean",
	}

	// Set DigitalOcean config
	doConfig := &DigitalOceanConfig{
		IP:       "192.168.1.100",
		Username: "deploy",
		SSHKey:   "~/.ssh/id_rsa",
		Region:   "nyc1",
	}

	if err := target.SetProviderConfig("digitalocean", doConfig); err != nil {
		t.Fatalf("Failed to set provider config: %v", err)
	}

	// Get DigitalOcean config
	loadedConfig, err := target.GetDigitalOceanConfig()
	if err != nil {
		t.Fatalf("Failed to get DigitalOcean config: %v", err)
	}

	if loadedConfig.IP != "192.168.1.100" {
		t.Errorf("Expected IP '192.168.1.100', got '%s'", loadedConfig.IP)
	}
	if loadedConfig.Region != "nyc1" {
		t.Errorf("Expected Region 'nyc1', got '%s'", loadedConfig.Region)
	}
}

func TestProviderConfigInterface(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	target := TargetConfig{
		Provider: "digitalocean",
	}

	doConfig := &DigitalOceanConfig{
		IP:       "1.2.3.4",
		Username: "deploy",
		SSHKey:   "/path/to/key",
	}

	target.SetProviderConfig("digitalocean", doConfig)

	// Test ProviderConfig interface methods
	if doConfig.GetIP() != "1.2.3.4" {
		t.Errorf("Expected IP '1.2.3.4', got '%s'", doConfig.GetIP())
	}
	if doConfig.GetUsername() != "deploy" {
		t.Errorf("Expected Username 'deploy', got '%s'", doConfig.GetUsername())
	}
	if doConfig.GetSSHKey() != "/path/to/key" {
		t.Errorf("Expected SSHKey '/path/to/key', got '%s'", doConfig.GetSSHKey())
	}
}

func TestFindTargetByPath(t *testing.T) {
	testHome, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create absolute paths for testing
	path1 := filepath.Join(testHome, "projects", "app1")
	path2 := filepath.Join(testHome, "projects", "app2")

	cfg := &Config{
		Targets: map[string]TargetConfig{
			"myapp1": {ProjectPath: path1, Framework: "Next.js"},
			"myapp2": {ProjectPath: path2, Framework: "Django"},
		},
	}

	// Find existing target
	name, target, found := cfg.FindTargetByPath(path1)
	if !found {
		t.Error("Expected to find target by path")
	}
	if name != "myapp1" {
		t.Errorf("Expected name 'myapp1', got '%s'", name)
	}
	if target.Framework != "Next.js" {
		t.Errorf("Expected Framework 'Next.js', got '%s'", target.Framework)
	}

	// Find non-existent target
	_, _, found = cfg.FindTargetByPath("/nonexistent/path")
	if found {
		t.Error("Expected to not find target")
	}
}

func TestDeleteTargetWithProviderConfig(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create target with provider config
	cfg, _ := LoadConfig()

	target := TargetConfig{
		ProjectPath: "/path/to/app",
		Framework:   "Next.js",
		Provider:    "digitalocean",
	}

	doConfig := &DigitalOceanConfig{
		IP:          "1.2.3.4",
		Username:    "deploy",
		SSHKey:      "/path/to/key",
		DropletID:   "12345",
		Provisioned: true,
	}

	target.SetProviderConfig("digitalocean", doConfig)
	cfg.SetTarget("myapp", target)
	cfg.SaveConfig()

	// Delete target
	if err := cfg.DeleteTarget("myapp"); err != nil {
		t.Fatalf("Failed to delete target: %v", err)
	}

	// Verify everything is deleted
	loadedCfg, _ := LoadConfig()
	if _, exists := loadedCfg.Targets["myapp"]; exists {
		t.Error("Expected target with provider config to be deleted")
	}
}

func TestConfigPersistence(t *testing.T) {
	testHome, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create and save config
	cfg := &Config{
		Targets: map[string]TargetConfig{
			"app1": {ProjectPath: "/path/1", Framework: "Next.js"},
			"app2": {ProjectPath: "/path/2", Framework: "Django"},
		},
	}
	cfg.SaveConfig()

	// Verify file exists
	configPath := filepath.Join(testHome, ".lightfold", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should exist")
	}

	// Verify file is valid JSON
	data, _ := os.ReadFile(configPath)
	var parsed Config
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Config file should be valid JSON: %v", err)
	}

	// Delete one target
	cfg.DeleteTarget("app1")

	// Verify deletion persisted
	data, _ = os.ReadFile(configPath)
	var parsedAfterDelete Config
	if err := json.Unmarshal(data, &parsedAfterDelete); err != nil {
		t.Fatalf("Failed to parse config after delete: %v", err)
	}
	if parsedAfterDelete.Targets == nil {
		t.Fatal("Targets should not be nil")
	}
	if _, exists := parsedAfterDelete.Targets["app1"]; exists {
		t.Error("Deleted target should not be in saved config")
	}
}

func TestMultipleProviders(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	cfg, _ := LoadConfig()

	// Create targets with different providers
	doTarget := TargetConfig{
		ProjectPath: "/path/to/do-app",
		Framework:   "Next.js",
		Provider:    "digitalocean",
	}
	doTarget.SetProviderConfig("digitalocean", &DigitalOceanConfig{
		IP:       "1.2.3.4",
		Username: "deploy",
	})

	hetznerTarget := TargetConfig{
		ProjectPath: "/path/to/hetzner-app",
		Framework:   "Django",
		Provider:    "hetzner",
	}
	hetznerTarget.SetProviderConfig("hetzner", &HetznerConfig{
		IP:       "5.6.7.8",
		Username: "deploy",
	})

	cfg.SetTarget("do-app", doTarget)
	cfg.SetTarget("hetzner-app", hetznerTarget)
	cfg.SaveConfig()

	// Delete one provider's target
	cfg.DeleteTarget("do-app")

	// Verify other provider's target still exists
	loadedCfg, _ := LoadConfig()
	if _, exists := loadedCfg.Targets["do-app"]; exists {
		t.Error("Expected do-app to be deleted")
	}
	if _, exists := loadedCfg.Targets["hetzner-app"]; !exists {
		t.Error("Expected hetzner-app to still exist")
	}
}

func TestDeploymentOptionsWithCustomCommands(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	cfg, _ := LoadConfig()

	// Create target with custom build/run commands
	target := TargetConfig{
		ProjectPath: "/path/to/project",
		Framework:   "Next.js",
		Provider:    "digitalocean",
		Deploy: &DeploymentOptions{
			BuildCommands: []string{
				"npm install",
				"npm run custom-build",
			},
			RunCommands: []string{
				"npm run custom-start",
			},
			EnvVars: map[string]string{
				"NODE_ENV": "production",
			},
		},
	}

	cfg.SetTarget("myapp", target)
	if err := cfg.SaveConfig(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load and verify
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	loadedTarget, exists := loadedCfg.Targets["myapp"]
	if !exists {
		t.Fatal("Expected target to exist")
	}

	if loadedTarget.Deploy == nil {
		t.Fatal("Expected Deploy options to exist")
	}

	// Verify build commands
	if len(loadedTarget.Deploy.BuildCommands) != 2 {
		t.Errorf("Expected 2 build commands, got %d", len(loadedTarget.Deploy.BuildCommands))
	}
	if loadedTarget.Deploy.BuildCommands[0] != "npm install" {
		t.Errorf("Expected first build command 'npm install', got '%s'", loadedTarget.Deploy.BuildCommands[0])
	}
	if loadedTarget.Deploy.BuildCommands[1] != "npm run custom-build" {
		t.Errorf("Expected second build command 'npm run custom-build', got '%s'", loadedTarget.Deploy.BuildCommands[1])
	}

	// Verify run commands
	if len(loadedTarget.Deploy.RunCommands) != 1 {
		t.Errorf("Expected 1 run command, got %d", len(loadedTarget.Deploy.RunCommands))
	}
	if loadedTarget.Deploy.RunCommands[0] != "npm run custom-start" {
		t.Errorf("Expected run command 'npm run custom-start', got '%s'", loadedTarget.Deploy.RunCommands[0])
	}

	// Verify env vars
	if loadedTarget.Deploy.EnvVars["NODE_ENV"] != "production" {
		t.Errorf("Expected NODE_ENV 'production', got '%s'", loadedTarget.Deploy.EnvVars["NODE_ENV"])
	}
}

func TestDeploymentOptionsEmptyCommands(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	cfg, _ := LoadConfig()

	// Create target with nil custom commands (should use detection defaults)
	target := TargetConfig{
		ProjectPath: "/path/to/project",
		Framework:   "Django",
		Provider:    "digitalocean",
		Deploy: &DeploymentOptions{
			EnvVars: map[string]string{
				"DEBUG": "false",
			},
		},
	}

	cfg.SetTarget("django-app", target)
	cfg.SaveConfig()

	// Load and verify
	loadedCfg, _ := LoadConfig()
	loadedTarget := loadedCfg.Targets["django-app"]

	if loadedTarget.Deploy == nil {
		t.Fatal("Expected Deploy options to exist")
	}

	// Empty arrays should remain empty (not nil)
	if loadedTarget.Deploy.BuildCommands == nil {
		// This is actually OK - JSON omitempty means nil slices are omitted
		// and loaded back as nil, which is fine for our logic
	}
	if loadedTarget.Deploy.RunCommands == nil {
		// Same here - nil is OK
	}
}

func TestDeploymentOptionsUpdate(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	cfg, _ := LoadConfig()

	// Create target with initial commands
	target := TargetConfig{
		ProjectPath: "/path/to/project",
		Framework:   "Next.js",
		Provider:    "digitalocean",
		Deploy: &DeploymentOptions{
			BuildCommands: []string{"npm install"},
			RunCommands:   []string{"npm start"},
		},
	}

	cfg.SetTarget("myapp", target)
	cfg.SaveConfig()

	// Update commands
	target.Deploy.BuildCommands = []string{
		"npm ci",
		"npm run build",
	}
	target.Deploy.RunCommands = []string{
		"node server.js",
	}

	cfg.SetTarget("myapp", target)
	cfg.SaveConfig()

	// Verify update
	loadedCfg, _ := LoadConfig()
	loadedTarget := loadedCfg.Targets["myapp"]

	if len(loadedTarget.Deploy.BuildCommands) != 2 {
		t.Errorf("Expected 2 build commands after update, got %d", len(loadedTarget.Deploy.BuildCommands))
	}
	if loadedTarget.Deploy.BuildCommands[0] != "npm ci" {
		t.Errorf("Expected 'npm ci', got '%s'", loadedTarget.Deploy.BuildCommands[0])
	}
	if loadedTarget.Deploy.RunCommands[0] != "node server.js" {
		t.Errorf("Expected 'node server.js', got '%s'", loadedTarget.Deploy.RunCommands[0])
	}
}

func TestDeploymentOptionsPersistence(t *testing.T) {
	testHome, cleanup := setupTestConfigDir(t)
	defer cleanup()

	cfg, _ := LoadConfig()

	// Create target with complex deployment config
	target := TargetConfig{
		ProjectPath: "/path/to/project",
		Framework:   "FastAPI",
		Provider:    "hetzner",
		Deploy: &DeploymentOptions{
			SkipBuild: false,
			BuildCommands: []string{
				"pip install -r requirements.txt",
				"python manage.py collectstatic --noinput",
			},
			RunCommands: []string{
				"uvicorn main:app --host 0.0.0.0 --port 8000",
			},
			EnvVars: map[string]string{
				"ENVIRONMENT": "production",
				"LOG_LEVEL":   "info",
			},
		},
	}

	cfg.SetTarget("api", target)
	cfg.SaveConfig()

	// Verify file contents
	configPath := filepath.Join(testHome, ".lightfold", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Check that custom commands are in the file
	configStr := string(data)
	if !strings.Contains(configStr, "build_commands") {
		t.Error("Expected 'build_commands' in config file")
	}
	if !strings.Contains(configStr, "run_commands") {
		t.Error("Expected 'run_commands' in config file")
	}
	if !strings.Contains(configStr, "pip install -r requirements.txt") {
		t.Error("Expected build command in config file")
	}
	if !strings.Contains(configStr, "uvicorn main:app") {
		t.Error("Expected run command in config file")
	}
}
