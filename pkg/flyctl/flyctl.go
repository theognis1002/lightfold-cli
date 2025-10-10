package flyctl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Client wraps flyctl CLI operations
type Client struct {
	token   string
	appName string
}

// NewClient creates a new flyctl client
func NewClient(token, appName string) *Client {
	return &Client{
		token:   token,
		appName: appName,
	}
}

// IsInstalled checks if flyctl is available in PATH
func IsInstalled() bool {
	_, err := exec.LookPath("flyctl")
	return err == nil
}

// InstallInstructions returns platform-specific installation instructions
func InstallInstructions() string {
	switch runtime.GOOS {
	case "darwin":
		return "Install flyctl:\n  brew install flyctl"
	case "linux":
		return "Install flyctl:\n  curl -L https://fly.io/install.sh | sh"
	case "windows":
		return "Install flyctl:\n  iwr https://fly.io/install.ps1 -useb | iex"
	default:
		return "Install flyctl: https://fly.io/docs/flyctl/install/"
	}
}

// EnsureFlyctl checks if flyctl is installed and provides instructions if not
func EnsureFlyctl() error {
	if IsInstalled() {
		return nil
	}
	return fmt.Errorf("flyctl not found\n\n%s", InstallInstructions())
}

// AuthenticateWithToken is deprecated - flyctl uses FLY_ACCESS_TOKEN environment variable
// We don't need to explicitly authenticate, just pass the token via env var
// This function is kept for compatibility but does nothing
func AuthenticateWithToken(token string) error {
	// No-op: flyctl automatically uses FLY_ACCESS_TOKEN from environment
	// which we set in all Deploy/SetSecrets/GetLogs commands
	return nil
}

// DeployOptions contains options for flyctl deploy
type DeployOptions struct {
	ProjectPath string            // Local project directory
	Secrets     map[string]string // Environment variables/secrets
	Region      string            // fly.io region
	RemoteOnly  bool              // Use remote builder (default: true)
	UseNixpacks bool              // Use fly.io's native nixpacks builder (recommended)
}

// Deploy runs flyctl deploy with remote builder
func (c *Client) Deploy(ctx context.Context, opts DeployOptions) (string, error) {
	if err := EnsureFlyctl(); err != nil {
		return "", err
	}

	// Build command
	args := []string{"deploy"}

	// Use fly.io's native nixpacks builder (recommended)
	if opts.UseNixpacks {
		args = append(args, "--nixpacks")
	}

	if opts.RemoteOnly {
		args = append(args, "--remote-only")
	}

	if opts.Region != "" {
		args = append(args, "--primary-region", opts.Region)
	}

	args = append(args, "--app", c.appName)

	// Run deploy
	cmd := exec.CommandContext(ctx, "flyctl", args...)
	cmd.Dir = opts.ProjectPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("FLY_ACCESS_TOKEN=%s", c.token))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if err != nil {
		return output, fmt.Errorf("flyctl deploy failed: %w\nOutput: %s\nError: %s", err, output, stderr.String())
	}

	return output, nil
}

// SetSecrets sets secrets for the app
func (c *Client) SetSecrets(ctx context.Context, secrets map[string]string) error {
	if err := EnsureFlyctl(); err != nil {
		return err
	}

	if len(secrets) == 0 {
		return nil
	}

	// Build secret pairs (KEY=VALUE format)
	var secretPairs []string
	for key, value := range secrets {
		secretPairs = append(secretPairs, fmt.Sprintf("%s=%s", key, value))
	}

	args := []string{"secrets", "set", "--app", c.appName}
	args = append(args, secretPairs...)

	cmd := exec.CommandContext(ctx, "flyctl", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("FLY_ACCESS_TOKEN=%s", c.token))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("flyctl secrets set failed: %w\nError: %s", err, stderr.String())
	}

	return nil
}

// GetLogs retrieves application logs
func (c *Client) GetLogs(ctx context.Context, lines int, follow bool) (string, error) {
	if err := EnsureFlyctl(); err != nil {
		return "", err
	}

	args := []string{"logs", "--app", c.appName}

	if lines > 0 {
		args = append(args, "--lines", fmt.Sprintf("%d", lines))
	}

	if follow {
		args = append(args, "--follow")
	}

	cmd := exec.CommandContext(ctx, "flyctl", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("FLY_ACCESS_TOKEN=%s", c.token))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if err != nil {
		return output, fmt.Errorf("flyctl logs failed: %w\nError: %s", err, stderr.String())
	}

	return output, nil
}

// GetStatus retrieves app status
func (c *Client) GetStatus(ctx context.Context) (string, error) {
	if err := EnsureFlyctl(); err != nil {
		return "", err
	}

	args := []string{"status", "--app", c.appName}

	cmd := exec.CommandContext(ctx, "flyctl", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("FLY_ACCESS_TOKEN=%s", c.token))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if err != nil {
		return output, fmt.Errorf("flyctl status failed: %w\nError: %s", err, stderr.String())
	}

	return output, nil
}

// GetAppInfo retrieves detailed app information
func (c *Client) GetAppInfo(ctx context.Context) (string, error) {
	if err := EnsureFlyctl(); err != nil {
		return "", err
	}

	args := []string{"apps", "info", c.appName, "--json"}

	cmd := exec.CommandContext(ctx, "flyctl", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("FLY_ACCESS_TOKEN=%s", c.token))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if err != nil {
		return output, fmt.Errorf("flyctl apps info failed: %w\nError: %s", err, stderr.String())
	}

	return output, nil
}

// Version returns the installed flyctl version
func Version() (string, error) {
	if !IsInstalled() {
		return "", fmt.Errorf("flyctl not installed")
	}

	cmd := exec.Command("flyctl", "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get flyctl version: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
