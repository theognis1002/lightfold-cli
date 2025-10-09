package state

import (
	"lightfold/pkg/state"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServerStateOperations(t *testing.T) {
	// Use temp directory for tests
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.100"

	t.Run("GetServerState creates new state", func(t *testing.T) {
		s, err := state.GetServerState(serverIP)
		if err != nil {
			t.Fatalf("GetServerState failed: %v", err)
		}

		if s.ServerIP != serverIP {
			t.Errorf("Expected ServerIP %s, got %s", serverIP, s.ServerIP)
		}

		if len(s.DeployedApps) != 0 {
			t.Errorf("Expected 0 deployed apps, got %d", len(s.DeployedApps))
		}

		if s.NextPort != state.PortRangeStart {
			t.Errorf("Expected NextPort %d, got %d", state.PortRangeStart, s.NextPort)
		}
	})

	t.Run("SaveServerState persists data", func(t *testing.T) {
		s, _ := state.GetServerState(serverIP)
		s.Provider = "digitalocean"
		s.ServerID = "12345"
		s.ProxyType = "nginx"

		if err := state.SaveServerState(s); err != nil {
			t.Fatalf("SaveServerState failed: %v", err)
		}

		// Reload and verify
		s2, err := state.GetServerState(serverIP)
		if err != nil {
			t.Fatalf("GetServerState failed: %v", err)
		}

		if s2.Provider != "digitalocean" {
			t.Errorf("Expected Provider digitalocean, got %s", s2.Provider)
		}

		if s2.ServerID != "12345" {
			t.Errorf("Expected ServerID 12345, got %s", s2.ServerID)
		}
	})

	t.Run("RegisterApp adds app to server", func(t *testing.T) {
		app := state.DeployedApp{
			TargetName: "myapp",
			AppName:    "myapp",
			Port:       3000,
			Domain:     "myapp.example.com",
			Framework:  "Next.js",
			LastDeploy: time.Now(),
		}

		if err := state.RegisterApp(serverIP, app); err != nil {
			t.Fatalf("RegisterApp failed: %v", err)
		}

		// Verify app was added
		apps, err := state.ListAppsOnServer(serverIP)
		if err != nil {
			t.Fatalf("ListAppsOnServer failed: %v", err)
		}

		if len(apps) != 1 {
			t.Fatalf("Expected 1 app, got %d", len(apps))
		}

		if apps[0].TargetName != "myapp" {
			t.Errorf("Expected TargetName myapp, got %s", apps[0].TargetName)
		}
	})

	t.Run("RegisterApp updates existing app", func(t *testing.T) {
		app := state.DeployedApp{
			TargetName: "myapp",
			AppName:    "myapp",
			Port:       3001, // Changed port
			Domain:     "myapp.example.com",
			Framework:  "Next.js",
			LastDeploy: time.Now(),
		}

		if err := state.RegisterApp(serverIP, app); err != nil {
			t.Fatalf("RegisterApp failed: %v", err)
		}

		// Verify only one app exists with updated port
		apps, err := state.ListAppsOnServer(serverIP)
		if err != nil {
			t.Fatalf("ListAppsOnServer failed: %v", err)
		}

		if len(apps) != 1 {
			t.Fatalf("Expected 1 app, got %d", len(apps))
		}

		if apps[0].Port != 3001 {
			t.Errorf("Expected Port 3001, got %d", apps[0].Port)
		}
	})

	t.Run("GetAppFromServer retrieves specific app", func(t *testing.T) {
		app, err := state.GetAppFromServer(serverIP, "myapp")
		if err != nil {
			t.Fatalf("GetAppFromServer failed: %v", err)
		}

		if app.TargetName != "myapp" {
			t.Errorf("Expected TargetName myapp, got %s", app.TargetName)
		}
	})

	t.Run("GetAppFromServer returns error for nonexistent app", func(t *testing.T) {
		_, err := state.GetAppFromServer(serverIP, "nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent app, got nil")
		}
	})

	t.Run("UnregisterApp removes app", func(t *testing.T) {
		if err := state.UnregisterApp(serverIP, "myapp"); err != nil {
			t.Fatalf("UnregisterApp failed: %v", err)
		}

		apps, err := state.ListAppsOnServer(serverIP)
		if err != nil {
			t.Fatalf("ListAppsOnServer failed: %v", err)
		}

		if len(apps) != 0 {
			t.Errorf("Expected 0 apps after unregister, got %d", len(apps))
		}
	})

	t.Run("DeleteServerState removes state file", func(t *testing.T) {
		// Re-add an app first
		app := state.DeployedApp{
			TargetName: "testapp",
			AppName:    "testapp",
			Port:       3000,
			Framework:  "Django",
			LastDeploy: time.Now(),
		}
		state.RegisterApp(serverIP, app)

		if err := state.DeleteServerState(serverIP); err != nil {
			t.Fatalf("DeleteServerState failed: %v", err)
		}

		// Verify file is gone
		if state.ServerStateExists(serverIP) {
			t.Error("Server state file still exists after deletion")
		}
	})
}

func TestMultipleAppsOnServer(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.200"

	// Add multiple apps
	apps := []state.DeployedApp{
		{TargetName: "app1", AppName: "app1", Port: 3000, Framework: "Next.js", LastDeploy: time.Now()},
		{TargetName: "app2", AppName: "app2", Port: 3001, Framework: "Django", LastDeploy: time.Now()},
		{TargetName: "app3", AppName: "app3", Port: 3002, Framework: "Express", LastDeploy: time.Now()},
	}

	for _, app := range apps {
		if err := state.RegisterApp(serverIP, app); err != nil {
			t.Fatalf("RegisterApp failed for %s: %v", app.TargetName, err)
		}
	}

	// Verify all apps are registered
	registeredApps, err := state.ListAppsOnServer(serverIP)
	if err != nil {
		t.Fatalf("ListAppsOnServer failed: %v", err)
	}

	if len(registeredApps) != 3 {
		t.Fatalf("Expected 3 apps, got %d", len(registeredApps))
	}

	// Remove middle app
	if err := state.UnregisterApp(serverIP, "app2"); err != nil {
		t.Fatalf("UnregisterApp failed: %v", err)
	}

	// Verify only 2 apps remain
	registeredApps, err = state.ListAppsOnServer(serverIP)
	if err != nil {
		t.Fatalf("ListAppsOnServer failed: %v", err)
	}

	if len(registeredApps) != 2 {
		t.Fatalf("Expected 2 apps after removal, got %d", len(registeredApps))
	}

	// Verify correct apps remain
	found := make(map[string]bool)
	for _, app := range registeredApps {
		found[app.TargetName] = true
	}

	if !found["app1"] || !found["app3"] {
		t.Error("Expected app1 and app3 to remain after removing app2")
	}

	if found["app2"] {
		t.Error("app2 should have been removed")
	}
}

func TestListAllServers(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create multiple server states
	serverIPs := []string{"192.168.1.100", "192.168.1.101", "192.168.1.102"}

	for _, ip := range serverIPs {
		s, _ := state.GetServerState(ip)
		s.Provider = "digitalocean"
		if err := state.SaveServerState(s); err != nil {
			t.Fatalf("SaveServerState failed for %s: %v", ip, err)
		}
	}

	// List all servers
	servers, err := state.ListAllServers()
	if err != nil {
		t.Fatalf("ListAllServers failed: %v", err)
	}

	if len(servers) != 3 {
		t.Fatalf("Expected 3 servers, got %d", len(servers))
	}

	// Verify all server IPs are present (sanitized format)
	found := make(map[string]bool)
	for _, server := range servers {
		found[server] = true
	}

	for _, ip := range serverIPs {
		sanitized := filepath.Base(ip) // Basic sanitization check
		if !found[sanitized] && !found[ip] {
			t.Errorf("Server %s not found in list", ip)
		}
	}
}

func TestUpdateAppDeployment(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.100"

	// Register an app
	app := state.DeployedApp{
		TargetName: "testapp",
		AppName:    "testapp",
		Port:       3000,
		Framework:  "Next.js",
		LastDeploy: time.Now().Add(-1 * time.Hour),
	}

	if err := state.RegisterApp(serverIP, app); err != nil {
		t.Fatalf("RegisterApp failed: %v", err)
	}

	// Update deployment time
	newTime := time.Now()
	if err := state.UpdateAppDeployment(serverIP, "testapp", newTime); err != nil {
		t.Fatalf("UpdateAppDeployment failed: %v", err)
	}

	// Verify update
	updatedApp, err := state.GetAppFromServer(serverIP, "testapp")
	if err != nil {
		t.Fatalf("GetAppFromServer failed: %v", err)
	}

	if updatedApp.LastDeploy.Unix() != newTime.Unix() {
		t.Errorf("Expected LastDeploy to be updated to %v, got %v", newTime, updatedApp.LastDeploy)
	}
}
