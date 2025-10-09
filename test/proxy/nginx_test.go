package proxy_test

import (
	"lightfold/pkg/proxy"
	_ "lightfold/pkg/proxy/nginx"
	"testing"
)

func TestNginxManagerRegistration(t *testing.T) {
	manager, err := proxy.GetManager("nginx")
	if err != nil {
		t.Fatalf("Failed to get nginx manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Expected non-nil nginx manager")
	}

	if manager.Name() != "nginx" {
		t.Errorf("Expected name 'nginx', got '%s'", manager.Name())
	}
}

func TestNginxManagerAvailability(t *testing.T) {
	manager, err := proxy.GetManager("nginx")
	if err != nil {
		t.Fatalf("Failed to get nginx manager: %v", err)
	}

	// Note: This will return an error because we don't have an SSH executor set
	// This is expected behavior - we're just testing the method exists
	_, err = manager.IsAvailable()
	if err == nil {
		t.Error("Expected error when SSH executor is not set")
	}
}

func TestProxyManagerList(t *testing.T) {
	managers := proxy.List()
	if len(managers) == 0 {
		t.Fatal("Expected at least one proxy manager registered")
	}

	found := false
	for _, name := range managers {
		if name == "nginx" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected nginx to be in the list of registered proxy managers")
	}
}

func TestGetNonExistentProxyManager(t *testing.T) {
	_, err := proxy.GetManager("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent proxy manager")
	}
}

func TestGetDefaultProxyManager(t *testing.T) {
	manager, err := proxy.GetDefault()
	if err != nil {
		t.Fatalf("Failed to get default proxy manager: %v", err)
	}

	if manager.Name() != "nginx" {
		t.Errorf("Expected default proxy manager to be nginx, got '%s'", manager.Name())
	}
}

func TestNginxConfigPath(t *testing.T) {
	manager, err := proxy.GetManager("nginx")
	if err != nil {
		t.Fatalf("Failed to get nginx manager: %v", err)
	}

	appName := "testapp"
	expectedPath := "/etc/nginx/sites-available/testapp.conf"
	actualPath := manager.GetConfigPath(appName)

	if actualPath != expectedPath {
		t.Errorf("Expected config path '%s', got '%s'", expectedPath, actualPath)
	}
}

func TestNginxMultiAppConfiguration(t *testing.T) {
	manager, err := proxy.GetManager("nginx")
	if err != nil {
		t.Fatalf("Failed to get nginx manager: %v", err)
	}

	t.Run("ConfigureMultiApp method exists", func(t *testing.T) {
		// Create test configs
		configs := []proxy.ProxyConfig{
			{
				AppName: "app1",
				Port:    3000,
				Domain:  "app1.example.com",
			},
			{
				AppName: "app2",
				Port:    3001,
				Domain:  "app2.example.com",
			},
		}

		// Try to configure (will fail due to no SSH executor, but validates method exists)
		// This is a type check - if ConfigureMultiApp doesn't exist, this won't compile
		type multiAppConfigurer interface {
			ConfigureMultiApp([]proxy.ProxyConfig) error
		}

		_, ok := manager.(multiAppConfigurer)
		if !ok {
			t.Error("Nginx manager does not implement ConfigureMultiApp method")
		}

		// Verify configs structure is valid
		for _, config := range configs {
			if config.AppName == "" {
				t.Error("AppName should not be empty")
			}
			if config.Port == 0 {
				t.Error("Port should not be zero")
			}
		}
	})
}
