package deploy

import (
	"context"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	"lightfold/pkg/flyctl"
	"lightfold/pkg/providers/flyio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FlyioDeployer handles fly.io-specific container deployments
type FlyioDeployer struct {
	appName     string
	projectPath string
	targetName  string
	detection   *detector.Detection
	flyConfig   *config.FlyioConfig
	token       string
	callback    ProgressCallback
}

// NewFlyioDeployer creates a new fly.io deployer
func NewFlyioDeployer(appName, projectPath, targetName string, detection *detector.Detection, flyConfig *config.FlyioConfig, token string) *FlyioDeployer {
	return &FlyioDeployer{
		appName:     appName,
		projectPath: projectPath,
		targetName:  targetName,
		detection:   detection,
		flyConfig:   flyConfig,
		token:       token,
	}
}

// SetProgressCallback sets the callback for progress updates
func (d *FlyioDeployer) SetProgressCallback(callback ProgressCallback) {
	d.callback = callback
}

// Deploy executes a full fly.io deployment using native nixpacks
func (d *FlyioDeployer) Deploy(ctx context.Context, deployOpts *config.DeploymentOptions) error {
	// Step 1: Check flyctl availability
	d.updateProgress("Checking flyctl availability", 10)
	if err := flyctl.EnsureFlyctl(); err != nil {
		return fmt.Errorf("flyctl check failed: %w", err)
	}

	// Step 2: Generate fly.toml (optional - for health checks)
	d.updateProgress("Generating fly.toml", 20)
	flyTomlContent, err := flyio.GenerateFlyToml(d.detection, d.flyConfig.AppName, d.flyConfig.Region, d.flyConfig.Size)
	if err != nil {
		return fmt.Errorf("failed to generate fly.toml: %w", err)
	}

	// Write fly.toml to project
	flyTomlPath := filepath.Join(d.projectPath, "fly.toml")
	existingFlyToml := false
	if _, err := os.Stat(flyTomlPath); err == nil {
		existingFlyToml = true
	}

	if err := os.WriteFile(flyTomlPath, []byte(flyTomlContent), 0644); err != nil {
		return fmt.Errorf("failed to write fly.toml: %w", err)
	}
	if !existingFlyToml {
		defer os.Remove(flyTomlPath)
	}

	// Step 3: Set secrets/env vars
	d.updateProgress("Setting environment variables", 40)
	client := flyctl.NewClient(d.token, d.flyConfig.AppName)

	if deployOpts != nil && len(deployOpts.EnvVars) > 0 {
		if err := client.SetSecrets(ctx, deployOpts.EnvVars); err != nil {
			return fmt.Errorf("failed to set secrets: %w", err)
		}
	}

	// Step 4: Deploy with fly.io's native nixpacks builder
	d.updateProgress("Deploying with fly.io nixpacks (remote build)", 50)
	output, err := client.Deploy(ctx, flyctl.DeployOptions{
		ProjectPath: d.projectPath,
		Region:      d.flyConfig.Region,
		RemoteOnly:  true,
		UseNixpacks: true, // Use fly.io's native nixpacks - no Dockerfile needed!
	})

	if err != nil {
		// Show last few lines of output for debugging
		lines := strings.Split(strings.TrimSpace(output), "\n")
		lastLines := ""
		if len(lines) > 10 {
			lastLines = strings.Join(lines[len(lines)-10:], "\n")
		} else {
			lastLines = output
		}
		return fmt.Errorf("deployment failed:\n%s\n%w", lastLines, err)
	}

	// Step 5: Wait for deployment to be healthy
	d.updateProgress("Waiting for health checks", 80)
	if err := d.waitForHealthy(ctx, client); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	d.updateProgress("Deployment complete", 100)
	return nil
}

// waitForHealthy waits for the app to pass health checks
func (d *FlyioDeployer) waitForHealthy(ctx context.Context, client *flyctl.Client) error {
	deadline := time.Now().Add(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for app to become healthy")
			}

			status, err := client.GetStatus(ctx)
			if err != nil {
				// Non-fatal, continue waiting
				continue
			}

			statusLower := strings.ToLower(status)
			if strings.Contains(statusLower, "running") || strings.Contains(statusLower, "started") {
				return nil
			}
		}
	}
}

// updateProgress sends progress updates via callback
func (d *FlyioDeployer) updateProgress(description string, progress int) {
	if d.callback != nil {
		d.callback(DeploymentStep{
			Name:        "deploy",
			Description: description,
			Progress:    progress,
		})
	}
}

// GetLogs retrieves logs from fly.io
func (d *FlyioDeployer) GetLogs(ctx context.Context, lines int, follow bool) (string, error) {
	client := flyctl.NewClient(d.token, d.flyConfig.AppName)
	return client.GetLogs(ctx, lines, follow)
}

// GetStatus retrieves app status from fly.io
func (d *FlyioDeployer) GetStatus(ctx context.Context) (string, error) {
	client := flyctl.NewClient(d.token, d.flyConfig.AppName)
	return client.GetStatus(ctx)
}
