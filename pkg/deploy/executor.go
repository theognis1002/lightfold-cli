package deploy

import (
	"archive/tar"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	installers "lightfold/pkg/runtime/installers"
	sshpkg "lightfold/pkg/ssh"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/systemd.service.tmpl
var systemdTemplate string

//go:embed templates/nginx.conf.tmpl
var nginxTemplate string

//go:embed templates/nginx-static.conf.tmpl
var nginxStaticTemplate string

// OutputCallback is called with streaming output from commands
type OutputCallback func(line string)

// Executor handles the actual deployment operations on a server
type Executor struct {
	ssh            *sshpkg.Executor
	appName        string
	projectPath    string
	detection      *detector.Detection
	deployOptions  *config.DeploymentOptions
	outputCallback OutputCallback
	startCommand   string
}

// NewExecutor creates a new deployment executor
func NewExecutor(sshExecutor *sshpkg.Executor, appName, projectPath string, detection *detector.Detection) *Executor {
	return &Executor{
		ssh:         sshExecutor,
		appName:     appName,
		projectPath: projectPath,
		detection:   detection,
	}
}

// NewExecutorWithOptions creates a new deployment executor with custom deployment options
func NewExecutorWithOptions(sshExecutor *sshpkg.Executor, appName, projectPath string, detection *detector.Detection, deployOptions *config.DeploymentOptions) *Executor {
	return &Executor{
		ssh:           sshExecutor,
		appName:       appName,
		projectPath:   projectPath,
		detection:     detection,
		deployOptions: deployOptions,
	}
}

// SetOutputCallback sets the callback for streaming command output
func (e *Executor) SetOutputCallback(callback OutputCallback) {
	e.outputCallback = callback
}

func (e *Executor) SetStartCommand(cmd string) {
	e.startCommand = cmd
}

// sendOutput sends output to the callback if set, showing only last N lines
func (e *Executor) sendOutput(output string, lastNLines int) {
	if e.outputCallback == nil || output == "" {
		return
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Get last N lines
	start := 0
	if len(lines) > lastNLines {
		start = len(lines) - lastNLines
	}

	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			e.outputCallback("  " + strings.TrimSpace(lines[i]))
		}
	}
}

// formatSSHError creates a detailed error message from SSH command result
func formatSSHError(operation string, result *sshpkg.CommandResult) error {
	var details []string
	if result.ExitCode != 0 {
		details = append(details, fmt.Sprintf("exit_code=%d", result.ExitCode))
	}
	if result.Stdout != "" {
		details = append(details, fmt.Sprintf("stdout=%q", result.Stdout))
	}
	if result.Stderr != "" {
		details = append(details, fmt.Sprintf("stderr=%q", result.Stderr))
	}
	if result.Error != nil {
		details = append(details, fmt.Sprintf("error=%v", result.Error))
	}

	if len(details) > 0 {
		return fmt.Errorf("%s: %s", operation, strings.Join(details, ", "))
	}
	return fmt.Errorf("%s", operation)
}

// WaitForAptLock waits for apt/dpkg locks to be released (cloud-init might be running)
func (e *Executor) WaitForAptLock(maxRetries int, retryDelay time.Duration) error {
	// First, wait for cloud-init to finish completely
	if e.outputCallback != nil {
		e.outputCallback("  Waiting for cloud-init to complete...")
	}
	cloudInitCmd := fmt.Sprintf("timeout %d cloud-init status --wait 2>/dev/null || echo 'done'", int(config.DefaultCloudInitTimeout.Seconds()))
	result := e.ssh.Execute(cloudInitCmd)
	if result.Error != nil {
		// cloud-init command not available or failed, continue anyway
		if e.outputCallback != nil {
			e.outputCallback("  cloud-init status check failed, proceeding...")
		}
	}

	// Now wait for apt locks to be released
	checkCmd := "sudo lsof /var/lib/dpkg/lock-frontend 2>/dev/null || sudo lsof /var/lib/apt/lists/lock 2>/dev/null"

	for i := 0; i < maxRetries; i++ {
		result := e.ssh.Execute(checkCmd)

		// If no output, locks are free
		if result.ExitCode == 1 || result.Stdout == "" {
			return nil
		}

		// Locks are held, wait and retry
		if i < maxRetries-1 {
			if i == 0 && e.outputCallback != nil {
				e.outputCallback("  Waiting for apt locks to be released...")
			}
			time.Sleep(retryDelay)
		}
	}

	return fmt.Errorf("apt/dpkg locks still held after %d retries", maxRetries)
}

// InstallBasePackages installs required system packages
func (e *Executor) InstallBasePackages() error {
	e.ssh.ExecuteSudo("rm -f /var/lib/apt/lists/*_Commands-* 2>/dev/null || true")
	e.ssh.ExecuteSudo("killall -q apt-get dpkg 2>/dev/null || true")
	e.ssh.ExecuteSudo("rm -f /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock 2>/dev/null || true")
	e.ssh.ExecuteSudo("apt-get clean 2>/dev/null || true")

	var result *sshpkg.CommandResult
	maxRetries := config.DefaultAptMaxRetries
	for attempt := 1; attempt <= maxRetries; attempt++ {
		result = e.ssh.ExecuteSudo("apt-get update 2>&1")
		if result.Error == nil && result.ExitCode == 0 {
			break
		}

		if attempt < maxRetries {
			if e.outputCallback != nil {
				e.outputCallback(fmt.Sprintf("  apt-get update failed (attempt %d/%d), retrying...", attempt, maxRetries))
			}
			time.Sleep(time.Duration(attempt*2) * time.Second)

			e.ssh.ExecuteSudo("dpkg --configure -a 2>/dev/null || true")
			e.ssh.ExecuteSudo("apt-get clean && rm -rf /var/lib/apt/lists/* && mkdir -p /var/lib/apt/lists/partial")
		}
	}

	if result.Error != nil || result.ExitCode != 0 {
		e.sendOutput(result.Stdout+"\n"+result.Stderr, 10)
		return formatSSHError("failed to update package lists after retries", result)
	}

	result = e.ssh.ExecuteSudo("DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::=\"--force-confdef\" -o Dpkg::Options::=\"--force-confold\" nginx")
	e.sendOutput(result.Stdout, 3)
	if result.Error != nil || result.ExitCode != 0 {
		return formatSSHError("failed to install nginx", result)
	}

	if e.detection != nil {
		var tailFn func(result *sshpkg.CommandResult, lastN int)
		if e.outputCallback != nil {
			tailFn = func(result *sshpkg.CommandResult, lastN int) {
				if result != nil {
					e.sendOutput(result.Stdout, lastN)
				}
			}
		}

		ctx := &installers.Context{
			SSH:       e.ssh,
			Detection: e.detection,
			Output:    e.outputCallback,
			Tail:      tailFn,
		}

		if err := installers.EnsureRuntimeInstalled(ctx); err != nil {
			return err
		}
	}

	return nil
}

// SetupDirectoryStructure creates the deployment directory structure
func (e *Executor) SetupDirectoryStructure() error {
	appPath := fmt.Sprintf("%s/%s", config.RemoteAppBaseDir, e.appName)

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

	result := e.ssh.ExecuteSudo(fmt.Sprintf("chown -R deploy:deploy %s", appPath))
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
	releasePath := fmt.Sprintf("%s/%s/releases/%s", config.RemoteAppBaseDir, e.appName, timestamp)

	result := e.ssh.ExecuteSudo(fmt.Sprintf("mkdir -p %s", releasePath))
	if result.Error != nil {
		return "", fmt.Errorf("failed to create release directory: %w", result.Error)
	}
	if result.ExitCode != 0 {
		errMsg := result.Stderr
		if errMsg == "" {
			errMsg = result.Stdout
		}
		return "", fmt.Errorf("failed to create release directory (exit code %d): %s", result.ExitCode, errMsg)
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
	// Set ownership to deploy user (service runs as deploy)
	result = e.ssh.ExecuteSudo(fmt.Sprintf("chown -R deploy:deploy %s", releasePath))
	if result.Error != nil || result.ExitCode != 0 {
		return "", fmt.Errorf("failed to set ownership: %s", result.Stderr)
	}

	return releasePath, nil
}

func (e *Executor) BuildRelease(releasePath string) error {
	return e.BuildReleaseWithEnv(releasePath, nil)
}

// getBuildPlan returns custom build commands if set, otherwise detection defaults
func (e *Executor) getBuildPlan() []string {
	if e.deployOptions != nil && len(e.deployOptions.BuildCommands) > 0 {
		return e.deployOptions.BuildCommands
	}
	if e.detection != nil {
		return e.detection.BuildPlan
	}
	return nil
}

// getRunPlan returns custom run commands if set, otherwise detection defaults
func (e *Executor) getRunPlan() []string {
	if e.deployOptions != nil && len(e.deployOptions.RunCommands) > 0 {
		return e.deployOptions.RunCommands
	}
	if e.detection != nil {
		return e.detection.RunPlan
	}
	return nil
}

// isStaticSite returns true if the detected app is a static site (no server process needed)
func (e *Executor) isStaticSite() bool {
	if e.detection == nil || e.detection.Meta == nil {
		return false
	}
	deploymentType, ok := e.detection.Meta["deployment_type"]
	return ok && deploymentType == "static"
}

func (e *Executor) BuildReleaseWithEnv(releasePath string, envVars map[string]string) error {
	buildPlan := e.getBuildPlan()
	if len(buildPlan) == 0 {
		return nil
	}

	// Load local .env file and merge with provided envVars
	localEnvVars := e.LoadLocalEnvFile()
	if envVars == nil {
		envVars = make(map[string]string)
	}
	// Merge: provided envVars take precedence over local .env
	for key, value := range localEnvVars {
		if _, exists := envVars[key]; !exists {
			envVars[key] = value
		}
	}

	// Write .env file to release directory BEFORE building (needed for Next.js NEXT_PUBLIC_* vars)
	if len(envVars) > 0 {
		var envContent strings.Builder
		for key, value := range envVars {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}

		envPath := fmt.Sprintf("%s/.env", releasePath)
		if err := e.ssh.WriteRemoteFile(envPath, envContent.String(), config.PermEnvFile); err != nil {
			return fmt.Errorf("failed to write .env for build: %w", err)
		}

		// Set ownership to deploy user for build
		e.ssh.ExecuteSudo(fmt.Sprintf("chown deploy:deploy %s", envPath))
	}

	if e.detection != nil && e.detection.Language == "Python" {
		venvPath := fmt.Sprintf("%s/%s/shared/venv", config.RemoteAppBaseDir, e.appName)
		result := e.ssh.ExecuteSudo(fmt.Sprintf("python3 -m venv %s", venvPath))
		if result.Error != nil || result.ExitCode != 0 {
			return fmt.Errorf("failed to create venv: %s", result.Stderr)
		}
		e.ssh.ExecuteSudo(fmt.Sprintf("chown -R deploy:deploy %s", venvPath))
	}

	for _, cmd := range buildPlan {
		trimmed := strings.TrimSpace(cmd)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		buildCmd := e.adjustBuildCommand(cmd, releasePath)

		pathPrefix := e.getPackageManagerPath()
		fullCmd := fmt.Sprintf("cd %s && %s%s", releasePath, pathPrefix, buildCmd)

		result := e.ssh.Execute(fullCmd)
		if result.Error != nil || result.ExitCode != 0 {
			e.sendOutput(result.Stdout, 15)
			e.sendOutput(result.Stderr, 15)

			errorOutput := result.Stderr
			if result.Stdout != "" {
				errorOutput = result.Stdout + "\n" + result.Stderr
			}

			if result.ExitCode == 137 || result.ExitCode == 143 || strings.Contains(errorOutput, "Killed") {
				suggestions := "Suggestions:\n  - Increase server memory (upgrade droplet size)\n  - Add swap space to the server"

				if strings.Contains(cmd, "bun") {
					suggestions += "\n  - Use npm instead of bun (bun uses more memory during install)"
				} else if strings.Contains(cmd, "poetry") || strings.Contains(cmd, "pip") {
					suggestions += "\n  - Use --no-cache-dir flag with pip to reduce memory usage"
				}

				return fmt.Errorf("build command failed '%s' (exit code %d):\n\n%s\n\nProcess was killed, likely due to insufficient memory (OOM).\n%s", cmd, result.ExitCode, errorOutput, suggestions)
			}

			return fmt.Errorf("build command failed '%s' (exit code %d): %s", cmd, result.ExitCode, errorOutput)
		}

		e.sendOutput(result.Stdout, 5)
	}

	result := e.ssh.ExecuteSudo(fmt.Sprintf("chown -R deploy:deploy %s", releasePath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to set ownership to deploy: %s", result.Stderr)
	}

	return nil
}

// LoadLocalEnvFile searches for and loads environment variables from local .env files
// Priority order: .env.production -> .env.prod -> .env
func (e *Executor) LoadLocalEnvFile() map[string]string {
	envFilePath := e.FindEnvFile()
	if envFilePath == "" {
		return nil
	}

	return ParseEnvFile(envFilePath)
}

// FindEnvFile searches for .env files in priority order
// Returns the path to the first file found, or empty string if none exist
func (e *Executor) FindEnvFile() string {
	candidates := []string{
		".env.production",
		".env.prod",
		".env",
	}

	for _, candidate := range candidates {
		path := filepath.Join(e.projectPath, candidate)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// ParseEnvFile reads and parses a .env file into a map
func ParseEnvFile(path string) map[string]string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	envVars := make(map[string]string)
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove quotes if present
			value = strings.Trim(value, `"'`)

			envVars[key] = value
		}
	}

	return envVars
}

// getPackageManagerPath returns the PATH prefix needed for package managers
func (e *Executor) getPackageManagerPath() string {
	if e.detection == nil {
		return ""
	}

	if e.detection.Language == "JavaScript/TypeScript" {
		basePath := "export PATH=\"/usr/bin:$PATH\" && export NODE=\"/usr/bin/node\" && hash -r && "

		pm, ok := e.detection.Meta["package_manager"]
		if !ok {
			return basePath
		}

		switch pm {
		case "bun":
			return basePath + "export PATH=\"$HOME/.bun/bin:$PATH\" && "
		case "pnpm":
			return basePath + "export PNPM_HOME=\"$HOME/.local/share/pnpm\" && export PATH=\"$PNPM_HOME:$PATH\" && "
		case "yarn":
			return basePath
		default:
			return basePath
		}
	}

	pm, ok := e.detection.Meta["package_manager"]
	if !ok {
		return ""
	}

	switch pm {
	case "poetry", "uv", "pipenv":
		return "export PATH=\"$HOME/.local/bin:$PATH\" && "
	default:
		return ""
	}
}

func (e *Executor) adjustBuildCommand(cmd string, _ string) string {
	if e.detection == nil {
		return cmd
	}

	if pm, ok := e.detection.Meta["package_manager"]; ok {
		switch pm {
		case "bun":
			if strings.Contains(cmd, "bun") {
				return "(command -v bun >/dev/null 2>&1 || curl -fsSL https://bun.sh/install | bash) && " + cmd
			}
		case "pnpm":
			if strings.Contains(cmd, "pnpm") {
				return "npm install -g pnpm && " + cmd
			}
		case "poetry":
			if strings.Contains(cmd, "poetry") {
				return "pip3 install poetry && " + cmd
			}
		case "uv":
			if strings.Contains(cmd, "uv") {
				return "pip3 install uv && " + cmd
			}
		case "pipenv":
			if strings.Contains(cmd, "pipenv") {
				return "pip3 install pipenv && " + cmd
			}
		}
	}

	switch e.detection.Language {
	case "Python":
		if strings.Contains(cmd, "pip install") {
			venvPath := fmt.Sprintf("%s/%s/shared/venv", config.RemoteAppBaseDir, e.appName)
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
			return "(command -v bun >/dev/null 2>&1 || curl -fsSL https://bun.sh/install | bash) && " + cmd
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

	tmpEnvPath := "/tmp/lightfold.env"
	if err := e.ssh.WriteRemoteFile(tmpEnvPath, envContent.String(), config.PermEnvFile); err != nil {
		return fmt.Errorf("failed to write environment file: %w", err)
	}

	finalEnvPath := fmt.Sprintf("%s/%s/shared/env/.env", config.RemoteAppBaseDir, e.appName)
	result := e.ssh.ExecuteSudo(fmt.Sprintf("mv %s %s", tmpEnvPath, finalEnvPath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to move env file to final location: %s", result.Stderr)
	}

	result = e.ssh.ExecuteSudo(fmt.Sprintf("chown deploy:deploy %s", finalEnvPath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to set env file ownership: %s", result.Stderr)
	}

	return nil
}

func (e *Executor) GetCurrentRelease() (string, error) {
	currentLink := fmt.Sprintf("%s/%s/current", config.RemoteAppBaseDir, e.appName)
	result := e.ssh.Execute(fmt.Sprintf("readlink -f %s", currentLink))
	if result.Error != nil || result.ExitCode != 0 {
		return "", nil
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (e *Executor) ListReleases() ([]string, error) {
	releasesPath := fmt.Sprintf("%s/%s/releases", config.RemoteAppBaseDir, e.appName)
	result := e.ssh.Execute(fmt.Sprintf("ls -1t %s", releasesPath))
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

	toDelete := releases[keepCount:]

	for _, release := range toDelete {
		releasePath := fmt.Sprintf("%s/%s/releases/%s", config.RemoteAppBaseDir, e.appName, release)
		result := e.ssh.ExecuteSudo(fmt.Sprintf("rm -rf %s", releasePath))
		if result.Error != nil || result.ExitCode != 0 {
			return fmt.Errorf("failed to delete release %s: %s", release, result.Stderr)
		}
	}

	return nil
}

func (e *Executor) GenerateSystemdUnit(releasePath string) error {
	// Static sites don't need a systemd service (nginx serves files directly)
	if e.isStaticSite() {
		return nil
	}

	execStart := e.getExecStartCommand()

	data := map[string]string{
		"APP_NAME":   e.appName,
		"EXEC_START": execStart,
	}

	tmpPath := fmt.Sprintf("/tmp/%s.service", e.appName)
	if err := e.ssh.RenderAndWriteTemplate(systemdTemplate, data, tmpPath, config.PermConfigFile); err != nil {
		return fmt.Errorf("failed to write systemd unit to temp: %w", err)
	}

	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", e.appName)
	result := e.ssh.ExecuteSudo(fmt.Sprintf("mv %s %s", tmpPath, unitPath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to move systemd unit to /etc: %s", result.Stderr)
	}

	result = e.ssh.ExecuteSudo(fmt.Sprintf("chown root:root %s", unitPath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to set systemd unit ownership: %s", result.Stderr)
	}

	result = e.ssh.ExecuteSudo("systemctl daemon-reload")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to reload systemd: %s", result.Stderr)
	}

	return nil
}

// adjustPackageManagerPath replaces package manager commands with full paths
// This is necessary because systemd doesn't execute with user's shell environment
// Only replaces if the command starts with the package manager name
func adjustPackageManagerPath(runCommand, packageManager string) string {
	var fullPath string

	switch packageManager {
	case "bun":
		fullPath = "/home/deploy/.bun/bin/bun"
	case "pnpm":
		fullPath = "/home/deploy/.local/share/pnpm/pnpm"
	case "npm":
		fullPath = "/usr/bin/npm"
	case "yarn":
		fullPath = "/usr/bin/yarn"
	default:
		return runCommand
	}

	if strings.HasPrefix(runCommand, packageManager+" ") {
		return strings.Replace(runCommand, packageManager+" ", fullPath+" ", 1)
	}

	return runCommand
}

func (e *Executor) getExecStartCommand() string {
	if e.startCommand != "" {
		return e.startCommand
	}

	runPlan := e.getRunPlan()
	if len(runPlan) > 0 {
		runCommand := runPlan[0]

		if e.detection != nil && e.detection.Language == "JavaScript/TypeScript" {
			pm, _ := e.detection.Meta["package_manager"]
			return adjustPackageManagerPath(runCommand, pm)
		}

		if e.detection != nil && e.detection.Language == "Python" {
			venvBin := fmt.Sprintf("%s/%s/shared/venv/bin", config.RemoteAppBaseDir, e.appName)
			if strings.Contains(runCommand, "uvicorn ") {
				return strings.Replace(runCommand, "uvicorn ", venvBin+"/uvicorn ", 1)
			}
			if strings.Contains(runCommand, "gunicorn ") {
				return strings.Replace(runCommand, "gunicorn ", venvBin+"/gunicorn ", 1)
			}
		}

		return runCommand
	}

	if e.detection == nil {
		return "/usr/bin/true"
	}

	framework := e.detection.Framework
	language := e.detection.Language
	appPath := fmt.Sprintf("%s/%s", config.RemoteAppBaseDir, e.appName)

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
			return fmt.Sprintf("/usr/bin/node %s/current/.next/standalone/server.js", appPath)
		case "Express.js":
			return fmt.Sprintf("/usr/bin/node %s/current/server.js", appPath)
		case "NestJS":
			return fmt.Sprintf("/usr/bin/node %s/current/dist/main.js", appPath)
		}

	case "Go":
		return fmt.Sprintf("%s/current/app --port 8000", appPath)

	case "Ruby":
		if framework == "Rails" {
			return fmt.Sprintf("%s/shared/bundle/bin/puma -C %s/current/config/puma.rb", appPath, appPath)
		}
	}

	return "/usr/bin/true"
}

// GenerateNginxConfig creates an nginx configuration (reverse proxy for SSR or static file server for static sites)
// If domain is empty, nginx configuration is skipped (app listens directly on port)
func (e *Executor) GenerateNginxConfig(port int, domain string) error {
	// Skip nginx if no domain is configured (multi-app without domain scenario)
	if domain == "" {
		return nil
	}

	data := map[string]string{
		"APP_NAME": e.appName,
		"PORT":     fmt.Sprintf("%d", port),
		"DOMAIN":   domain,
	}

	// Use different templates for static vs SSR sites
	template := nginxTemplate
	if e.isStaticSite() {
		template = nginxStaticTemplate
		// Get build output directory from detection metadata
		buildOutput := "dist/"
		if e.detection != nil && e.detection.Meta != nil {
			if output, ok := e.detection.Meta["build_output"]; ok {
				buildOutput = output
			}
		}
		data["BUILD_OUTPUT"] = buildOutput
	}

	tmpPath := fmt.Sprintf("/tmp/nginx-%s.conf", e.appName)
	if err := e.ssh.RenderAndWriteTemplate(template, data, tmpPath, config.PermConfigFile); err != nil {
		return fmt.Errorf("failed to write nginx config to temp: %w", err)
	}

	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", e.appName)
	result := e.ssh.ExecuteSudo(fmt.Sprintf("mv %s %s", tmpPath, configPath))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to move nginx config to /etc: %s", result.Stderr)
	}

	symlinkCmd := fmt.Sprintf("ln -sf /etc/nginx/sites-available/%s /etc/nginx/sites-enabled/%s", e.appName, e.appName)
	result = e.ssh.ExecuteSudo(symlinkCmd)
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to enable nginx site: %s", result.Stderr)
	}

	// Only remove default site if we have a domain configured
	e.ssh.ExecuteSudo("rm -f /etc/nginx/sites-enabled/default")

	return nil
}

func (e *Executor) TestNginxConfig() error {
	result := e.ssh.ExecuteSudo("nginx -t")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("nginx config test failed: %s", result.Stderr)
	}
	return nil
}

func (e *Executor) ReloadNginx() error {
	result := e.ssh.ExecuteSudo("systemctl reload nginx")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to reload nginx: %s", result.Stderr)
	}
	return nil
}

func (e *Executor) EnableService() error {
	// Static sites don't have a systemd service
	if e.isStaticSite() {
		return nil
	}

	result := e.ssh.ExecuteSudo(fmt.Sprintf("systemctl enable %s", e.appName))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to enable service: %s", result.Stderr)
	}
	return nil
}

func (e *Executor) StartService() error {
	result := e.ssh.ExecuteSudo(fmt.Sprintf("systemctl start %s", e.appName))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to start service: %s", result.Stderr)
	}
	return nil
}

func (e *Executor) RestartService() error {
	result := e.ssh.ExecuteSudo(fmt.Sprintf("systemctl restart %s", e.appName))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to restart service: %s", result.Stderr)
	}
	return nil
}

func (e *Executor) StopService() error {
	result := e.ssh.ExecuteSudo(fmt.Sprintf("systemctl stop %s", e.appName))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to stop service: %s", result.Stderr)
	}
	return nil
}

func (e *Executor) GetServiceStatus() (bool, error) {
	result := e.ssh.Execute(fmt.Sprintf("systemctl is-active %s", e.appName))
	isActive := result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == "active"
	return isActive, nil
}

func (e *Executor) SwitchRelease(releasePath string) error {
	currentLink := fmt.Sprintf("%s/%s/current", config.RemoteAppBaseDir, e.appName)
	tempLink := fmt.Sprintf("%s/%s/current.tmp", config.RemoteAppBaseDir, e.appName)

	result := e.ssh.ExecuteSudo(fmt.Sprintf("ln -sf %s %s", releasePath, tempLink))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to create temp symlink: %s", result.Stderr)
	}

	result = e.ssh.ExecuteSudo(fmt.Sprintf("mv -Tf %s %s", tempLink, currentLink))
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to switch release: %s", result.Stderr)
	}

	return nil
}

func (e *Executor) PerformHealthCheck(maxRetries int, retryDelay time.Duration) error {
	if e.detection == nil || e.detection.Healthcheck == nil {
		return nil
	}

	healthPath := "/"
	expectedStatus := 200
	timeout := int(config.DefaultHealthCheckTimeout.Seconds())

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
			return nil
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

	previousRelease := releases[len(releases)-2]
	previousPath := fmt.Sprintf("%s/%s/releases/%s", config.RemoteAppBaseDir, e.appName, previousRelease)

	if err := e.StopService(); err != nil {
		return fmt.Errorf("failed to stop service during rollback: %w", err)
	}

	if err := e.SwitchRelease(previousPath); err != nil {
		return fmt.Errorf("failed to switch to previous release: %w", err)
	}

	if err := e.StartService(); err != nil {
		return fmt.Errorf("failed to start service after rollback: %w", err)
	}

	return nil
}

// RollbackToRelease rolls back to a specific release by timestamp
func (e *Executor) RollbackToRelease(timestamp string) error {
	releasePath := fmt.Sprintf("%s/%s/releases/%s", config.RemoteAppBaseDir, e.appName, timestamp)

	result := e.ssh.Execute(fmt.Sprintf("test -d %s", releasePath))
	if result.ExitCode != 0 {
		return fmt.Errorf("release %s does not exist", timestamp)
	}

	if err := e.StopService(); err != nil {
		return fmt.Errorf("failed to stop service during rollback: %w", err)
	}

	if err := e.SwitchRelease(releasePath); err != nil {
		return fmt.Errorf("failed to switch to release %s: %w", timestamp, err)
	}

	if err := e.StartService(); err != nil {
		return fmt.Errorf("failed to start service after rollback: %w", err)
	}

	return nil
}

func (e *Executor) DeployWithHealthCheck(releasePath string, healthCheckRetries int, healthCheckDelay time.Duration) error {
	currentRelease, err := e.GetCurrentRelease()
	if err != nil {
		return fmt.Errorf("failed to get current release: %w", err)
	}

	if err := e.SwitchRelease(releasePath); err != nil {
		return fmt.Errorf("failed to switch release: %w", err)
	}

	// Static sites don't need systemd services or health checks
	if e.isStaticSite() {
		// Just reload nginx to pick up the new release
		return e.ReloadNginx()
	}

	// For SSR apps, manage systemd service and health checks
	isFirstDeploy := currentRelease == ""
	if isFirstDeploy {
		if err := e.StartService(); err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}
	} else {
		if err := e.RestartService(); err != nil {
			e.SwitchRelease(currentRelease)
			e.RestartService()
			return fmt.Errorf("failed to restart service: %w", err)
		}
	}

	if err := e.PerformHealthCheck(healthCheckRetries, healthCheckDelay); err != nil {
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
