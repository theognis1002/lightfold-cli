package caddy

import (
	"fmt"
	"lightfold/pkg/proxy"
	"lightfold/pkg/ssh"
	"strings"
)

func init() {
	proxy.Register("caddy", func() proxy.ProxyManager {
		return &Manager{}
	})
}

// Manager implements reverse proxy management using Caddy
type Manager struct {
	executor *ssh.Executor
}

// NewManager creates a new Caddy proxy manager
func NewManager(executor *ssh.Executor) *Manager {
	return &Manager{
		executor: executor,
	}
}

// SetExecutor sets the SSH executor for remote command execution
func (m *Manager) SetExecutor(executor *ssh.Executor) {
	m.executor = executor
}

// Name returns the name of this proxy manager
func (m *Manager) Name() string {
	return "caddy"
}

// IsAvailable checks if Caddy is installed and available
func (m *Manager) IsAvailable() (bool, error) {
	if m.executor == nil {
		return false, fmt.Errorf("SSH executor not configured")
	}

	result := m.executor.Execute("which caddy")
	if result.Error != nil {
		return false, result.Error
	}

	if result.ExitCode != 0 {
		return false, nil
	}

	return true, nil
}

// Install installs Caddy on the remote server
func (m *Manager) Install() error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	// Check if already installed
	available, err := m.IsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check Caddy availability: %w", err)
	}
	if available {
		return nil // Already installed
	}

	// Install Caddy via official installation script
	installScript := `
# Install dependencies
apt-get update
apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl

# Add Caddy repository
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list

# Install Caddy
apt-get update
apt-get install -y caddy

# Ensure Caddy is enabled and started
systemctl enable caddy
systemctl start caddy
`

	result := m.executor.ExecuteSudo(installScript)
	if result.Error != nil {
		return fmt.Errorf("failed to install Caddy: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("caddy installation failed (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	return nil
}

// Configure sets up the Caddy configuration for a single application
func (m *Manager) Configure(config proxy.ProxyConfig) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	// Validate config
	if config.AppName == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	if config.Port == 0 {
		return fmt.Errorf("port cannot be zero")
	}

	// Check if Caddy is available
	available, err := m.IsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check Caddy availability: %w", err)
	}
	if !available {
		return fmt.Errorf("caddy is not installed on the server")
	}

	// Generate Caddy configuration block for this app
	caddyBlock := m.generateCaddyBlock(config)

	// Write configuration to app-specific file
	configPath := m.GetConfigPath(config.AppName)

	// Write to temp file first
	tmpFile := fmt.Sprintf("/tmp/caddy-%s.conf", config.AppName)
	escapedConfig := strings.ReplaceAll(caddyBlock, "'", "'\"'\"'")

	writeTmpCmd := fmt.Sprintf("echo '%s' > %s", escapedConfig, tmpFile)
	result := m.executor.Execute(writeTmpCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to write temp config: %w", result.Error)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to write temp config (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	// Ensure Caddy config directory exists
	mkdirCmd := "mkdir -p /etc/caddy/apps.d"
	result = m.executor.ExecuteSudo(mkdirCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to create Caddy config directory: %w", result.Error)
	}

	// Move temp file to final location with sudo
	moveCmd := fmt.Sprintf("mv %s %s", tmpFile, configPath)
	result = m.executor.ExecuteSudo(moveCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to write Caddy config: %w", result.Error)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to write Caddy config (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	// Update main Caddyfile to import app configs
	if err := m.ensureMainCaddyfile(); err != nil {
		return fmt.Errorf("failed to update main Caddyfile: %w", err)
	}

	// Reload Caddy configuration
	return m.Reload()
}

// ConfigureMultiApp sets up Caddy configuration for multiple applications at once
func (m *Manager) ConfigureMultiApp(configs []proxy.ProxyConfig) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	// Check if Caddy is available
	available, err := m.IsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check Caddy availability: %w", err)
	}
	if !available {
		return fmt.Errorf("caddy is not installed on the server")
	}

	// Ensure Caddy config directory exists
	mkdirCmd := "mkdir -p /etc/caddy/apps.d"
	result := m.executor.ExecuteSudo(mkdirCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to create Caddy config directory: %w", result.Error)
	}

	// Write each app's config
	for _, config := range configs {
		// Validate config
		if config.AppName == "" {
			return fmt.Errorf("app name cannot be empty")
		}
		if config.Port == 0 {
			return fmt.Errorf("port cannot be zero for app %s", config.AppName)
		}

		// Generate Caddy configuration block
		caddyBlock := m.generateCaddyBlock(config)

		// Write configuration to app-specific file
		configPath := m.GetConfigPath(config.AppName)
		tmpFile := fmt.Sprintf("/tmp/caddy-%s.conf", config.AppName)
		escapedConfig := strings.ReplaceAll(caddyBlock, "'", "'\"'\"'")

		writeTmpCmd := fmt.Sprintf("echo '%s' > %s", escapedConfig, tmpFile)
		result := m.executor.Execute(writeTmpCmd)
		if result.Error != nil {
			return fmt.Errorf("failed to write temp config for %s: %w", config.AppName, result.Error)
		}

		moveCmd := fmt.Sprintf("mv %s %s", tmpFile, configPath)
		result = m.executor.ExecuteSudo(moveCmd)
		if result.Error != nil {
			return fmt.Errorf("failed to write Caddy config for %s: %w", config.AppName, result.Error)
		}
	}

	// Update main Caddyfile to import app configs
	if err := m.ensureMainCaddyfile(); err != nil {
		return fmt.Errorf("failed to update main Caddyfile: %w", err)
	}

	// Reload Caddy once for all apps
	return m.Reload()
}

// Reload reloads the Caddy configuration
func (m *Manager) Reload() error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	// Validate configuration first
	result := m.executor.ExecuteSudo("caddy validate --config /etc/caddy/Caddyfile")
	if result.Error != nil {
		return fmt.Errorf("caddy config validation failed: %w", result.Error)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("caddy config validation failed: %s", result.Stderr)
	}

	// Reload via systemctl (graceful reload)
	result = m.executor.ExecuteSudo("systemctl reload caddy")
	if result.Error != nil {
		return fmt.Errorf("failed to reload Caddy: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("caddy reload failed (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	return nil
}

// Remove removes the Caddy configuration for an application
func (m *Manager) Remove(appName string) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	// Remove configuration file
	configPath := m.GetConfigPath(appName)
	result := m.executor.ExecuteSudo(fmt.Sprintf("rm -f %s", configPath))
	if result.Error != nil {
		return fmt.Errorf("failed to remove Caddy config: %w", result.Error)
	}

	// Reload Caddy to apply changes
	return m.Reload()
}

// GetConfigPath returns the path to the Caddy configuration file for an app
func (m *Manager) GetConfigPath(appName string) string {
	return fmt.Sprintf("/etc/caddy/apps.d/%s.conf", appName)
}

// ensureMainCaddyfile ensures the main Caddyfile imports app configs
func (m *Manager) ensureMainCaddyfile() error {
	mainCaddyfile := `# Lightfold Caddy Configuration
# This file is managed by Lightfold - do not edit manually

# Import all app configurations
import /etc/caddy/apps.d/*.conf
`

	tmpFile := "/tmp/Caddyfile"
	escapedConfig := strings.ReplaceAll(mainCaddyfile, "'", "'\"'\"'")

	writeTmpCmd := fmt.Sprintf("echo '%s' > %s", escapedConfig, tmpFile)
	result := m.executor.Execute(writeTmpCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to write temp Caddyfile: %w", result.Error)
	}

	moveCmd := "mv /tmp/Caddyfile /etc/caddy/Caddyfile"
	result = m.executor.ExecuteSudo(moveCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to write main Caddyfile: %w", result.Error)
	}

	return nil
}

// generateCaddyBlock generates a Caddyfile block for a single app
func (m *Manager) generateCaddyBlock(config proxy.ProxyConfig) string {
	// If no domain, use IP-based configuration (for testing/development)
	if config.Domain == "" {
		return fmt.Sprintf(`# HTTP-only configuration for %s (no domain)
:80 {
	reverse_proxy localhost:%d
	encode gzip
	log {
		output file /var/log/caddy/%s.log
	}
}
`, config.AppName, config.Port, config.AppName)
	}

	// With domain - Caddy handles SSL automatically
	return fmt.Sprintf(`# %s - Automatic HTTPS
%s {
	reverse_proxy localhost:%d
	encode gzip
	log {
		output file /var/log/caddy/%s.log
	}
}
`, config.AppName, config.Domain, config.Port, config.AppName)
}
