package nginx

import (
	"fmt"
	"lightfold/pkg/proxy"
	"lightfold/pkg/ssh"
	"strings"
)

func init() {
	proxy.Register("nginx", func() proxy.ProxyManager {
		return &Manager{}
	})
}

// Manager implements reverse proxy management using Nginx
type Manager struct {
	executor *ssh.Executor
}

// NewManager creates a new Nginx proxy manager
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
	return "nginx"
}

// IsAvailable checks if nginx is installed and available
func (m *Manager) IsAvailable() (bool, error) {
	if m.executor == nil {
		return false, fmt.Errorf("SSH executor not configured")
	}

	result := m.executor.Execute("which nginx")
	if result.Error != nil {
		return false, result.Error
	}

	if result.ExitCode != 0 {
		return false, nil
	}

	return true, nil
}

// Configure sets up the nginx configuration for the application
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

	// Check if nginx is available
	available, err := m.IsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check nginx availability: %w", err)
	}
	if !available {
		return fmt.Errorf("nginx is not installed on the server")
	}

	// Generate nginx configuration
	var nginxConfig string
	if config.SSLEnabled && config.Domain != "" {
		nginxConfig = m.generateSSLConfig(config)
	} else {
		nginxConfig = m.generateHTTPConfig(config)
	}

	// Write configuration to file
	configPath := m.GetConfigPath(config.AppName)

	// First write to a temp file, then move it with sudo
	// This avoids issues with shell redirection and sudo
	tmpFile := fmt.Sprintf("/tmp/nginx-%s.conf", config.AppName)
	escapedConfig := strings.ReplaceAll(nginxConfig, "'", "'\"'\"'")

	// Write to temp file (no sudo needed)
	writeTmpCmd := fmt.Sprintf("echo '%s' > %s", escapedConfig, tmpFile)
	result := m.executor.Execute(writeTmpCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to write temp config: %w", result.Error)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to write temp config (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	// Move temp file to final location with sudo
	moveCmd := fmt.Sprintf("mv %s %s", tmpFile, configPath)
	result = m.executor.ExecuteSudo(moveCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to write nginx config: %w", result.Error)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to write nginx config (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	// Create symlink to enable the site
	symlinkCmd := fmt.Sprintf(
		"ln -sf %s /etc/nginx/sites-enabled/%s.conf",
		configPath,
		config.AppName,
	)
	result = m.executor.ExecuteSudo(symlinkCmd)
	if result.Error != nil {
		return fmt.Errorf("failed to enable nginx site: %w", result.Error)
	}

	// Test nginx configuration
	result = m.executor.ExecuteSudo("nginx -t")
	if result.Error != nil {
		return fmt.Errorf("nginx config test failed: %w", result.Error)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("nginx config test failed: %s", result.Stderr)
	}

	return nil
}

// Reload reloads the nginx configuration
func (m *Manager) Reload() error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	result := m.executor.ExecuteSudo("systemctl reload nginx")
	if result.Error != nil {
		return fmt.Errorf("failed to reload nginx: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("nginx reload failed (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	return nil
}

// ConfigureMultiApp sets up nginx configuration for multiple applications at once
func (m *Manager) ConfigureMultiApp(configs []proxy.ProxyConfig) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	// Check if nginx is available
	available, err := m.IsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check nginx availability: %w", err)
	}
	if !available {
		return fmt.Errorf("nginx is not installed on the server")
	}

	// Configure each app
	for _, config := range configs {
		// Validate config
		if config.AppName == "" {
			return fmt.Errorf("app name cannot be empty")
		}
		if config.Port == 0 {
			return fmt.Errorf("port cannot be zero for app %s", config.AppName)
		}

		// Generate nginx configuration
		var nginxConfig string
		if config.SSLEnabled && config.Domain != "" {
			nginxConfig = m.generateSSLConfig(config)
		} else {
			nginxConfig = m.generateHTTPConfig(config)
		}

		// Write configuration to file
		configPath := m.GetConfigPath(config.AppName)
		tmpFile := fmt.Sprintf("/tmp/nginx-%s.conf", config.AppName)
		escapedConfig := strings.ReplaceAll(nginxConfig, "'", "'\"'\"'")

		// Write to temp file
		writeTmpCmd := fmt.Sprintf("echo '%s' > %s", escapedConfig, tmpFile)
		result := m.executor.Execute(writeTmpCmd)
		if result.Error != nil {
			return fmt.Errorf("failed to write temp config for %s: %w", config.AppName, result.Error)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("failed to write temp config for %s (exit code %d): %s", config.AppName, result.ExitCode, result.Stderr)
		}

		// Move temp file to final location with sudo
		moveCmd := fmt.Sprintf("mv %s %s", tmpFile, configPath)
		result = m.executor.ExecuteSudo(moveCmd)
		if result.Error != nil {
			return fmt.Errorf("failed to write nginx config for %s: %w", config.AppName, result.Error)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("failed to write nginx config for %s (exit code %d): %s", config.AppName, result.ExitCode, result.Stderr)
		}

		// Create symlink to enable the site
		symlinkCmd := fmt.Sprintf(
			"ln -sf %s /etc/nginx/sites-enabled/%s.conf",
			configPath,
			config.AppName,
		)
		result = m.executor.ExecuteSudo(symlinkCmd)
		if result.Error != nil {
			return fmt.Errorf("failed to enable nginx site for %s: %w", config.AppName, result.Error)
		}
	}

	// Test nginx configuration once for all apps
	result := m.executor.ExecuteSudo("nginx -t")
	if result.Error != nil {
		return fmt.Errorf("nginx config test failed: %w", result.Error)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("nginx config test failed: %s", result.Stderr)
	}

	// Reload nginx once for all apps
	return m.Reload()
}

// Remove removes the nginx configuration for an application
func (m *Manager) Remove(appName string) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	// Remove symlink from sites-enabled
	result := m.executor.ExecuteSudo(fmt.Sprintf("rm -f /etc/nginx/sites-enabled/%s.conf", appName))
	if result.Error != nil {
		return fmt.Errorf("failed to remove nginx site link: %w", result.Error)
	}

	// Remove configuration file
	configPath := m.GetConfigPath(appName)
	result = m.executor.ExecuteSudo(fmt.Sprintf("rm -f %s", configPath))
	if result.Error != nil {
		return fmt.Errorf("failed to remove nginx config: %w", result.Error)
	}

	// Reload nginx
	return m.Reload()
}

// GetConfigPath returns the path to the nginx configuration file
func (m *Manager) GetConfigPath(appName string) string {
	return fmt.Sprintf("/etc/nginx/sites-available/%s.conf", appName)
}

// generateHTTPConfig generates HTTP-only nginx configuration
func (m *Manager) generateHTTPConfig(config proxy.ProxyConfig) string {
	serverName := "_"
	if config.Domain != "" {
		serverName = config.Domain
	}

	return fmt.Sprintf(`server {
  listen 80;
  server_name %s;

  access_log /var/log/nginx/%s_access.log;
  error_log  /var/log/nginx/%s_error.log;

  location /static/ { alias /srv/%s/shared/static/; }
  location /media/  { alias /srv/%s/shared/media/; }

  location / {
    proxy_pass http://127.0.0.1:%d;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
`,
		serverName,
		config.AppName,
		config.AppName,
		config.AppName,
		config.AppName,
		config.Port,
	)
}

// generateSSLConfig generates HTTPS-enabled nginx configuration with HTTP redirect
func (m *Manager) generateSSLConfig(config proxy.ProxyConfig) string {
	return fmt.Sprintf(`# HTTP server - redirect to HTTPS
server {
  listen 80;
  server_name %s;
  return 301 https://$server_name$request_uri;
}

# HTTPS server
server {
  listen 443 ssl http2;
  server_name %s;

  # SSL configuration
  ssl_certificate %s;
  ssl_certificate_key %s;
  ssl_protocols TLSv1.2 TLSv1.3;
  ssl_ciphers HIGH:!aNULL:!MD5;
  ssl_prefer_server_ciphers on;

  # Security headers
  add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
  add_header X-Frame-Options "SAMEORIGIN" always;
  add_header X-Content-Type-Options "nosniff" always;

  # Logging
  access_log /var/log/nginx/%s_access.log;
  error_log  /var/log/nginx/%s_error.log;

  # Static files
  location /static/ { alias /srv/%s/shared/static/; }
  location /media/  { alias /srv/%s/shared/media/; }

  # Proxy to application
  location / {
    proxy_pass http://127.0.0.1:%d;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Host $server_name;
  }
}
`,
		config.Domain,
		config.Domain,
		config.SSLCertPath,
		config.SSLKeyPath,
		config.AppName,
		config.AppName,
		config.AppName,
		config.AppName,
		config.Port,
	)
}
