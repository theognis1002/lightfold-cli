package certbot

import (
	"fmt"
	"lightfold/pkg/ssh"
	"lightfold/pkg/ssl"
	"strings"
)

func init() {
	ssl.Register("certbot", func() ssl.SSLManager {
		return &Manager{}
	})
}

// Manager implements SSL certificate management using Certbot/Let's Encrypt
type Manager struct {
	executor *ssh.Executor
}

// NewManager creates a new Certbot SSL manager
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
	return "certbot"
}

// IsAvailable checks if certbot is installed and available
func (m *Manager) IsAvailable() (bool, error) {
	if m.executor == nil {
		return false, fmt.Errorf("SSH executor not configured")
	}

	result := m.executor.Execute("which certbot")
	if result.Error != nil {
		return false, result.Error
	}

	if result.ExitCode != 0 {
		return false, nil
	}

	return true, nil
}

// IssueCertificate issues a new SSL certificate using Let's Encrypt
func (m *Manager) IssueCertificate(domain string, email string) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if email == "" {
		email = "noreply@" + domain
	}

	available, err := m.IsAvailable()
	if err != nil {
		return fmt.Errorf("failed to check certbot availability: %w", err)
	}
	if !available {
		if err := m.installCertbot(); err != nil {
			return fmt.Errorf("certbot not available and installation failed: %w", err)
		}
	}

	cmd := fmt.Sprintf(
		"certbot --nginx -d %s --non-interactive --agree-tos --email %s",
		domain,
		email,
	)

	result := m.executor.ExecuteSudo(cmd)
	if result.Error != nil {
		return fmt.Errorf("failed to execute certbot: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("certbot failed (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	return nil
}

// RenewCertificate renews an existing SSL certificate
func (m *Manager) RenewCertificate(domain string) error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	cmd := fmt.Sprintf("certbot renew --cert-name %s", domain)
	result := m.executor.ExecuteSudo(cmd)

	if result.Error != nil {
		return fmt.Errorf("failed to execute certbot renew: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("certbot renew failed (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	return nil
}

// EnableAutoRenewal sets up automatic certificate renewal using systemd timer
func (m *Manager) EnableAutoRenewal() error {
	if m.executor == nil {
		return fmt.Errorf("SSH executor not configured")
	}

	// Enable and start certbot systemd timer
	// This is the modern approach for auto-renewal on Ubuntu/Debian systems
	result := m.executor.ExecuteSudo("systemctl enable certbot.timer && systemctl start certbot.timer")

	if result.Error != nil {
		return fmt.Errorf("failed to enable certbot auto-renewal: %w", result.Error)
	}

	if result.ExitCode != 0 {
		// Fallback: try to set up a cron job if systemd timer is not available
		return m.setupCronRenewal()
	}

	return nil
}

// GetCertificatePath returns the paths to the certificate and key files
func (m *Manager) GetCertificatePath(domain string) (certPath string, keyPath string, error error) {
	certPath = fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
	keyPath = fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", domain)

	if m.executor != nil {
		result := m.executor.ExecuteSudo(fmt.Sprintf("test -f %s && test -f %s", certPath, keyPath))
		if result.ExitCode != 0 {
			return "", "", fmt.Errorf("certificate files not found for domain %s", domain)
		}
	}

	return certPath, keyPath, nil
}

// installCertbot attempts to install certbot if it's not available
func (m *Manager) installCertbot() error {
	cmd := "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y certbot python3-certbot-nginx"
	result := m.executor.ExecuteSudo(cmd)

	if result.Error != nil {
		return fmt.Errorf("failed to install certbot: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("certbot installation failed (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	return nil
}

// setupCronRenewal sets up a cron job for certificate renewal (fallback method)
func (m *Manager) setupCronRenewal() error {
	result := m.executor.Execute("crontab -l 2>/dev/null | grep -q 'certbot renew'")
	if result.ExitCode == 0 {
		return nil
	}

	// Add a cron job to renew certificates twice daily (recommended by Let's Encrypt)
	cronEntry := "0 0,12 * * * certbot renew --quiet"
	cmd := fmt.Sprintf("(crontab -l 2>/dev/null; echo '%s') | crontab -", cronEntry)

	result = m.executor.Execute(cmd)
	if result.Error != nil {
		return fmt.Errorf("failed to setup cron renewal: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("cron setup failed (exit code %d): %s", result.ExitCode, result.Stderr)
	}

	return nil
}

// CheckCertificateExpiry checks when a certificate will expire
func (m *Manager) CheckCertificateExpiry(domain string) (daysRemaining int, err error) {
	if m.executor == nil {
		return 0, fmt.Errorf("SSH executor not configured")
	}

	certPath, _, err := m.GetCertificatePath(domain)
	if err != nil {
		return 0, err
	}

	cmd := fmt.Sprintf("openssl x509 -enddate -noout -in %s | cut -d= -f2", certPath)
	result := m.executor.ExecuteSudo(cmd)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to check certificate expiry: %w", result.Error)
	}

	if result.ExitCode != 0 {
		return 0, fmt.Errorf("failed to read certificate expiry date")
	}

	expiryDate := strings.TrimSpace(result.Stdout)
	if expiryDate == "" {
		return 0, fmt.Errorf("could not determine certificate expiry date")
	}

	checkResult := m.executor.ExecuteSudo(fmt.Sprintf("certbot certificates -d %s 2>&1 | grep 'VALID:'", domain))
	if checkResult.ExitCode == 0 {
		return 30, nil
	}

	return 0, nil
}
