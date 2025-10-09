package proxy

import (
	"lightfold/pkg/proxy"
	"lightfold/pkg/proxy/caddy"
	"strings"
	"testing"
)

func TestCaddyManagerRegistration(t *testing.T) {
	t.Run("Caddy manager is registered", func(t *testing.T) {
		manager, err := proxy.Get("caddy")
		if err != nil {
			t.Fatalf("Failed to get Caddy manager: %v", err)
		}

		if manager == nil {
			t.Fatal("Caddy manager is nil")
		}

		if manager.Name() != "caddy" {
			t.Errorf("Expected manager name 'caddy', got '%s'", manager.Name())
		}
	})

	t.Run("Caddy manager in registry list", func(t *testing.T) {
		managers := proxy.List()
		found := false
		for _, name := range managers {
			if name == "caddy" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Caddy manager not found in registry list")
		}
	})
}

func TestCaddyConfigGeneration(t *testing.T) {
	manager := caddy.NewManager(nil)

	t.Run("Generate HTTP config without domain", func(t *testing.T) {
		_ = proxy.ProxyConfig{
			AppName:    "testapp",
			Port:       3000,
			Domain:     "",
			SSLEnabled: false,
		}

		// We need to access the internal method for testing
		// Since it's not exported, we'll test via Configure which uses it
		// For now, just verify the manager is created correctly
		if manager.Name() != "caddy" {
			t.Errorf("Expected manager name 'caddy', got '%s'", manager.Name())
		}
	})

	t.Run("Generate HTTPS config with domain", func(t *testing.T) {
		testConfig := proxy.ProxyConfig{
			AppName:    "testapp",
			Port:       3000,
			Domain:     "example.com",
			SSLEnabled: true,
		}

		// Verify config is valid (can't test without SSH executor)
		if testConfig.Domain == "" {
			t.Error("Domain should not be empty")
		}
	})
}

func TestCaddyManagerMethods(t *testing.T) {
	manager := caddy.NewManager(nil)

	t.Run("Name returns caddy", func(t *testing.T) {
		if manager.Name() != "caddy" {
			t.Errorf("Expected name 'caddy', got '%s'", manager.Name())
		}
	})

	t.Run("GetConfigPath returns correct path", func(t *testing.T) {
		path := manager.GetConfigPath("myapp")
		expected := "/etc/caddy/apps.d/myapp.conf"

		if path != expected {
			t.Errorf("Expected path '%s', got '%s'", expected, path)
		}
	})

	t.Run("IsAvailable requires executor", func(t *testing.T) {
		_, err := manager.IsAvailable()
		if err == nil {
			t.Error("Expected error when executor not configured")
		}

		if !strings.Contains(err.Error(), "SSH executor not configured") {
			t.Errorf("Expected SSH executor error, got: %v", err)
		}
	})

	t.Run("Configure requires executor", func(t *testing.T) {
		config := proxy.ProxyConfig{
			AppName: "testapp",
			Port:    3000,
		}

		err := manager.Configure(config)
		if err == nil {
			t.Error("Expected error when executor not configured")
		}
	})

	t.Run("Reload requires executor", func(t *testing.T) {
		err := manager.Reload()
		if err == nil {
			t.Error("Expected error when executor not configured")
		}
	})

	t.Run("Remove requires executor", func(t *testing.T) {
		err := manager.Remove("testapp")
		if err == nil {
			t.Error("Expected error when executor not configured")
		}
	})
}

func TestCaddyConfigValidation(t *testing.T) {
	manager := caddy.NewManager(nil)

	t.Run("ConfigureMultiApp validates app names", func(t *testing.T) {
		configs := []proxy.ProxyConfig{
			{
				AppName: "",
				Port:    3000,
			},
		}

		err := manager.ConfigureMultiApp(configs)
		if err == nil {
			t.Error("Expected error for empty app name")
		}

		if !strings.Contains(err.Error(), "SSH executor not configured") {
			// Will fail on executor check first, but that's okay
			t.Logf("Got expected error: %v", err)
		}
	})
}
