package deploy

import (
	"context"
	"fmt"
	"lightfold/pkg/builders"
	_ "lightfold/pkg/builders/dockerfile"
	_ "lightfold/pkg/builders/native"
	_ "lightfold/pkg/builders/nixpacks"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	"lightfold/pkg/providers"
	"lightfold/pkg/providers/cloudinit"
	_ "lightfold/pkg/providers/digitalocean" // Register DigitalOcean provider
	_ "lightfold/pkg/providers/flyio"        // Register Fly.io provider
	_ "lightfold/pkg/providers/hetzner"      // Register Hetzner provider
	_ "lightfold/pkg/providers/vultr"        // Register Vultr provider
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
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
	tokens           config.TokenConfig
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
	if o.config.Provider == "s3" {
		return nil
	}

	providerCfg, err := o.config.GetSSHProviderConfig()
	if err != nil {
		return nil
	}

	if providerCfg.IsProvisioned() && providerCfg.GetServerID() != "" && providerCfg.GetIP() != "" {
		return fmt.Errorf("server already provisioned (ID: %s, IP: %s). Use a different command to redeploy or destroy the existing server first",
			providerCfg.GetServerID(),
			providerCfg.GetIP())
	}

	return nil
}

func (o *Orchestrator) deployWithProvider(ctx context.Context) (*DeploymentResult, error) {
	token := o.tokens.GetToken(o.config.Provider)

	if o.config.Provider == "s3" {
		return o.deployS3(ctx)
	}

	// Check if this is a container provider (Fly.io)
	if o.config.Provider == "flyio" {
		// Check if app needs to be created first
		flyioConfig, err := o.config.GetFlyioConfig()
		if err != nil || flyioConfig.AppName == "" {
			// App not created yet - provision it first
			return o.provisionServer(ctx, token)
		}
		// App exists - deploy to it
		return o.deployFlyio(ctx, token)
	}

	providerCfg, err := o.config.GetSSHProviderConfig()
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

	o.notifyProgress(DeploymentStep{
		Name:        "validate_credentials",
		Description: "Validating API credentials...",
		Progress:    10,
	})

	if err := client.ValidateCredentials(ctx); err != nil {
		return nil, fmt.Errorf("failed to validate credentials: %w", err)
	}

	region, size, sshKeyPath, username, sshKeyName, err := o.getProvisioningParams()
	if err != nil {
		return nil, err
	}

	var uploadedKey *providers.SSHKey
	var publicKey string
	var userData string

	// Skip SSH setup for container providers
	if client.SupportsSSH() {
		o.notifyProgress(DeploymentStep{
			Name:        "load_ssh_key",
			Description: "Loading SSH key...",
			Progress:    20,
		})

		publicKeyPath := sshKeyPath + ".pub"
		publicKey, err = sshpkg.LoadPublicKey(publicKeyPath)
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

		uploadedKey, err = client.UploadSSHKey(ctx, sshKeyName, publicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to upload SSH key: %w", err)
		}

		o.notifyProgress(DeploymentStep{
			Name:        "generate_cloudinit",
			Description: "Generating cloud-init configuration...",
			Progress:    40,
		})

		userData, err = cloudinit.GenerateWebAppUserData(username, publicKey, o.projectName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate cloud-init: %w", err)
		}
	} else {
		o.notifyProgress(DeploymentStep{
			Name:        "skip_ssh_setup",
			Description: "Container provider - skipping SSH setup...",
			Progress:    40,
		})
	}

	o.notifyProgress(DeploymentStep{
		Name:        "create_server",
		Description: fmt.Sprintf("Creating %s server...", client.DisplayName()),
		Progress:    50,
	})

	sanitizedName := util.SanitizeHostname(o.projectName)

	imageName := "ubuntu-22-04-x64"
	switch o.config.Provider {
	case "hetzner":
		imageName = "ubuntu-22.04"
	case "vultr":
		imageName = "1743"
	case "flyio":
		imageName = "ubuntu:22.04"
	}

	metadata := map[string]string{
		"managed_by": "lightfold",
		"project":    o.projectName,
	}

	if o.config.Provider == "flyio" {
		flyioConfig, err := o.config.GetFlyioConfig()
		if err == nil {
			if flyioConfig.OrganizationID != "" {
				metadata["organization_id"] = flyioConfig.OrganizationID
			}
			if flyioConfig.AppName != "" {
				metadata["app_name"] = flyioConfig.AppName
			}
		}
	}

	provisionConfig := providers.ProvisionConfig{
		Name:              fmt.Sprintf("%s-app", sanitizedName),
		Region:            region,
		Size:              size,
		Image:             imageName,
		SSHKeys:           []string{},
		UserData:          userData,
		Tags:              []string{"lightfold", "auto-provisioned", o.projectName},
		BackupsEnabled:    false,
		MonitoringEnabled: true,
		Metadata:          metadata,
	}

	// Add SSH key for SSH-based providers
	if uploadedKey != nil {
		provisionConfig.SSHKeys = []string{uploadedKey.ID}
	}

	server, err := client.Provision(ctx, provisionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to provision server: %w", err)
	}

	// Skip waiting for container providers - they don't have active servers until deployment
	if client.SupportsSSH() {
		o.notifyProgress(DeploymentStep{
			Name:        "wait_active",
			Description: fmt.Sprintf("Waiting for server %s to become active...", server.ID),
			Progress:    70,
		})

		server, err = client.WaitForActive(ctx, server.ID, 5*time.Minute)
		if err != nil {
			return nil, fmt.Errorf("failed waiting for server: %w", err)
		}
	} else {
		o.notifyProgress(DeploymentStep{
			Name:        "skip_wait",
			Description: "Container provider - skipping server wait...",
			Progress:    70,
		})
	}

	o.notifyProgress(DeploymentStep{
		Name:        "update_config",
		Description: "Updating configuration with server details...",
		Progress:    90,
	})

	if err := o.updateProviderConfigWithServerInfo(server); err != nil {
		return nil, fmt.Errorf("failed to update provider config: %w", err)
	}

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

	markerCheck := sshExecutor.Execute(fmt.Sprintf("test -f %s/%s && echo 'configured'", config.RemoteLightfoldDir, config.RemoteConfiguredMarker))
	isConfigured := markerCheck.ExitCode == 0 && strings.TrimSpace(markerCheck.Stdout) == "configured"

	executor := NewExecutor(sshExecutor, o.projectName, o.projectPath, &detection)

	executor.SetOutputCallback(func(line string) {
		if o.progressCallback != nil {
			o.progressCallback(DeploymentStep{
				Name:        "command_output",
				Description: line,
				Progress:    -1,
			})
		}
	})

	o.notifyProgress(DeploymentStep{
		Name:        "wait_cloud_init",
		Description: "Initializing server...",
		Progress:    15,
	})

	if err := executor.WaitForAptLock(30, 10*time.Second); err != nil {
		return nil, fmt.Errorf("failed to acquire apt lock: %w", err)
	}

	if !isConfigured {
		o.notifyProgress(DeploymentStep{
			Name:        "install_packages",
			Description: "Installing web server, system packages, and runtimes...",
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

	envVars := make(map[string]string)
	if o.config.Deploy != nil && o.config.Deploy.EnvVars != nil {
		envVars = o.config.Deploy.EnvVars
	}

	builderName := o.config.Builder
	if builderName == "" {
		builderName = "native"
	}

	builder, err := builders.GetBuilder(builderName)
	if err != nil {
		return nil, fmt.Errorf("failed to get builder %s: %w", builderName, err)
	}

	var buildResult *builders.BuildResult
	skipBuild := o.config.Deploy != nil && o.config.Deploy.SkipBuild
	if !skipBuild {
		o.notifyProgress(DeploymentStep{
			Name:        "build_release",
			Description: fmt.Sprintf("Building with %s...", builder.Name()),
			Progress:    60,
		})

		var err error
		buildResult, err = builder.Build(ctx, &builders.BuildOptions{
			ProjectPath: o.projectPath,
			Detection:   &detection,
			ReleasePath: releasePath,
			EnvVars:     envVars,
			SSHExecutor: sshExecutor,
		})
		if err != nil {
			return nil, fmt.Errorf("build failed: %w", err)
		}

		if !buildResult.Success {
			return nil, fmt.Errorf("build failed")
		}

		if buildResult.StartCommand != "" {
			executor.SetStartCommand(buildResult.StartCommand)
		} else if len(detection.RunPlan) > 0 {
			executor.SetStartCommand(detection.RunPlan[0])
		}

		if err := state.UpdateBuilder(o.targetName, builder.Name()); err != nil {
			fmt.Printf("Warning: failed to update builder in state: %v\n", err)
		}
	}

	o.notifyProgress(DeploymentStep{
		Name:        "write_env",
		Description: "Configuring environment variables...",
		Progress:    65,
	})

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

	if builder.NeedsNginx() {
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
	} else {
		o.notifyProgress(DeploymentStep{
			Name:        "skip_nginx",
			Description: fmt.Sprintf("Skipping nginx (handled by %s)...", builder.Name()),
			Progress:    75,
		})
	}

	o.notifyProgress(DeploymentStep{
		Name:        "deploy_app",
		Description: "Deploying application with health check...",
		Progress:    85,
	})

	if err := executor.DeployWithHealthCheck(releasePath, config.DefaultHealthCheckMaxRetries, config.DefaultHealthCheckRetryDelay); err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "cleanup",
		Description: "Cleaning up old releases...",
		Progress:    95,
	})

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Warning: failed to load config for cleanup: %v\n", err)
		cfg = &config.Config{KeepReleases: config.DefaultKeepReleases}
	}

	if err := executor.CleanupOldReleases(cfg.KeepReleases); err != nil {
		fmt.Printf("Warning: failed to cleanup old releases: %v\n", err)
	}

	o.notifyProgress(DeploymentStep{
		Name:        "complete",
		Description: "Configuration complete!",
		Progress:    100,
	})

	if !isConfigured {
		sshExecutor.Execute(fmt.Sprintf("sudo mkdir -p %s", config.RemoteLightfoldDir))
		markerResult := sshExecutor.Execute(fmt.Sprintf("echo 'configured' | sudo tee %s/%s > /dev/null", config.RemoteLightfoldDir, config.RemoteConfiguredMarker))
		if markerResult.Error != nil {
			return nil, fmt.Errorf("failed to write configured marker: %w", markerResult.Error)
		}
		if markerResult.ExitCode != 0 {
			return nil, fmt.Errorf("failed to write configured marker: command exited with code %d, stderr: %s", markerResult.ExitCode, markerResult.Stderr)
		}

		o.notifyProgress(DeploymentStep{
			Name:        "schedule_updates",
			Description: "Scheduling system updates and reboot...",
			Progress:    98,
		})
		updateCmd := fmt.Sprintf("nohup bash -c 'apt-get update && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -o Dpkg::Options::=\"--force-confdef\" -o Dpkg::Options::=\"--force-confold\" && shutdown -r +%d' > /var/log/lightfold-update.log 2>&1 &", config.DefaultRebootDelayMinutes)
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

// getProvisioningParams extracts provisioning parameters from provider-specific config
func (o *Orchestrator) getProvisioningParams() (region, size, sshKeyPath, username, sshKeyName string, err error) {
	switch o.config.Provider {
	case "digitalocean":
		doConfig, e := o.config.GetDigitalOceanConfig()
		if e != nil {
			err = fmt.Errorf("failed to get DigitalOcean config: %w", e)
			return
		}
		region = doConfig.Region
		size = doConfig.Size
		sshKeyPath = doConfig.SSHKey
		username = doConfig.Username
		sshKeyName = doConfig.SSHKeyName
	case "hetzner":
		hetznerConfig, e := o.config.GetHetznerConfig()
		if e != nil {
			err = fmt.Errorf("failed to get Hetzner config: %w", e)
			return
		}
		region = hetznerConfig.Location
		size = hetznerConfig.ServerType
		sshKeyPath = hetznerConfig.SSHKey
		username = hetznerConfig.Username
		sshKeyName = hetznerConfig.SSHKeyName
	case "vultr":
		vultrConfig, e := o.config.GetVultrConfig()
		if e != nil {
			err = fmt.Errorf("failed to get Vultr config: %w", e)
			return
		}
		region = vultrConfig.Region
		size = vultrConfig.Plan
		sshKeyPath = vultrConfig.SSHKey
		username = vultrConfig.Username
		sshKeyName = vultrConfig.SSHKeyName
	case "flyio":
		flyioConfig, e := o.config.GetFlyioConfig()
		if e != nil {
			err = fmt.Errorf("failed to get Fly.io config: %w", e)
			return
		}
		region = flyioConfig.Region
		size = flyioConfig.Size
		sshKeyPath = flyioConfig.SSHKey
		username = flyioConfig.Username
		sshKeyName = flyioConfig.SSHKeyName
	default:
		err = fmt.Errorf("unsupported provider for provisioning: %s", o.config.Provider)
	}
	return
}

func (o *Orchestrator) updateProviderConfigWithServerInfo(server *providers.Server) error {
	switch o.config.Provider {
	case "digitalocean":
		doConfig, err := o.config.GetDigitalOceanConfig()
		if err != nil {
			return err
		}
		doConfig.IP = server.PublicIPv4
		doConfig.DropletID = server.ID
		return o.config.SetProviderConfig("digitalocean", doConfig)
	case "hetzner":
		hetznerConfig, err := o.config.GetHetznerConfig()
		if err != nil {
			return err
		}
		hetznerConfig.IP = server.PublicIPv4
		hetznerConfig.ServerID = server.ID
		return o.config.SetProviderConfig("hetzner", hetznerConfig)
	case "vultr":
		vultrConfig, err := o.config.GetVultrConfig()
		if err != nil {
			return err
		}
		vultrConfig.IP = server.PublicIPv4
		vultrConfig.InstanceID = server.ID
		return o.config.SetProviderConfig("vultr", vultrConfig)
	case "flyio":
		flyioConfig, err := o.config.GetFlyioConfig()
		if err != nil {
			return err
		}
		flyioConfig.IP = server.PublicIPv4
		flyioConfig.MachineID = server.ID

		if orgID, exists := server.Metadata["organization_id"]; exists {
			flyioConfig.OrganizationID = orgID
		}
		if appName, exists := server.Metadata["app_name"]; exists {
			flyioConfig.AppName = appName
		}

		return o.config.SetProviderConfig("flyio", flyioConfig)
	default:
		return fmt.Errorf("unsupported provider: %s", o.config.Provider)
	}
}

func (o *Orchestrator) deployFlyio(ctx context.Context, token string) (*DeploymentResult, error) {
	result := &DeploymentResult{
		Steps: []DeploymentStep{},
	}

	o.notifyProgress(DeploymentStep{
		Name:        "start_flyio_deploy",
		Description: "Starting Fly.io container deployment...",
		Progress:    5,
	})

	// Get Fly.io config
	flyioConfig, err := o.config.GetFlyioConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Fly.io config: %w", err)
	}

	// Detect framework
	o.notifyProgress(DeploymentStep{
		Name:        "detect_framework",
		Description: "Analyzing application...",
		Progress:    10,
	})

	detection := detector.DetectFramework(o.projectPath)

	// Create Fly.io deployer
	deployer := NewFlyioDeployer(o.projectName, o.projectPath, o.targetName, &detection, flyioConfig, token)
	deployer.SetProgressCallback(o.progressCallback)

	// Get deployment options
	deployOpts := o.config.Deploy

	// Execute deployment
	if err := deployer.Deploy(ctx, deployOpts); err != nil {
		result.Success = false
		result.Error = err
		result.Message = fmt.Sprintf("Fly.io deployment failed: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = "Successfully deployed to Fly.io"

	return result, nil
}
