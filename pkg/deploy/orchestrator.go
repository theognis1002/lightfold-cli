package deploy

import (
	"context"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/providers"
	"lightfold/pkg/providers/cloudinit"
	"lightfold/pkg/providers/digitalocean"
	"lightfold/pkg/ssh"
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
	// Check if server is already provisioned to prevent duplicates
	if err := o.checkExistingServer(); err != nil {
		return nil, err
	}

	// Determine deployment type and execute
	switch o.config.Target {
	case "digitalocean":
		return o.deployDigitalOcean(ctx)
	case "s3":
		return o.deployS3(ctx)
	default:
		return nil, fmt.Errorf("unsupported deployment target: %s", o.config.Target)
	}
}

// checkExistingServer verifies if a server is already provisioned to prevent duplicate provisioning
func (o *Orchestrator) checkExistingServer() error {
	switch o.config.Target {
	case "digitalocean":
		if o.config.DigitalOcean != nil && o.config.DigitalOcean.Provisioned {
			// If we have both a droplet ID and IP, server already exists
			if o.config.DigitalOcean.DropletID != "" && o.config.DigitalOcean.IP != "" {
				return fmt.Errorf("server already provisioned (ID: %s, IP: %s). Use a different command to redeploy or destroy the existing server first",
					o.config.DigitalOcean.DropletID,
					o.config.DigitalOcean.IP)
			}
		}
	case "s3":
		// S3 deployments don't provision servers, so no check needed
		return nil
	}
	return nil
}

// deployDigitalOcean handles DigitalOcean deployment
func (o *Orchestrator) deployDigitalOcean(ctx context.Context) (*DeploymentResult, error) {
	result := &DeploymentResult{
		Steps: []DeploymentStep{},
	}

	doConfig := o.config.DigitalOcean
	if doConfig == nil {
		return nil, fmt.Errorf("DigitalOcean configuration is missing")
	}

	// If this is a provisioned droplet and no IP exists, we need to provision it
	if doConfig.Provisioned && doConfig.IP == "" {
		return o.provisionDigitalOceanDroplet(ctx, doConfig)
	}

	// Otherwise, this is BYOS - just verify connection
	o.notifyProgress(DeploymentStep{
		Name:        "verify_connection",
		Description: "Verifying server connection...",
		Progress:    10,
	})

	// For BYOS, we would SSH into the server and deploy
	// This will be implemented as needed
	result.Success = true
	result.Message = fmt.Sprintf("Connected to server at %s", doConfig.IP)

	return result, nil
}

// provisionDigitalOceanDroplet provisions a new DigitalOcean droplet
func (o *Orchestrator) provisionDigitalOceanDroplet(ctx context.Context, doConfig *config.DigitalOceanConfig) (*DeploymentResult, error) {
	result := &DeploymentResult{
		Steps: []DeploymentStep{},
	}

	// Step 1: Initialize DigitalOcean client
	o.notifyProgress(DeploymentStep{
		Name:        "init_client",
		Description: "Initializing DigitalOcean client...",
		Progress:    5,
	})

	token := o.tokens.GetDigitalOceanToken()
	if token == "" {
		return nil, fmt.Errorf("DigitalOcean API token not found")
	}

	client := digitalocean.NewClient(token)

	// Step 2: Validate credentials
	o.notifyProgress(DeploymentStep{
		Name:        "validate_credentials",
		Description: "Validating API credentials...",
		Progress:    10,
	})

	if err := client.ValidateCredentials(ctx); err != nil {
		return nil, fmt.Errorf("failed to validate credentials: %w", err)
	}

	// Step 3: Load SSH public key
	o.notifyProgress(DeploymentStep{
		Name:        "load_ssh_key",
		Description: "Loading SSH key...",
		Progress:    20,
	})

	publicKeyPath := doConfig.SSHKey + ".pub"
	publicKey, err := ssh.LoadPublicKey(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}

	// Step 4: Upload SSH key to DigitalOcean
	o.notifyProgress(DeploymentStep{
		Name:        "upload_ssh_key",
		Description: "Uploading SSH key to DigitalOcean...",
		Progress:    30,
	})

	sshKeyName := doConfig.SSHKeyName
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

	userData, err := cloudinit.GenerateWebAppUserData(doConfig.Username, publicKey, o.projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud-init: %w", err)
	}

	// Step 6: Provision droplet
	o.notifyProgress(DeploymentStep{
		Name:        "create_droplet",
		Description: "Creating DigitalOcean droplet...",
		Progress:    50,
	})

	provisionConfig := providers.ProvisionConfig{
		Name:     fmt.Sprintf("%s-app", o.projectName),
		Region:   doConfig.Region,
		Size:     doConfig.Size,
		Image:    "ubuntu-22-04-x64",
		SSHKeys:  []string{uploadedKey.ID},
		UserData: userData,
		Tags:     []string{"lightfold", "auto-provisioned", o.projectName},
		BackupsEnabled:    false,
		MonitoringEnabled: true,
	}

	server, err := client.Provision(ctx, provisionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to provision droplet: %w", err)
	}

	// Step 7: Wait for droplet to become active
	o.notifyProgress(DeploymentStep{
		Name:        "wait_active",
		Description: fmt.Sprintf("Waiting for droplet %s to become active...", server.ID),
		Progress:    70,
	})

	server, err = client.WaitForActive(ctx, server.ID, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for droplet: %w", err)
	}

	// Step 8: Update configuration with server IP
	o.notifyProgress(DeploymentStep{
		Name:        "update_config",
		Description: "Updating configuration with server details...",
		Progress:    90,
	})

	doConfig.IP = server.PublicIPv4
	doConfig.DropletID = server.ID

	// Save updated configuration
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
		Description: "Deployment complete!",
		Progress:    100,
	})

	result.Success = true
	result.Server = server
	result.Message = fmt.Sprintf("Successfully provisioned droplet at %s", server.PublicIPv4)

	return result, nil
}

// deployS3 handles S3 deployment (static sites)
func (o *Orchestrator) deployS3(ctx context.Context) (*DeploymentResult, error) {
	// TODO: Implement S3 deployment
	return &DeploymentResult{
		Success: false,
		Message: "S3 deployment not yet implemented",
		Error:   fmt.Errorf("S3 deployment not yet implemented"),
	}, nil
}

// notifyProgress sends progress updates to the callback
func (o *Orchestrator) notifyProgress(step DeploymentStep) {
	if o.progressCallback != nil {
		o.progressCallback(step)
	}
}
