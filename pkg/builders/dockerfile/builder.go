package dockerfile

import (
	"context"
	"fmt"
	"lightfold/pkg/builders"
	"lightfold/pkg/config"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DockerfileBuilder uses an existing Dockerfile for building
type DockerfileBuilder struct{}

func init() {
	builders.Register("dockerfile", func() builders.Builder {
		return &DockerfileBuilder{}
	})
}

func (d *DockerfileBuilder) Name() string {
	return "dockerfile"
}

func (d *DockerfileBuilder) IsAvailable() bool {
	// Check if docker command exists
	_, err := exec.LookPath("docker")
	if err != nil {
		return false
	}

	// Check if Docker daemon is accessible
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

func (d *DockerfileBuilder) NeedsNginx() bool {
	// Assume Dockerfile includes its own web server
	// User can configure nginx separately if needed
	return false
}

func (d *DockerfileBuilder) Build(ctx context.Context, opts *builders.BuildOptions) (*builders.BuildResult, error) {
	// Verify Dockerfile exists
	dockerfilePath := filepath.Join(opts.ProjectPath, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return &builders.BuildResult{
			Success: false,
		}, fmt.Errorf("Dockerfile not found at %s", dockerfilePath)
	}

	// Extract app name from release path
	appName := extractAppName(opts.ReleasePath)
	if appName == "" {
		return &builders.BuildResult{
			Success: false,
		}, fmt.Errorf("failed to extract app name from release path: %s", opts.ReleasePath)
	}

	imageName := fmt.Sprintf("lightfold-%s:latest", appName)
	tarballPath := filepath.Join(os.TempDir(), fmt.Sprintf("lightfold-%s-image.tar", appName))

	var buildLog strings.Builder

	// Step 1: Build Docker image locally
	buildLog.WriteString(fmt.Sprintf("Building Docker image: %s\n", imageName))
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, opts.ProjectPath)
	buildCmd.Dir = opts.ProjectPath

	buildOutput, err := buildCmd.CombinedOutput()
	buildLog.Write(buildOutput)

	if err != nil {
		return &builders.BuildResult{
			Success:  false,
			BuildLog: buildLog.String(),
		}, fmt.Errorf("docker build failed: %w\nOutput: %s", err, buildOutput)
	}

	// Step 2: Export image as tarball
	buildLog.WriteString(fmt.Sprintf("\nExporting image to tarball: %s\n", tarballPath))
	saveCmd := exec.CommandContext(ctx, "docker", "save", "-o", tarballPath, imageName)

	saveOutput, err := saveCmd.CombinedOutput()
	buildLog.Write(saveOutput)

	if err != nil {
		return &builders.BuildResult{
			Success:  false,
			BuildLog: buildLog.String(),
		}, fmt.Errorf("docker save failed: %w\nOutput: %s", err, saveOutput)
	}

	// Ensure cleanup of tarball
	defer os.Remove(tarballPath)

	// Step 3: Ensure Docker is installed on remote server
	buildLog.WriteString("\nChecking Docker on remote server...\n")
	ssh := opts.SSHExecutor

	dockerCheck := ssh.Execute("which docker")
	if dockerCheck.ExitCode != 0 {
		buildLog.WriteString("Installing Docker on remote server...\n")

		// Install Docker using official script
		installCmds := []string{
			"curl -fsSL https://get.docker.com -o /tmp/get-docker.sh",
			"sudo sh /tmp/get-docker.sh",
			"sudo usermod -aG docker deploy",
			"rm /tmp/get-docker.sh",
		}

		for _, cmd := range installCmds {
			result := ssh.ExecuteSudo(cmd)
			buildLog.WriteString(result.Stdout)
			buildLog.WriteString(result.Stderr)

			if result.ExitCode != 0 {
				return &builders.BuildResult{
					Success:  false,
					BuildLog: buildLog.String(),
				}, fmt.Errorf("failed to install Docker on remote server: %s", result.Stderr)
			}
		}

		buildLog.WriteString("Docker installed successfully\n")
	}

	// Step 4: Transfer tarball to remote server
	remoteTarballPath := fmt.Sprintf("/tmp/lightfold-%s-image.tar", appName)
	buildLog.WriteString(fmt.Sprintf("\nTransferring image to server: %s\n", remoteTarballPath))

	if err := ssh.UploadFile(tarballPath, remoteTarballPath); err != nil {
		return &builders.BuildResult{
			Success:  false,
			BuildLog: buildLog.String(),
		}, fmt.Errorf("failed to transfer image tarball: %w", err)
	}

	// Step 5: Load Docker image on remote server
	buildLog.WriteString("\nLoading Docker image on remote server...\n")
	loadResult := ssh.Execute(fmt.Sprintf("docker load -i %s", remoteTarballPath))
	buildLog.WriteString(loadResult.Stdout)
	buildLog.WriteString(loadResult.Stderr)

	if loadResult.ExitCode != 0 {
		ssh.Execute(fmt.Sprintf("rm %s", remoteTarballPath))
		return &builders.BuildResult{
			Success:  false,
			BuildLog: buildLog.String(),
		}, fmt.Errorf("docker load failed: %s", loadResult.Stderr)
	}

	// Cleanup remote tarball
	ssh.Execute(fmt.Sprintf("rm %s", remoteTarballPath))

	// Step 6: Write environment variables to .env file if provided
	if len(opts.EnvVars) > 0 {
		var envContent strings.Builder
		for key, value := range opts.EnvVars {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}

		envPath := fmt.Sprintf("%s/.env", opts.ReleasePath)
		if err := ssh.WriteRemoteFile(envPath, envContent.String(), config.PermEnvFile); err != nil {
			return &builders.BuildResult{
				Success:  false,
				BuildLog: buildLog.String(),
			}, fmt.Errorf("failed to write .env file: %w", err)
		}

		ssh.ExecuteSudo(fmt.Sprintf("chown deploy:deploy %s", envPath))
		buildLog.WriteString(fmt.Sprintf("\nWrote environment variables to %s\n", envPath))
	}

	// Step 7: Create docker-compose.yml or docker run script
	// We'll create a simple run script that can be used by systemd
	port := extractPortFromEnv(opts.EnvVars)
	if port == "" {
		port = fmt.Sprintf("%d", config.DefaultApplicationPort) // Default port
	}

	runScript := fmt.Sprintf(`#!/bin/bash
# Lightfold Docker container run script
CONTAINER_NAME="lightfold-%s"
IMAGE_NAME="%s"
RELEASE_PATH="%s"

# Stop and remove existing container if running
docker stop $CONTAINER_NAME 2>/dev/null || true
docker rm $CONTAINER_NAME 2>/dev/null || true

# Run new container
docker run -d \
  --name $CONTAINER_NAME \
  --restart unless-stopped \
  -p %s:3000 \
  --env-file $RELEASE_PATH/.env \
  $IMAGE_NAME
`, appName, imageName, opts.ReleasePath, port)

	runScriptPath := fmt.Sprintf("%s/docker-run.sh", opts.ReleasePath)
	if err := ssh.WriteRemoteFile(runScriptPath, runScript, 0755); err != nil {
		return &builders.BuildResult{
			Success:  false,
			BuildLog: buildLog.String(),
		}, fmt.Errorf("failed to write docker run script: %w", err)
	}

	ssh.ExecuteSudo(fmt.Sprintf("chown deploy:deploy %s", runScriptPath))
	buildLog.WriteString(fmt.Sprintf("\nCreated Docker run script at %s\n", runScriptPath))

	buildLog.WriteString("\n✓ Docker build completed successfully\n")

	return &builders.BuildResult{
		Success:       true,
		BuildLog:      buildLog.String(),
		IncludesNginx: false, // Docker containers typically include their own web server
		StartCommand:  fmt.Sprintf("bash %s/docker-run.sh", opts.ReleasePath),
	}, nil
}

// extractAppName extracts the app name from the release path
// Example: /srv/myapp/releases/20240101120000 -> myapp
func extractAppName(releasePath string) string {
	parts := strings.Split(releasePath, "/")
	if len(parts) >= 3 {
		// Path format: /srv/{appname}/releases/{timestamp}
		return parts[2]
	}
	return ""
}

// extractPortFromEnv tries to find PORT env var, defaults to ""
func extractPortFromEnv(envVars map[string]string) string {
	if port, ok := envVars["PORT"]; ok {
		return port
	}
	return ""
}
