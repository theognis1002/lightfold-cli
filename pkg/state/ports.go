package state

import (
	"fmt"
)

const (
	// PortRangeStart is the first port in the allocation range
	PortRangeStart = 3000
	// PortRangeEnd is the last port in the allocation range
	PortRangeEnd = 9000
)

// AllocatePort finds and allocates the next available port on a server
func AllocatePort(serverIP string) (int, error) {
	state, err := GetServerState(serverIP)
	if err != nil {
		return 0, fmt.Errorf("failed to get server state: %w", err)
	}

	// Initialize NextPort if not set
	if state.NextPort == 0 {
		state.NextPort = PortRangeStart
	}

	// Build map of used ports
	usedPorts := make(map[int]bool)
	for _, app := range state.DeployedApps {
		usedPorts[app.Port] = true
	}

	// Find next available port starting from NextPort
	port := state.NextPort
	for {
		// Check if port is available
		if !usedPorts[port] {
			// Found available port
			state.NextPort = port + 1
			if err := SaveServerState(state); err != nil {
				return 0, fmt.Errorf("failed to save server state: %w", err)
			}
			return port, nil
		}

		// Try next port
		port++
		if port > PortRangeEnd {
			// Wrap around and search from beginning
			port = PortRangeStart
		}

		// Check if we've exhausted all ports
		if port == state.NextPort {
			return 0, fmt.Errorf("no available ports in range %d-%d (all %d ports are in use)", PortRangeStart, PortRangeEnd, len(usedPorts))
		}
	}
}

// GetAppPort retrieves the port for an existing app
func GetAppPort(serverIP, targetName string) (int, error) {
	state, err := GetServerState(serverIP)
	if err != nil {
		return 0, fmt.Errorf("failed to get server state: %w", err)
	}

	for _, app := range state.DeployedApps {
		if app.TargetName == targetName {
			return app.Port, nil
		}
	}

	return 0, fmt.Errorf("app not found on server: %s", targetName)
}

// IsPortAvailable checks if a specific port is available on a server
func IsPortAvailable(serverIP string, port int) (bool, error) {
	if port < PortRangeStart || port > PortRangeEnd {
		return false, fmt.Errorf("port %d is outside valid range %d-%d", port, PortRangeStart, PortRangeEnd)
	}

	state, err := GetServerState(serverIP)
	if err != nil {
		return false, fmt.Errorf("failed to get server state: %w", err)
	}

	for _, app := range state.DeployedApps {
		if app.Port == port {
			return false, nil
		}
	}

	return true, nil
}

// ReleasePort frees up a port when an app is destroyed
// Note: This is typically called as part of UnregisterApp
func ReleasePort(serverIP string, port int) error {
	state, err := GetServerState(serverIP)
	if err != nil {
		return fmt.Errorf("failed to get server state: %w", err)
	}

	// Remove any app using this port
	newApps := []DeployedApp{}
	for _, app := range state.DeployedApps {
		if app.Port != port {
			newApps = append(newApps, app)
		}
	}

	state.DeployedApps = newApps

	// If NextPort is ahead of released port, potentially reuse it
	if port < state.NextPort {
		state.NextPort = port
	}

	return SaveServerState(state)
}

// GetPortStatistics returns statistics about port usage on a server
func GetPortStatistics(serverIP string) (used int, available int, err error) {
	state, err := GetServerState(serverIP)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get server state: %w", err)
	}

	used = len(state.DeployedApps)
	totalPorts := PortRangeEnd - PortRangeStart + 1
	available = totalPorts - used

	return used, available, nil
}

// DetectPortConflicts checks for any port conflicts in server state
func DetectPortConflicts(serverIP string) ([]string, error) {
	state, err := GetServerState(serverIP)
	if err != nil {
		return nil, fmt.Errorf("failed to get server state: %w", err)
	}

	portMap := make(map[int][]string)
	for _, app := range state.DeployedApps {
		portMap[app.Port] = append(portMap[app.Port], app.TargetName)
	}

	var conflicts []string
	for port, targets := range portMap {
		if len(targets) > 1 {
			conflicts = append(conflicts, fmt.Sprintf("Port %d used by: %v", port, targets))
		}
	}

	return conflicts, nil
}
