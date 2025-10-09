package caddy

import (
	"fmt"
	"lightfold/pkg/ssh"
	"lightfold/pkg/ssl"
)

func init() {
	ssl.Register("caddy", func() ssl.SSLManager {
		return &Manager{}
	})
}

// Manager implements SSL management for Caddy (which handles SSL automatically)
// This is essentially a no-op wrapper since Caddy handles ACME internally
type Manager struct {
	executor *ssh.Executor
}

// NewManager creates a new Caddy SSL manager
func NewManager(executor *ssh.Executor) *Manager {
	return &Manager{
		executor: executor,
	}
}

// SetExecutor sets the SSH executor for remote command execution
func (m *Manager) SetExecutor(executor *ssh.Executor) {
	m.executor = executor
}

// Name returns the name of this SSL manager
func (m *Manager) Name() string {
	return "caddy"
}

// IsAvailable checks if Caddy is available (which handles SSL automatically)
func (m *Manager) IsAvailable() (bool, error) {
	if m.executor == nil {
		return false, fmt.Errorf("SSH executor not configured")
	}

	result := m.executor.Execute("which caddy")
	if result.Error != nil {
		return false, result.Error
	}

	return result.ExitCode == 0, nil
}

// IssueCertificate is a no-op for Caddy since it handles SSL automatically
// Caddy will issue certificates automatically when domains are configured
func (m *Manager) IssueCertificate(domain string, email string) error {
	// Caddy handles this automatically through its ACME integration
	// No action needed - certificates are issued on first request
	return nil
}

// RenewCertificate is a no-op for Caddy since it handles renewal automatically
func (m *Manager) RenewCertificate(domain string) error {
	// Caddy handles renewal automatically
	// No action needed
	return nil
}

// EnableAutoRenewal is a no-op for Caddy since auto-renewal is built-in
func (m *Manager) EnableAutoRenewal() error {
	// Caddy has auto-renewal enabled by default
	// No action needed
	return nil
}

// GetCertificatePath returns the path to Caddy's certificate storage
// Caddy stores certificates in /var/lib/caddy/.local/share/caddy/certificates/
func (m *Manager) GetCertificatePath(domain string) (certPath string, keyPath string, err error) {
	// Caddy's certificate storage structure:
	// /var/lib/caddy/.local/share/caddy/certificates/acme-v02.api.letsencrypt.org-directory/<domain>/<domain>.crt
	// /var/lib/caddy/.local/share/caddy/certificates/acme-v02.api.letsencrypt.org-directory/<domain>/<domain>.key

	baseDir := "/var/lib/caddy/.local/share/caddy/certificates/acme-v02.api.letsencrypt.org-directory"
	certPath = fmt.Sprintf("%s/%s/%s.crt", baseDir, domain, domain)
	keyPath = fmt.Sprintf("%s/%s/%s.key", baseDir, domain, domain)

	return certPath, keyPath, nil
}
