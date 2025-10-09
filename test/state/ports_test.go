package state

import (
	"lightfold/pkg/state"
	"os"
	"testing"
)

func TestPortAllocation(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.100"

	t.Run("AllocatePort returns sequential ports", func(t *testing.T) {
		port1, err := state.AllocatePort(serverIP)
		if err != nil {
			t.Fatalf("AllocatePort failed: %v", err)
		}

		if port1 != state.PortRangeStart {
			t.Errorf("Expected first port to be %d, got %d", state.PortRangeStart, port1)
		}

		port2, err := state.AllocatePort(serverIP)
		if err != nil {
			t.Fatalf("AllocatePort failed: %v", err)
		}

		if port2 != state.PortRangeStart+1 {
			t.Errorf("Expected second port to be %d, got %d", state.PortRangeStart+1, port2)
		}
	})

	t.Run("IsPortAvailable detects used ports", func(t *testing.T) {
		// Register an app with a specific port
		app := state.DeployedApp{
			TargetName: "testapp",
			Port:       3005,
			Framework:  "Next.js",
		}

		if err := state.RegisterApp(serverIP, app); err != nil {
			t.Fatalf("RegisterApp failed: %v", err)
		}

		// Check if port is available
		available, err := state.IsPortAvailable(serverIP, 3005)
		if err != nil {
			t.Fatalf("IsPortAvailable failed: %v", err)
		}

		if available {
			t.Error("Expected port 3005 to be unavailable")
		}

		// Check an unused port
		available, err = state.IsPortAvailable(serverIP, 3010)
		if err != nil {
			t.Fatalf("IsPortAvailable failed: %v", err)
		}

		if !available {
			t.Error("Expected port 3010 to be available")
		}
	})

	t.Run("IsPortAvailable validates port range", func(t *testing.T) {
		_, err := state.IsPortAvailable(serverIP, 2000) // Below range
		if err == nil {
			t.Error("Expected error for port below range")
		}

		_, err = state.IsPortAvailable(serverIP, 10000) // Above range
		if err == nil {
			t.Error("Expected error for port above range")
		}
	})
}

func TestGetAppPort(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.100"

	// Register apps with specific ports
	apps := []state.DeployedApp{
		{TargetName: "app1", Port: 3000, Framework: "Next.js"},
		{TargetName: "app2", Port: 3001, Framework: "Django"},
		{TargetName: "app3", Port: 3002, Framework: "Express"},
	}

	for _, app := range apps {
		if err := state.RegisterApp(serverIP, app); err != nil {
			t.Fatalf("RegisterApp failed: %v", err)
		}
	}

	t.Run("GetAppPort returns correct port", func(t *testing.T) {
		port, err := state.GetAppPort(serverIP, "app2")
		if err != nil {
			t.Fatalf("GetAppPort failed: %v", err)
		}

		if port != 3001 {
			t.Errorf("Expected port 3001, got %d", port)
		}
	})

	t.Run("GetAppPort returns error for nonexistent app", func(t *testing.T) {
		_, err := state.GetAppPort(serverIP, "nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent app")
		}
	})
}

func TestPortConflictDetection(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.100"

	t.Run("DetectPortConflicts finds conflicts", func(t *testing.T) {
		// Manually create conflicting apps (normally prevented by allocation logic)
		s, _ := state.GetServerState(serverIP)
		s.DeployedApps = []state.DeployedApp{
			{TargetName: "app1", Port: 3000, Framework: "Next.js"},
			{TargetName: "app2", Port: 3000, Framework: "Django"}, // Conflict!
			{TargetName: "app3", Port: 3001, Framework: "Express"},
		}
		state.SaveServerState(s)

		conflicts, err := state.DetectPortConflicts(serverIP)
		if err != nil {
			t.Fatalf("DetectPortConflicts failed: %v", err)
		}

		if len(conflicts) != 1 {
			t.Fatalf("Expected 1 conflict, got %d", len(conflicts))
		}
	})

	t.Run("DetectPortConflicts returns empty for no conflicts", func(t *testing.T) {
		// Create server with no conflicts
		serverIP2 := "192.168.1.101"
		apps := []state.DeployedApp{
			{TargetName: "app1", Port: 3000, Framework: "Next.js"},
			{TargetName: "app2", Port: 3001, Framework: "Django"},
		}

		for _, app := range apps {
			state.RegisterApp(serverIP2, app)
		}

		conflicts, err := state.DetectPortConflicts(serverIP2)
		if err != nil {
			t.Fatalf("DetectPortConflicts failed: %v", err)
		}

		if len(conflicts) != 0 {
			t.Errorf("Expected no conflicts, got %d", len(conflicts))
		}
	})
}

func TestPortStatistics(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.100"

	// Register several apps
	for i := 0; i < 5; i++ {
		app := state.DeployedApp{
			TargetName: "app" + string(rune('1'+i)),
			Port:       3000 + i,
			Framework:  "Next.js",
		}
		if err := state.RegisterApp(serverIP, app); err != nil {
			t.Fatalf("RegisterApp failed: %v", err)
		}
	}

	used, available, err := state.GetPortStatistics(serverIP)
	if err != nil {
		t.Fatalf("GetPortStatistics failed: %v", err)
	}

	if used != 5 {
		t.Errorf("Expected 5 used ports, got %d", used)
	}

	expectedAvailable := (state.PortRangeEnd - state.PortRangeStart + 1) - 5
	if available != expectedAvailable {
		t.Errorf("Expected %d available ports, got %d", expectedAvailable, available)
	}
}

func TestPortReuse(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.100"

	// Allocate ports
	port1, _ := state.AllocatePort(serverIP)
	port2, _ := state.AllocatePort(serverIP)
	port3, _ := state.AllocatePort(serverIP)

	// Register apps
	apps := []state.DeployedApp{
		{TargetName: "app1", Port: port1, Framework: "Next.js"},
		{TargetName: "app2", Port: port2, Framework: "Django"},
		{TargetName: "app3", Port: port3, Framework: "Express"},
	}

	for _, app := range apps {
		state.RegisterApp(serverIP, app)
	}

	// Remove middle app
	state.UnregisterApp(serverIP, "app2")

	// Allocate new port - should skip to next available
	port4, err := state.AllocatePort(serverIP)
	if err != nil {
		t.Fatalf("AllocatePort failed: %v", err)
	}

	// Port allocation should continue from NextPort, not reuse port2 immediately
	// unless NextPort is reset (which happens when a port is released that's < NextPort)
	if port4 < port1 || port4 < port3 {
		t.Errorf("Expected port4 to be >= existing ports, got %d", port4)
	}
}

func TestPortExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping port exhaustion test in short mode")
	}

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverIP := "192.168.1.100"

	// Fill up all ports in the range (this would take a while in reality)
	// For testing, we'll use a smaller range by manually creating apps
	totalPorts := state.PortRangeEnd - state.PortRangeStart + 1

	// Create a scenario with many ports used (simulate near-exhaustion)
	s, _ := state.GetServerState(serverIP)
	for i := 0; i < 100; i++ { // Use 100 ports
		app := state.DeployedApp{
			TargetName: "app" + string(rune('0'+i/10)) + string(rune('0'+i%10)),
			Port:       state.PortRangeStart + i,
			Framework:  "Test",
		}
		s.DeployedApps = append(s.DeployedApps, app)
	}
	state.SaveServerState(s)

	// Verify we can still allocate
	port, err := state.AllocatePort(serverIP)
	if err != nil {
		t.Fatalf("AllocatePort failed when ports should be available: %v", err)
	}

	if port < state.PortRangeStart || port > state.PortRangeEnd {
		t.Errorf("Allocated port %d is outside valid range", port)
	}

	// Verify we get an error when truly exhausted
	s, _ = state.GetServerState(serverIP)
	s.DeployedApps = []state.DeployedApp{}
	for i := state.PortRangeStart; i <= state.PortRangeEnd; i++ {
		app := state.DeployedApp{
			TargetName: "app_" + string(rune(i)),
			Port:       i,
			Framework:  "Test",
		}
		s.DeployedApps = append(s.DeployedApps, app)
	}
	state.SaveServerState(s)

	_, err = state.AllocatePort(serverIP)
	if err == nil {
		t.Error("Expected error when all ports are exhausted")
	}

	t.Logf("Port exhaustion correctly detected (total ports: %d)", totalPorts)
}
