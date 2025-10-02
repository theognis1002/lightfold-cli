package deploy

import (
	"archive/tar"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"lightfold/pkg/detector"
	"lightfold/pkg/ssh"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/systemd.service.tmpl
var systemdTemplate string

//go:embed templates/nginx.conf.tmpl
var nginxTemplate string

// Executor handles the actual deployment operations on a server
type Executor struct {
	ssh         *ssh.Executor
	appName     string
	projectPath string
	detection   *detector.Detection
}

// NewExecutor creates a new deployment executor
func NewExecutor(sshExecutor *ssh.Executor, appName, projectPath string, detection *detector.Detection) *Executor {
	return &Executor{
		ssh:         sshExecutor,
		appName:     appName,
		projectPath: projectPath,
		detection:   detection,
	}
}

// InstallBasePackages installs required system packages
func (e *Executor) InstallBasePackages() error {
	result := e.ssh.ExecuteSudo("apt-get update -y")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to update packages: %s", result.Stderr)
	}

	result = e.ssh.ExecuteSudo("apt-get install -y nginx")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to install nginx: %s", result.Stderr)
	}

	if e.detection != nil {
		switch e.detection.Language {
		case "JavaScript/TypeScript":
			result = e.ssh.ExecuteSudo("curl -fsSL https://deb.nodesource.com/setup_lts.x | bash -")
			if result.Error != nil || result.ExitCode != 0 {
				return fmt.Errorf("failed to setup NodeSource repository: %s", result.Stderr)
			}
			result = e.ssh.ExecuteSudo("apt-get install -y nodejs")
			if result.Error != nil || result.ExitCode != 0 {
				return fmt.Errorf("failed to install Node.js: %s", result.Stderr)
			}

		case "Python":
			result = e.ssh.ExecuteSudo("apt-get install -y python3 python3-pip python3-venv")
			if result.Error != nil || result.ExitCode != 0 {
				return fmt.Errorf("failed to install Python: %s", result.Stderr)
			}

		case "Go":
			result = e.ssh.ExecuteSudo("apt-get install -y golang-go")
			if result.Error != nil || result.ExitCode != 0 {
				return fmt.Errorf("failed to install Go: %s", result.Stderr)
			}

		case "PHP":
			result = e.ssh.ExecuteSudo("apt-get install -y php php-fpm php-mysql php-xml php-mbstring")
			if result.Error != nil || result.ExitCode != 0 {
				return fmt.Errorf("failed to install PHP: %s", result.Stderr)
			}

		case "Ruby":
			result = e.ssh.ExecuteSudo("apt-get install -y ruby-full")
			if result.Error != nil || result.ExitCode != 0 {
				return fmt.Errorf("failed to install Ruby: %s", result.Stderr)
			}
		}
	}

	return nil
}

// SetupDirectoryStructure creates the deployment directory structure
func (e *Executor) SetupDirectoryStructure() error {
	appPath := fmt.Sprintf("/srv/%s", e.appName)

	directories := []string{
		appPath,
		filepath.Join(appPath, "releases"),
		filepath.Join(appPath, "shared"),
		filepath.Join(appPath, "shared", "env"),
		filepath.Join(appPath, "shared", "logs"),
		filepath.Join(appPath, "shared", "static"),
		filepath.Join(appPath, "shared", "media"),
	}

	for _, dir := range directories {
		result := e.ssh.ExecuteSudo(fmt.Sprintf("mkdir -p %s", dir))
		if result.Error != nil || result.ExitCode != 0 {
			return fmt.Errorf("failed to create directory %s: %s", dir, result.Stderr)
		}
	}

	result := e.ssh.ExecuteSudo(fmt.Sprintf("chown -R www-data:www-data %s", appPath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to set ownership: %s", result.Stderr)
	}

	return nil
}

func (e *Executor) CreateReleaseTarball(outputPath string) error {
	tarFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create tarball: %w", err)
	}
	defer tarFile.Close()

	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	ignorePatterns := []string{
		".git",
		"node_modules",
		"__pycache__",
		".venv",
		"venv",
		".env",
		".env.local",
		"*.pyc",
		"*.pyo",
		".DS_Store",
		"dist",
		".next",
		"build",
		"target",
		".idea",
		".vscode",
	}

	err = filepath.WalkDir(e.projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(e.projectPath, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		for _, pattern := range ignorePatterns {
			if matched, _ := filepath.Match(pattern, d.Name()); matched {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.Contains(relPath, "/"+pattern+"/") || strings.HasPrefix(relPath, pattern+"/") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if !d.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (e *Executor) UploadRelease(tarballPath string) (string, error) {
	timestamp := time.Now().Format("20060102150405")
	releasePath := fmt.Sprintf("/srv/%s/releases/%s", e.appName, timestamp)

	result := e.ssh.ExecuteSudo(fmt.Sprintf("mkdir -p %s", releasePath))
	if result.Error != nil || result.ExitCode != 0 {
		return "", fmt.Errorf("failed to create release directory: %s", result.Stderr)
	}

	remoteTarball := fmt.Sprintf("/tmp/%s-release.tar.gz", e.appName)
	if err := e.ssh.UploadFile(tarballPath, remoteTarball); err != nil {
		return "", fmt.Errorf("failed to upload tarball: %w", err)
	}

	result = e.ssh.ExecuteSudo(fmt.Sprintf("tar -xzf %s -C %s", remoteTarball, releasePath))
	if result.Error != nil || result.ExitCode != 0 {
		return "", fmt.Errorf("failed to extract tarball: %s", result.Stderr)
	}

	e.ssh.Execute(fmt.Sprintf("rm %s", remoteTarball))
	result = e.ssh.ExecuteSudo(fmt.Sprintf("chown -R www-data:www-data %s", releasePath))
	if result.Error != nil || result.ExitCode != 0 {
		return "", fmt.Errorf("failed to set ownership: %s", result.Stderr)
	}

	return releasePath, nil
}

func (e *Executor) BuildRelease(releasePath string) error {
	if e.detection == nil || len(e.detection.BuildPlan) == 0 {
		return nil
	}
	if e.detection.Language == "Python" {
		venvPath := fmt.Sprintf("/srv/%s/shared/venv", e.appName)
		result := e.ssh.ExecuteSudo(fmt.Sprintf("python3 -m venv %s", venvPath))
		if result.Error != nil || result.ExitCode != 0 {
			return fmt.Errorf("failed to create venv: %s", result.Stderr)
		}
	}

	for _, cmd := range e.detection.BuildPlan {
		buildCmd := e.adjustBuildCommand(cmd, releasePath)

		result := e.ssh.ExecuteSudo(fmt.Sprintf("cd %s && %s", releasePath, buildCmd))
		if result.Error != nil || result.ExitCode != 0 {
			return fmt.Errorf("build command failed '%s': %s", cmd, result.Stderr)
		}
	}

	return nil
}

func (e *Executor) adjustBuildCommand(cmd, releasePath string) string {
	if e.detection == nil {
		return cmd
	}

	switch e.detection.Language {
	case "Python":
		if strings.Contains(cmd, "pip install") {
			venvPath := fmt.Sprintf("/srv/%s/shared/venv", e.appName)
			return strings.Replace(cmd, "pip install", fmt.Sprintf("%s/bin/pip install", venvPath), 1)
		}
		if strings.Contains(cmd, "poetry install") {
			return "pip3 install poetry && poetry install"
		}
		if strings.Contains(cmd, "uv") {
			return "pip3 install uv && " + cmd
		}

	case "JavaScript/TypeScript":
		if strings.Contains(cmd, "pnpm install") {
			return "npm install -g pnpm && " + cmd
		}
		if strings.Contains(cmd, "bun install") {
			return "npm install -g bun && " + cmd
		}
		if strings.Contains(cmd, "npm install") || strings.Contains(cmd, "yarn install") {
			return cmd
		}

	case "Go":
		return cmd

	case "Ruby":
		if strings.Contains(cmd, "bundle install") {
			return "gem install bundler && " + cmd
		}
	}

	return cmd
}

func (e *Executor) WriteEnvironmentFile(envVars map[string]string) error {
	if len(envVars) == 0 {
		return nil
	}
	var envContent strings.Builder
	for key, value := range envVars {
		envContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}

	envPath := fmt.Sprintf("/srv/%s/shared/env/.env", e.appName)
	if err := e.ssh.WriteRemoteFile(envPath, envContent.String(), 0600); err != nil {
		return fmt.Errorf("failed to write environment file: %w", err)
	}

	result := e.ssh.ExecuteSudo(fmt.Sprintf("chown www-data:www-data %s", envPath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to set env file ownership: %s", result.Stderr)
	}

	return nil
}

func (e *Executor) GetCurrentRelease() (string, error) {
	currentLink := fmt.Sprintf("/srv/%s/current", e.appName)
	result := e.ssh.Execute(fmt.Sprintf("readlink -f %s", currentLink))
	if result.Error != nil || result.ExitCode != 0 {
		return "", nil
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (e *Executor) ListReleases() ([]string, error) {
	releasesPath := fmt.Sprintf("/srv/%s/releases", e.appName)
	result := e.ssh.Execute(fmt.Sprintf("ls -1 %s", releasesPath))
	if result.Error != nil || result.ExitCode != 0 {
		return []string{}, nil
	}

	releases := []string{}
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line != "" {
			releases = append(releases, line)
		}
	}
	return releases, nil
}

func (e *Executor) CleanupOldReleases(keepCount int) error {
	releases, err := e.ListReleases()
	if err != nil {
		return err
	}

	if len(releases) <= keepCount {
		return nil
	}

	toDelete := releases[:len(releases)-keepCount]

	for _, release := range toDelete {
		releasePath := fmt.Sprintf("/srv/%s/releases/%s", e.appName, release)
		result := e.ssh.ExecuteSudo(fmt.Sprintf("rm -rf %s", releasePath))
		if result.Error != nil || result.ExitCode != 0 {
			return fmt.Errorf("failed to delete release %s: %s", release, result.Stderr)
		}
	}

	return nil
}

// --- Phase 3: Service Configuration ---

// GenerateSystemdUnit creates a systemd service unit file for the application
func (e *Executor) GenerateSystemdUnit(releasePath string) error {
	execStart := e.getExecStartCommand()

	data := map[string]string{
		"APP_NAME":   e.appName,
		"EXEC_START": execStart,
	}

	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", e.appName)
	if err := e.ssh.RenderAndWriteTemplate(systemdTemplate, data, unitPath, 0644); err != nil {
		return fmt.Errorf("failed to write systemd unit: %w", err)
	}

	// Set proper ownership
	result := e.ssh.ExecuteSudo(fmt.Sprintf("chown root:root %s", unitPath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to set systemd unit ownership: %s", result.Stderr)
	}

	// Reload systemd
	result = e.ssh.ExecuteSudo("systemctl daemon-reload")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to reload systemd: %s", result.Stderr)
	}

	return nil
}

// getExecStartCommand returns the framework-specific ExecStart command
func (e *Executor) getExecStartCommand() string {
	if e.detection == nil {
		return "/usr/bin/true"
	}

	framework := e.detection.Framework
	language := e.detection.Language
	appPath := fmt.Sprintf("/srv/%s", e.appName)

	switch language {
	case "Python":
		venvBin := fmt.Sprintf("%s/shared/venv/bin", appPath)
		switch framework {
		case "Django":
			return fmt.Sprintf("%s/gunicorn --bind 127.0.0.1:8000 --workers 2 wsgi:application", venvBin)
		case "FastAPI":
			return fmt.Sprintf("%s/uvicorn main:app --host 127.0.0.1 --port 8000 --workers 2", venvBin)
		case "Flask":
			return fmt.Sprintf("%s/gunicorn --bind 127.0.0.1:8000 --workers 2 app:app", venvBin)
		}

	case "JavaScript/TypeScript":
		switch framework {
		case "Next.js":
			return fmt.Sprintf("PORT=8000 /usr/bin/node %s/current/.next/standalone/server.js", appPath)
		case "Express.js":
			return fmt.Sprintf("PORT=8000 /usr/bin/node %s/current/server.js", appPath)
		case "NestJS":
			return fmt.Sprintf("PORT=8000 /usr/bin/node %s/current/dist/main.js", appPath)
		}

	case "Go":
		return fmt.Sprintf("%s/current/app --port 8000", appPath)

	case "Ruby":
		if framework == "Rails" {
			return fmt.Sprintf("%s/shared/bundle/bin/puma -C %s/current/config/puma.rb", appPath, appPath)
		}
	}

	// Fallback - use the first run command if available
	if len(e.detection.RunPlan) > 0 {
		return e.detection.RunPlan[0]
	}

	return "/usr/bin/true"
}

// GenerateNginxConfig creates an nginx reverse proxy configuration
func (e *Executor) GenerateNginxConfig() error {
	data := map[string]string{
		"APP_NAME": e.appName,
	}

	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", e.appName)
	if err := e.ssh.RenderAndWriteTemplate(nginxTemplate, data, configPath, 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// Create symlink to sites-enabled
	symlinkCmd := fmt.Sprintf("ln -sf /etc/nginx/sites-available/%s /etc/nginx/sites-enabled/%s", e.appName, e.appName)
	result := e.ssh.ExecuteSudo(symlinkCmd)
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to enable nginx site: %s", result.Stderr)
	}

	// Remove default nginx site if exists
	e.ssh.ExecuteSudo("rm -f /etc/nginx/sites-enabled/default")

	return nil
}

// TestNginxConfig validates the nginx configuration
func (e *Executor) TestNginxConfig() error {
	result := e.ssh.ExecuteSudo("nginx -t")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("nginx config test failed: %s", result.Stderr)
	}
	return nil
}

// ReloadNginx reloads nginx configuration
func (e *Executor) ReloadNginx() error {
	result := e.ssh.ExecuteSudo("systemctl reload nginx")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to reload nginx: %s", result.Stderr)
	}
	return nil
}

// EnableService enables the systemd service to start on boot
func (e *Executor) EnableService() error {
	result := e.ssh.ExecuteSudo(fmt.Sprintf("systemctl enable %s", e.appName))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to enable service: %s", result.Stderr)
	}
	return nil
}

// StartService starts the systemd service
func (e *Executor) StartService() error {
	result := e.ssh.ExecuteSudo(fmt.Sprintf("systemctl start %s", e.appName))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to start service: %s", result.Stderr)
	}
	return nil
}

// RestartService restarts the systemd service
func (e *Executor) RestartService() error {
	result := e.ssh.ExecuteSudo(fmt.Sprintf("systemctl restart %s", e.appName))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to restart service: %s", result.Stderr)
	}
	return nil
}

// StopService stops the systemd service
func (e *Executor) StopService() error {
	result := e.ssh.ExecuteSudo(fmt.Sprintf("systemctl stop %s", e.appName))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to stop service: %s", result.Stderr)
	}
	return nil
}

// GetServiceStatus gets the status of the systemd service
func (e *Executor) GetServiceStatus() (bool, error) {
	result := e.ssh.Execute(fmt.Sprintf("systemctl is-active %s", e.appName))
	isActive := result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == "active"
	return isActive, nil
}

// --- Phase 4: Release Management ---

// SwitchRelease atomically switches the current symlink to a new release
func (e *Executor) SwitchRelease(releasePath string) error {
	currentLink := fmt.Sprintf("/srv/%s/current", e.appName)
	tempLink := fmt.Sprintf("/srv/%s/current.tmp", e.appName)

	// Create temporary symlink
	result := e.ssh.ExecuteSudo(fmt.Sprintf("ln -sf %s %s", releasePath, tempLink))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to create temp symlink: %s", result.Stderr)
	}

	// Atomically replace current symlink
	result = e.ssh.ExecuteSudo(fmt.Sprintf("mv -Tf %s %s", tempLink, currentLink))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to switch release: %s", result.Stderr)
	}

	return nil
}

// PerformHealthCheck checks if the application is responding correctly
func (e *Executor) PerformHealthCheck(maxRetries int, retryDelay time.Duration) error {
	if e.detection == nil || e.detection.Healthcheck == nil {
		// No healthcheck configured, assume success
		return nil
	}

	healthPath := "/"
	expectedStatus := 200
	timeout := 30

	if path, ok := e.detection.Healthcheck["path"].(string); ok {
		healthPath = path
	}
	if expect, ok := e.detection.Healthcheck["expect"].(int); ok {
		expectedStatus = expect
	}
	if expectFloat, ok := e.detection.Healthcheck["expect"].(float64); ok {
		expectedStatus = int(expectFloat)
	}
	if timeoutSec, ok := e.detection.Healthcheck["timeout_seconds"].(int); ok {
		timeout = timeoutSec
	}
	if timeoutFloat, ok := e.detection.Healthcheck["timeout_seconds"].(float64); ok {
		timeout = int(timeoutFloat)
	}

	url := fmt.Sprintf("http://127.0.0.1:8000%s", healthPath)
	curlCmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' --max-time %d %s", timeout, url)

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		result := e.ssh.Execute(curlCmd)
		if result.Error != nil {
			lastErr = fmt.Errorf("health check failed (attempt %d/%d): %w", attempt+1, maxRetries, result.Error)
			continue
		}

		statusCode := strings.TrimSpace(result.Stdout)
		if statusCode == fmt.Sprintf("%d", expectedStatus) {
			return nil // Health check passed
		}

		lastErr = fmt.Errorf("health check failed (attempt %d/%d): expected status %d, got %s", attempt+1, maxRetries, expectedStatus, statusCode)
	}

	return lastErr
}

// RollbackToPreviousRelease rolls back to the previous release
func (e *Executor) RollbackToPreviousRelease() error {
	releases, err := e.ListReleases()
	if err != nil {
		return fmt.Errorf("failed to list releases: %w", err)
	}

	if len(releases) < 2 {
		return fmt.Errorf("no previous release available for rollback")
	}

	// Current release is the last one, previous is second-to-last
	previousRelease := releases[len(releases)-2]
	previousPath := fmt.Sprintf("/srv/%s/releases/%s", e.appName, previousRelease)

	// Stop the service
	if err := e.StopService(); err != nil {
		return fmt.Errorf("failed to stop service during rollback: %w", err)
	}

	// Switch to previous release
	if err := e.SwitchRelease(previousPath); err != nil {
		return fmt.Errorf("failed to switch to previous release: %w", err)
	}

	// Restart the service
	if err := e.StartService(); err != nil {
		return fmt.Errorf("failed to start service after rollback: %w", err)
	}

	return nil
}

// RollbackToRelease rolls back to a specific release by timestamp
func (e *Executor) RollbackToRelease(timestamp string) error {
	releasePath := fmt.Sprintf("/srv/%s/releases/%s", e.appName, timestamp)

	// Verify release exists
	result := e.ssh.Execute(fmt.Sprintf("test -d %s", releasePath))
	if result.ExitCode != 0 {
		return fmt.Errorf("release %s does not exist", timestamp)
	}

	// Stop the service
	if err := e.StopService(); err != nil {
		return fmt.Errorf("failed to stop service during rollback: %w", err)
	}

	// Switch to specified release
	if err := e.SwitchRelease(releasePath); err != nil {
		return fmt.Errorf("failed to switch to release %s: %w", timestamp, err)
	}

	// Restart the service
	if err := e.StartService(); err != nil {
		return fmt.Errorf("failed to start service after rollback: %w", err)
	}

	return nil
}

// DeployWithHealthCheck performs a complete deployment with health check and rollback
func (e *Executor) DeployWithHealthCheck(releasePath string, healthCheckRetries int, healthCheckDelay time.Duration) error {
	// Get current release for potential rollback
	currentRelease, err := e.GetCurrentRelease()
	if err != nil {
		return fmt.Errorf("failed to get current release: %w", err)
	}

	// Switch to new release
	if err := e.SwitchRelease(releasePath); err != nil {
		return fmt.Errorf("failed to switch release: %w", err)
	}

	// Restart service to pick up new release
	if err := e.RestartService(); err != nil {
		// Try to rollback
		if currentRelease != "" {
			e.SwitchRelease(currentRelease)
			e.RestartService()
		}
		return fmt.Errorf("failed to restart service: %w", err)
	}

	// Perform health check
	if err := e.PerformHealthCheck(healthCheckRetries, healthCheckDelay); err != nil {
		// Health check failed - rollback
		if currentRelease != "" {
			e.StopService()
			e.SwitchRelease(currentRelease)
			e.StartService()
			return fmt.Errorf("health check failed, rolled back to previous release: %w", err)
		}
		return fmt.Errorf("health check failed and no previous release to rollback to: %w", err)
	}

	return nil
}
