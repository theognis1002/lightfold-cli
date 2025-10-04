package deploy

import (
	"context"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	"lightfold/pkg/providers"
	"lightfold/pkg/providers/cloudinit"
	_ "lightfold/pkg/providers/digitalocean" // Register DigitalOcean provider
	_ "lightfold/pkg/providers/hetzner"      // Register Hetzner provider
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/util"
	"os"
	"strings"
	"time"
)

// DeploymentStep represents a step in the deployment process
type DeploymentStep struct {
	Name        string
	Description string
	Progress    int // 0-100
}

// DeploymentResult contains the result of a deployment operation
type DeploymentResult struct {
	Success bool
	Server  *providers.Server
	Message string
	Error   error
	Steps   []DeploymentStep
}

// ProgressCallback is called for each deployment step
type ProgressCallback func(step DeploymentStep)

// Orchestrator manages the deployment process
type Orchestrator struct {
	config           config.TargetConfig
	projectPath      string
	projectName      string
	targetName       string
	tokens           *config.TokenConfig
	progressCallback ProgressCallback
}

// GetOrchestrator creates a new deployment orchestrator
func GetOrchestrator(targetConfig config.TargetConfig, projectPath, projectName, targetName string) (*Orchestrator, error) {
	tokens, err := config.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to load tokens: %w", err)
	}

	return &Orchestrator{
		config:      targetConfig,
		projectPath: projectPath,
		projectName: projectName,
		targetName:  targetName,
		tokens:      tokens,
	}, nil
}

// SetProgressCallback sets the callback for progress updates
func (o *Orchestrator) SetProgressCallback(callback ProgressCallback) {
	o.progressCallback = callback
}

func (o *Orchestrator) Deploy(ctx context.Context) (*DeploymentResult, error) {
	if !providers.IsRegistered(o.config.Provider) {
		return nil, fmt.Errorf("unknown provider: %s", o.config.Provider)
	}

	if err := o.checkExistingServer(); err != nil {
		return nil, err
	}

	return o.deployWithProvider(ctx)
}

func (o *Orchestrator) checkExistingServer() error {
	switch o.config.Provider {
	case "digitalocean":
		doConfig, err := o.config.GetDigitalOceanConfig()
		if err == nil && doConfig.Provisioned {
			if doConfig.DropletID != "" && doConfig.IP != "" {
				return fmt.Errorf("server already provisioned (ID: %s, IP: %s). Use a different command to redeploy or destroy the existing server first",
					doConfig.DropletID,
					doConfig.IP)
			}
		}
	case "hetzner":
		hetznerConfig, err := o.config.GetHetznerConfig()
		if err == nil && hetznerConfig.Provisioned {
			if hetznerConfig.ServerID != "" && hetznerConfig.IP != "" {
				return fmt.Errorf("server already provisioned (ID: %s, IP: %s). Use a different command to redeploy or destroy the existing server first",
					hetznerConfig.ServerID,
					hetznerConfig.IP)
			}
		}
	case "s3":
		return nil
	}
	return nil
}

func (o *Orchestrator) deployWithProvider(ctx context.Context) (*DeploymentResult, error) {
	token := o.tokens.GetToken(o.config.Provider)

	if o.config.Provider == "s3" {
		return o.deployS3(ctx)
	}

	var providerCfg config.ProviderConfig
	var err error

	switch o.config.Provider {
	case "digitalocean":
		providerCfg, err = o.config.GetDigitalOceanConfig()
	case "hetzner":
		providerCfg, err = o.config.GetHetznerConfig()
	default:
		return nil, fmt.Errorf("unsupported provider for SSH deployment: %s", o.config.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get provider configuration: %w", err)
	}

	if providerCfg.IsProvisioned() && providerCfg.GetIP() == "" {
		return o.provisionServer(ctx, token)
	}

	return o.deployToServer(ctx, providerCfg)
}

func (o *Orchestrator) provisionServer(ctx context.Context, token string) (*DeploymentResult, error) {
	result := &DeploymentResult{
		Steps: []DeploymentStep{},
	}

	// Step 1: Initialize provider client
	o.notifyProgress(DeploymentStep{
		Name:        "init_client",
		Description: fmt.Sprintf("Initializing %s client...", o.config.Provider),
		Progress:    5,
	})

	if token == "" {
		return nil, fmt.Errorf("%s API token not found", o.config.Provider)
	}

	client, err := providers.GetProvider(o.config.Provider, token)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}

	// Step 2: Validate credentials
	o.notifyProgress(DeploymentStep{
		Name:        "validate_credentials",
		Description: "Validating API credentials...",
		Progress:    10,
	})

	if err := client.ValidateCredentials(ctx); err != nil {
		return nil, fmt.Errorf("failed to validate credentials: %w", err)
	}

	var sshKeyPath, username, region, size, sshKeyName string

	switch o.config.Provider {
	case "digitalocean":
		doConfig, err := o.config.GetDigitalOceanConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get DigitalOcean config: %w", err)
		}
		sshKeyPath = doConfig.SSHKey
		username = doConfig.Username
		region = doConfig.Region
		size = doConfig.Size
		sshKeyName = doConfig.SSHKeyName
	case "hetzner":
		hetznerConfig, err := o.config.GetHetznerConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get Hetzner config: %w", err)
		}
		sshKeyPath = hetznerConfig.SSHKey
		username = hetznerConfig.Username
		region = hetznerConfig.Location
		size = hetznerConfig.ServerType
		sshKeyName = hetznerConfig.SSHKeyName
	default:
		return nil, fmt.Errorf("unsupported provider for provisioning: %s", o.config.Provider)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "load_ssh_key",
		Description: "Loading SSH key...",
		Progress:    20,
	})

	publicKeyPath := sshKeyPath + ".pub"
	publicKey, err := sshpkg.LoadPublicKey(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "upload_ssh_key",
		Description: fmt.Sprintf("Uploading SSH key to %s...", client.DisplayName()),
		Progress:    30,
	})

	if sshKeyName == "" {
		sshKeyName = fmt.Sprintf("lightfold-%s", o.projectName)
	}

	uploadedKey, err := client.UploadSSHKey(ctx, sshKeyName, publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to upload SSH key: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "generate_cloudinit",
		Description: "Generating cloud-init configuration...",
		Progress:    40,
	})

	userData, err := cloudinit.GenerateWebAppUserData(username, publicKey, o.projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud-init: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "create_server",
		Description: fmt.Sprintf("Creating %s server...", client.DisplayName()),
		Progress:    50,
	})

	// Sanitize project name for use as hostname (only a-z, A-Z, 0-9, . and -)
	sanitizedName := util.SanitizeHostname(o.projectName)

	provisionConfig := providers.ProvisionConfig{
		Name:              fmt.Sprintf("%s-app", sanitizedName),
		Region:            region,
		Size:              size,
		Image:             "ubuntu-22-04-x64",
		SSHKeys:           []string{uploadedKey.ID},
		UserData:          userData,
		Tags:              []string{"lightfold", "auto-provisioned", o.projectName},
		BackupsEnabled:    false,
		MonitoringEnabled: true,
	}

	server, err := client.Provision(ctx, provisionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to provision server: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "wait_active",
		Description: fmt.Sprintf("Waiting for server %s to become active...", server.ID),
		Progress:    70,
	})

	server, err = client.WaitForActive(ctx, server.ID, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for server: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "update_config",
		Description: "Updating configuration with server details...",
		Progress:    90,
	})

	switch o.config.Provider {
	case "digitalocean":
		doConfig, _ := o.config.GetDigitalOceanConfig()
		doConfig.IP = server.PublicIPv4
		doConfig.DropletID = server.ID
		o.config.SetProviderConfig("digitalocean", doConfig)
	case "hetzner":
		hetznerConfig, _ := o.config.GetHetznerConfig()
		hetznerConfig.IP = server.PublicIPv4
		hetznerConfig.ServerID = server.ID
		o.config.SetProviderConfig("hetzner", hetznerConfig)
	}

	// Save the updated configuration to disk
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config for saving: %w", err)
	}
	if err := cfg.SetTarget(o.targetName, o.config); err != nil {
		return nil, fmt.Errorf("failed to set target config: %w", err)
	}
	if err := cfg.SaveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "complete",
		Description: fmt.Sprintf("Provisioning complete! (Server IP: %s)", server.PublicIPv4),
		Progress:    100,
	})

	result.Success = true
	result.Server = server
	result.Message = fmt.Sprintf("Successfully provisioned server at %s", server.PublicIPv4)

	return result, nil
}

func (o *Orchestrator) deployS3(ctx context.Context) (*DeploymentResult, error) {
	return &DeploymentResult{
		Success: false,
		Message: "S3 deployment not yet implemented",
		Error:   fmt.Errorf("S3 deployment not yet implemented"),
	}, nil
}

func (o *Orchestrator) ConfigureServer(ctx context.Context, providerCfg config.ProviderConfig) (*DeploymentResult, error) {
	result := &DeploymentResult{
		Steps: []DeploymentStep{},
	}

	o.notifyProgress(DeploymentStep{
		Name:        "start_config",
		Description: fmt.Sprintf("Configuring target '%s' (%s on %s)...", o.targetName, o.config.Framework, o.config.Provider),
		Progress:    2,
	})

	o.notifyProgress(DeploymentStep{
		Name:        "detect_framework",
		Description: "Analyzing target app...",
		Progress:    5,
	})

	detection := detector.DetectFramework(o.projectPath)

	o.notifyProgress(DeploymentStep{
		Name:        "connect_ssh",
		Description: fmt.Sprintf("Connecting to server at %s...", providerCfg.GetIP()),
		Progress:    10,
	})

	sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
	defer sshExecutor.Disconnect()

	if err := sshExecutor.Connect(17, 10*time.Second); err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Check if server is already configured
	markerCheck := sshExecutor.Execute("test -f /etc/lightfold/configured && echo 'configured'")
	isConfigured := markerCheck.ExitCode == 0 && strings.TrimSpace(markerCheck.Stdout) == "configured"

	executor := NewExecutor(sshExecutor, o.projectName, o.projectPath, &detection)

	// Set up output callback to show command output as progress
	executor.SetOutputCallback(func(line string) {
		if o.progressCallback != nil {
			o.progressCallback(DeploymentStep{
				Name:        "command_output",
				Description: line,
				Progress:    -1, // -1 indicates this is output, not a progress step
			})
		}
	})

	o.notifyProgress(DeploymentStep{
		Name:        "wait_cloud_init",
		Description: "Initializing server...",
		Progress:    15,
	})

	// Wait for apt locks regardless of configured status
	if err := executor.WaitForAptLock(30, 10*time.Second); err != nil {
		return nil, fmt.Errorf("failed to acquire apt lock: %w", err)
	}

	// Only run full initial setup if not already configured
	if !isConfigured {
		o.notifyProgress(DeploymentStep{
			Name:        "install_packages",
			Description: "Installing system packages and runtimes...",
			Progress:    20,
		})

		if err := executor.InstallBasePackages(); err != nil {
			return nil, fmt.Errorf("failed to install packages: %w", err)
		}

		o.notifyProgress(DeploymentStep{
			Name:        "setup_directories",
			Description: "Setting up deployment directories...",
			Progress:    30,
		})

		if err := executor.SetupDirectoryStructure(); err != nil {
			return nil, fmt.Errorf("failed to setup directories: %w", err)
		}
	} else {
		// Server configured, but ensure runtimes are up to date
		o.notifyProgress(DeploymentStep{
			Name:        "verify_runtimes",
			Description: "Verifying runtime versions...",
			Progress:    20,
		})

		if err := executor.InstallBasePackages(); err != nil {
			return nil, fmt.Errorf("failed to update packages: %w", err)
		}

		o.notifyProgress(DeploymentStep{
			Name:        "skip_initial_setup",
			Description: "Server already configured...",
			Progress:    30,
		})
	}

	o.notifyProgress(DeploymentStep{
		Name:        "create_tarball",
		Description: "Creating release tarball...",
		Progress:    40,
	})

	tmpTarball := fmt.Sprintf("/tmp/lightfold-%s-release.tar.gz", o.projectName)
	if err := executor.CreateReleaseTarball(tmpTarball); err != nil {
		return nil, fmt.Errorf("failed to create tarball: %w", err)
	}
	defer os.Remove(tmpTarball)

	o.notifyProgress(DeploymentStep{
		Name:        "upload_release",
		Description: "Uploading release to server...",
		Progress:    50,
	})

	releasePath, err := executor.UploadRelease(tmpTarball)
	if err != nil {
		return nil, fmt.Errorf("failed to upload release: %w", err)
	}

	// Get env vars first (needed for build-time vars like NEXT_PUBLIC_*)
	envVars := make(map[string]string)
	if o.config.Deploy != nil && o.config.Deploy.EnvVars != nil {
		envVars = o.config.Deploy.EnvVars
	}

	skipBuild := o.config.Deploy != nil && o.config.Deploy.SkipBuild
	if !skipBuild {
		o.notifyProgress(DeploymentStep{
			Name:        "build_release",
			Description: "Building app...",
			Progress:    60,
		})

		// Pass env vars to build (for NEXT_PUBLIC_*, etc.)
		if err := executor.BuildReleaseWithEnv(releasePath, envVars); err != nil {
			return nil, fmt.Errorf("failed to build release: %w", err)
		}
	}

	o.notifyProgress(DeploymentStep{
		Name:        "write_env",
		Description: "Configuring environment variables...",
		Progress:    65,
	})

	// Also write to shared location for runtime
	if err := executor.WriteEnvironmentFile(envVars); err != nil {
		return nil, fmt.Errorf("failed to write environment: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "configure_service",
		Description: "Configuring systemd service...",
		Progress:    70,
	})

	if err := executor.GenerateSystemdUnit(releasePath); err != nil {
		return nil, fmt.Errorf("failed to generate systemd unit: %w", err)
	}

	if err := executor.EnableService(); err != nil {
		return nil, fmt.Errorf("failed to enable service: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "configure_nginx",
		Description: "Configuring nginx reverse proxy...",
		Progress:    75,
	})

	if err := executor.GenerateNginxConfig(); err != nil {
		return nil, fmt.Errorf("failed to generate nginx config: %w", err)
	}

	if err := executor.TestNginxConfig(); err != nil {
		return nil, fmt.Errorf("nginx config test failed: %w", err)
	}

	if err := executor.ReloadNginx(); err != nil {
		return nil, fmt.Errorf("failed to reload nginx: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "deploy_app",
		Description: "Deploying application with health check...",
		Progress:    85,
	})

	if err := executor.DeployWithHealthCheck(releasePath, 5, 3*time.Second); err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "cleanup",
		Description: "Cleaning up old releases...",
		Progress:    95,
	})

	// Load config to get KeepReleases setting
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Warning: failed to load config for cleanup: %v\n", err)
		cfg = &config.Config{KeepReleases: 5} // Fallback to default
	}

	if err := executor.CleanupOldReleases(cfg.KeepReleases); err != nil {
		fmt.Printf("Warning: failed to cleanup old releases: %v\n", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "complete",
		Description: "Configuration complete!",
		Progress:    100,
	})

	// Write configured marker to indicate server is fully configured (only on first configure)
	if !isConfigured {
		sshExecutor.Execute("sudo mkdir -p /etc/lightfold")
		markerResult := sshExecutor.Execute("echo 'configured' | sudo tee /etc/lightfold/configured > /dev/null")
		if markerResult.Error != nil {
			return nil, fmt.Errorf("failed to write configured marker: %w", markerResult.Error)
		}
		if markerResult.ExitCode != 0 {
			return nil, fmt.Errorf("failed to write configured marker: command exited with code %d, stderr: %s", markerResult.ExitCode, markerResult.Stderr)
		}

		// Trigger system updates in the background (will reboot in 5 minutes after completion)
		o.notifyProgress(DeploymentStep{
			Name:        "schedule_updates",
			Description: "Scheduling system updates and reboot...",
			Progress:    98,
		})
		updateCmd := "nohup bash -c 'apt-get update && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -o Dpkg::Options::=\"--force-confdef\" -o Dpkg::Options::=\"--force-confold\" && shutdown -r +5' > /var/log/lightfold-update.log 2>&1 &"
		sshExecutor.ExecuteSudo(updateCmd)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Successfully configured %s on %s", o.projectName, providerCfg.GetIP())

	return result, nil
}

func (o *Orchestrator) deployToServer(ctx context.Context, providerCfg config.ProviderConfig) (*DeploymentResult, error) {
	return o.ConfigureServer(ctx, providerCfg)
}

func (o *Orchestrator) notifyProgress(step DeploymentStep) {
	if o.progressCallback != nil {
		o.progressCallback(step)
	}
}
