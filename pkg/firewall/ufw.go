package firewall

import (
	"fmt"
	"lightfold/pkg/ssh"
	"strconv"
	"strings"
)

func init() {
	Register("ufw", func(executor *ssh.Executor) Manager {
		return &UFWManager{executor: executor}
	})
}

// UFWManager implements firewall management using UFW (Uncomplicated Firewall)
type UFWManager struct {
	executor *ssh.Executor
}

// NewUFWManager creates a new UFW firewall manager
func NewUFWManager(executor *ssh.Executor) *UFWManager {
	return &UFWManager{executor: executor}
}

// Name returns the name of this firewall manager
func (m *UFWManager) Name() string {
	return "ufw"
}

// IsAvailable checks if UFW is installed and available
func (m *UFWManager) IsAvailable() (bool, error) {
	if m.executor == nil {
		return false, fmt.Errorf("SSH executor not configured")
	}

	result := m.executor.Execute("which ufw")
	if result.Error != nil {
		return false, result.Error
	}

	return result.ExitCode == 0, nil
}

// OpenPort opens a port in the firewall
func (m *UFWManager) OpenPort(port int) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d", port)
	}

	// Check if UFW is available
	available, err := m.IsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check UFW availability: %w", err)
	}
	if !available {
		return fmt.Errorf("UFW is not installed on the server")
	}

	// Add the rule
	cmd := fmt.Sprintf("ufw allow %d/tcp", port)
	result := m.executor.ExecuteSudo(cmd)
	if result.Error != nil {
		return fmt.Errorf("failed to open port %d: %w", port, result.Error)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to open port %d (exit code %d): %s", port, result.ExitCode, result.Stderr)
	}

	// Reload UFW to apply changes
	result = m.executor.ExecuteSudo("ufw reload")
	if result.Error != nil {
		return fmt.Errorf("failed to reload UFW: %w", result.Error)
	}

	return nil
}

// ClosePort closes a port in the firewall
func (m *UFWManager) ClosePort(port int) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d", port)
	}

	// Check if UFW is available
	available, err := m.IsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check UFW availability: %w", err)
	}
	if !available {
		return fmt.Errorf("UFW is not installed on the server")
	}

	// Delete the rule
	cmd := fmt.Sprintf("ufw delete allow %d/tcp", port)
	result := m.executor.ExecuteSudo(cmd)
	if result.Error != nil {
		return fmt.Errorf("failed to close port %d: %w", port, result.Error)
	}

	if result.ExitCode != 0 {
		// It's okay if the rule doesn't exist
		if strings.Contains(result.Stderr, "Could not delete") {
			return nil
		}
		return fmt.Errorf("failed to close port %d (exit code %d): %s", port, result.ExitCode, result.Stderr)
	}

	// Reload UFW to apply changes
	result = m.executor.ExecuteSudo("ufw reload")
	if result.Error != nil {
		return fmt.Errorf("failed to reload UFW: %w", result.Error)
	}

	return nil
}

// IsPortOpen checks if a port is open
func (m *UFWManager) IsPortOpen(port int) (bool, error) {
	if m.executor == nil {
		return false, fmt.Errorf("SSH executor not configured")
	}

	if port < 1 || port > 65535 {
		return false, fmt.Errorf("invalid port number: %d", port)
	}

	// Check if UFW is available
	available, err := m.IsAvailable()
	if err != nil {
		return false, fmt.Errorf("failed to check UFW availability: %w", err)
	}
	if !available {
		return false, fmt.Errorf("UFW is not installed on the server")
	}

	// Check UFW status for the port
	result := m.executor.ExecuteSudo("ufw status numbered")
	if result.Error != nil {
		return false, fmt.Errorf("failed to check UFW status: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return false, fmt.Errorf("failed to check UFW status (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	// Parse output to check if port is open
	portStr := fmt.Sprintf("%d/tcp", port)
	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, portStr) && strings.Contains(line, "ALLOW") {
			return true, nil
		}
	}

	return false, nil
}

// ListOpenPorts returns all open TCP ports
func (m *UFWManager) ListOpenPorts() ([]int, error) {
	if m.executor == nil {
		return nil, fmt.Errorf("SSH executor not configured")
	}

	// Check if UFW is available
	available, err := m.IsAvailable()
	if err != nil {
		return nil, fmt.Errorf("failed to check UFW availability: %w", err)
	}
	if !available {
		return nil, fmt.Errorf("UFW is not installed on the server")
	}

	// Get UFW status
	result := m.executor.ExecuteSudo("ufw status numbered")
	if result.Error != nil {
		return nil, fmt.Errorf("failed to check UFW status: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to check UFW status (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	// Parse output to extract open ports
	var ports []int
	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "ALLOW") {
			continue
		}

		// Extract port number from lines like: "[ 1] 22/tcp                     ALLOW IN    Anywhere"
		parts := strings.Fields(line)
		for _, part := range parts {
			if strings.Contains(part, "/tcp") {
				portStr := strings.TrimSuffix(part, "/tcp")
				if port, err := strconv.Atoi(portStr); err == nil {
					ports = append(ports, port)
				}
				break
			}
		}
	}

	return ports, nil
}
