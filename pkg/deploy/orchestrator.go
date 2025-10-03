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
	"os"
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
	Success   bool
	Server    *providers.Server
	Message   string
	Error     error
	Steps     []DeploymentStep
}

// ProgressCallback is called for each deployment step
type ProgressCallback func(step DeploymentStep)

// Orchestrator manages the deployment process
type Orchestrator struct {
	config           config.ProjectConfig
	projectPath      string
	projectName      string
	tokens           *config.TokenConfig
	progressCallback ProgressCallback
}

// NewOrchestrator creates a new deployment orchestrator
func NewOrchestrator(projectConfig config.ProjectConfig, projectPath, projectName string) (*Orchestrator, error) {
	tokens, err := config.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to load tokens: %w", err)
	}

	return &Orchestrator{
		config:      projectConfig,
		projectPath: projectPath,
		projectName: projectName,
		tokens:      tokens,
	}, nil
}

// SetProgressCallback sets the callback for progress updates
func (o *Orchestrator) SetProgressCallback(callback ProgressCallback) {
	o.progressCallback = callback
}

// Deploy executes the deployment
func (o *Orchestrator) Deploy(ctx context.Context) (*DeploymentResult, error) {
	// Validate provider is registered
	if !providers.IsRegistered(o.config.Provider) {
		return nil, fmt.Errorf("unknown provider: %s", o.config.Provider)
	}

	// Check if server is already provisioned to prevent duplicates
	if err := o.checkExistingServer(); err != nil {
		return nil, err
	}

	// Get provider-specific config
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

	// Step 3: Load SSH public key
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

	// Step 4: Upload SSH key to provider
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

	// Step 5: Generate cloud-init user data
	o.notifyProgress(DeploymentStep{
		Name:        "generate_cloudinit",
		Description: "Generating cloud-init configuration...",
		Progress:    40,
	})

	userData, err := cloudinit.GenerateWebAppUserData(username, publicKey, o.projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud-init: %w", err)
	}

	// Step 6: Provision server
	o.notifyProgress(DeploymentStep{
		Name:        "create_server",
		Description: fmt.Sprintf("Creating %s server...", client.DisplayName()),
		Progress:    50,
	})

	provisionConfig := providers.ProvisionConfig{
		Name:              fmt.Sprintf("%s-app", o.projectName),
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

	// Step 7: Wait for server to become active
	o.notifyProgress(DeploymentStep{
		Name:        "wait_active",
		Description: fmt.Sprintf("Waiting for server %s to become active...", server.ID),
		Progress:    70,
	})

	server, err = client.WaitForActive(ctx, server.ID, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for server: %w", err)
	}

	// Step 8: Update configuration with server IP and ID
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
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetProject(o.projectPath, o.config); err != nil {
		return nil, fmt.Errorf("failed to update project config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// Step 9: Complete
	o.notifyProgress(DeploymentStep{
		Name:        "complete",
		Description: "Provisioning complete!",
		Progress:    100,
	})

	result.Success = true
	result.Server = server
	result.Message = fmt.Sprintf("Successfully provisioned server at %s", server.PublicIPv4)

	return result, nil
}

func (o *Orchestrator) deployS3(ctx context.Context) (*DeploymentResult, error) {
	// TODO: Implement S3 deployment
	return &DeploymentResult{
		Success: false,
		Message: "S3 deployment not yet implemented",
		Error:   fmt.Errorf("S3 deployment not yet implemented"),
	}, nil
}

// ConfigureServer performs the full VM configuration (package install, build, systemd, nginx, deploy)
// This is used by both Deploy and Configure commands to avoid duplication
func (o *Orchestrator) ConfigureServer(ctx context.Context, providerCfg config.ProviderConfig) (*DeploymentResult, error) {
	result := &DeploymentResult{
		Steps: []DeploymentStep{},
	}

	// Step 1: Run detection to get build/run plans
	o.notifyProgress(DeploymentStep{
		Name:        "detect_framework",
		Description: "Detecting framework configuration...",
		Progress:    5,
	})

	detection := detector.DetectFramework(o.projectPath)

	// Step 2: Connect to server via SSH
	o.notifyProgress(DeploymentStep{
		Name:        "connect_ssh",
		Description: fmt.Sprintf("Connecting to server at %s...", providerCfg.GetIP()),
		Progress:    10,
	})

	sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
	defer sshExecutor.Disconnect()

	executor := NewExecutor(sshExecutor, o.projectName, o.projectPath, &detection)

	// Step 3: Install base packages
	o.notifyProgress(DeploymentStep{
		Name:        "install_packages",
		Description: "Installing system packages and runtimes...",
		Progress:    20,
	})

	if err := executor.InstallBasePackages(); err != nil {
		return nil, fmt.Errorf("failed to install packages: %w", err)
	}

	// Step 4: Setup directory structure
	o.notifyProgress(DeploymentStep{
		Name:        "setup_directories",
		Description: "Setting up deployment directories...",
		Progress:    30,
	})

	if err := executor.SetupDirectoryStructure(); err != nil {
		return nil, fmt.Errorf("failed to setup directories: %w", err)
	}

	// Step 5: Create and upload release tarball
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

	skipBuild := o.config.Deploy != nil && o.config.Deploy.SkipBuild
	if !skipBuild {
		o.notifyProgress(DeploymentStep{
			Name:        "build_release",
			Description: "Building application...",
			Progress:    60,
		})

		if err := executor.BuildRelease(releasePath); err != nil {
			return nil, fmt.Errorf("failed to build release: %w", err)
		}
	}

	// Step 7: Write environment variables
	o.notifyProgress(DeploymentStep{
		Name:        "write_env",
		Description: "Configuring environment variables...",
		Progress:    65,
	})

	envVars := make(map[string]string)
	if o.config.Deploy != nil && o.config.Deploy.EnvVars != nil {
		envVars = o.config.Deploy.EnvVars
	}

	if err := executor.WriteEnvironmentFile(envVars); err != nil {
		return nil, fmt.Errorf("failed to write environment: %w", err)
	}

	// Step 8: Configure systemd service
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

	// Step 9: Configure nginx
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

	// Step 10: Deploy with health check
	o.notifyProgress(DeploymentStep{
		Name:        "deploy_app",
		Description: "Deploying application with health check...",
		Progress:    85,
	})

	if err := executor.DeployWithHealthCheck(releasePath, 5, 3*time.Second); err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	// Step 11: Cleanup old releases
	o.notifyProgress(DeploymentStep{
		Name:        "cleanup",
		Description: "Cleaning up old releases...",
		Progress:    95,
	})

	if err := executor.CleanupOldReleases(5); err != nil {
		fmt.Printf("Warning: failed to cleanup old releases: %v\n", err)
	}

	// Step 12: Complete
	o.notifyProgress(DeploymentStep{
		Name:        "complete",
		Description: "Configuration complete!",
		Progress:    100,
	})

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
