package sequential

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/ssh"
	"lightfold/pkg/util"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func CreateDigitalOceanFlow(projectName string) *FlowModel {
	steps := []Step{
		CreateIPStep("ip", "192.168.1.100"),
		CreateSSHKeyStep("ssh_key"),
		CreateUsernameStep("username", "root"),
	}

	flow := NewFlow("Configure DigitalOcean Deployment", steps)
	flow.SetProjectName(projectName)
	return flow
}

func CreateS3Flow() *FlowModel {
	steps := []Step{
		CreateS3BucketStep("bucket", "my-static-site"),
		CreateAWSRegionStep("region"),
		CreateAWSAccessKeyStep("access_key"),
		CreateAWSSecretKeyStep("secret_key"),
	}

	return NewFlow("Configure S3 Deployment", steps)
}

func RunDigitalOceanFlow(projectName string) (*config.DigitalOceanConfig, error) {
	flow := CreateDigitalOceanFlow(projectName)

	p := tea.NewProgram(flow)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(FlowModel)
	if final.Cancelled {
		return nil, fmt.Errorf("configuration cancelled")
	}

	if !final.Completed {
		return nil, fmt.Errorf("configuration not completed")
	}

	results := final.GetResults()

	sshKeyPath, sshKeyName := final.GetSSHKeyInfo("ssh_key")

	return &config.DigitalOceanConfig{
		IP:         results["ip"],
		SSHKey:     sshKeyPath,
		SSHKeyName: sshKeyName,
		Username:   results["username"],
	}, nil
}

func CreateProvisionDigitalOceanFlow(projectName string, hasExistingToken bool) *FlowModel {
	var steps []Step

	// Only ask for API token if we don't have one stored
	if !hasExistingToken {
		steps = append(steps, CreateAPITokenStep("api_token"))
	}

	steps = append(steps,
		CreateRegionStep("region"),
		CreateSizeStep("size"),
	)

	flow := NewFlow("Provision DigitalOcean Droplet", steps)
	flow.SetProjectName(projectName)
	return flow
}

func RunProvisionDigitalOceanFlow(projectName string) (*config.DigitalOceanConfig, error) {
	// Check if we already have a stored token
	tokens, err := config.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to load tokens: %w", err)
	}

	existingToken := tokens.GetToken("digitalocean")
	hasExistingToken := existingToken != ""

	flow := CreateProvisionDigitalOceanFlow(projectName, hasExistingToken)

	p := tea.NewProgram(flow)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(FlowModel)
	if final.Cancelled {
		return nil, fmt.Errorf("provisioning cancelled")
	}

	if !final.Completed {
		return nil, fmt.Errorf("provisioning not completed")
	}

	results := final.GetResults()

	// Store API token securely if a new one was provided
	if newToken, ok := results["api_token"]; ok && newToken != "" {
		tokens.SetToken("digitalocean", newToken)
		if err := tokens.SaveTokens(); err != nil {
			return nil, fmt.Errorf("failed to save API token: %w", err)
		}
	}

	// Generate SSH keypair for the provisioned droplet
	keyName := ssh.GetKeyName(projectName)

	// Check if key already exists, generate if not
	exists, err := ssh.KeyExists(keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to check SSH key existence: %w", err)
	}

	var keyPath string
	if !exists {
		keyPair, err := ssh.GenerateKeyPair(keyName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SSH key pair: %w", err)
		}
		keyPath = keyPair.PrivateKeyPath
	} else {
		// Key exists, get its path
		keysDir, err := ssh.GetKeysDirectory()
		if err != nil {
			return nil, fmt.Errorf("failed to get keys directory: %w", err)
		}
		keyPath = filepath.Join(keysDir, keyName)
	}

	// Extract the size ID from the display string (e.g., "s-1vcpu-512mb-10gb (512 MB RAM, 1 vCPU)")
	sizeStr := results["size"]
	sizeID := sizeStr
	if idx := strings.Index(sizeStr, " ("); idx > 0 {
		sizeID = sizeStr[:idx]
	}

	// Return config with SSH key path and provisioning parameters
	// IP will be filled in during actual provisioning in deploy command
	return &config.DigitalOceanConfig{
		IP:          "",                       // Will be filled by actual provisioning
		Username:    "deploy",                 // Standard user for provisioned droplets
		SSHKey:      keyPath,                  // Path to generated private key
		SSHKeyName:  keyName,                  // Name of the SSH key for uploading to DO
		Region:      results["region"],        // Store selected region
		Size:        sizeID,                   // Store selected size ID
		Provisioned: true,                     // Mark as provisioned
	}, nil
}

func RunS3Flow() (*config.S3Config, error) {
	flow := CreateS3Flow()

	p := tea.NewProgram(flow)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(FlowModel)
	if final.Cancelled {
		return nil, fmt.Errorf("configuration cancelled")
	}

	if !final.Completed {
		return nil, fmt.Errorf("configuration not completed")
	}

	results := final.GetResults()

	return &config.S3Config{
		Bucket:    results["bucket"],
		Region:    results["region"],
		AccessKey: results["access_key"],
		SecretKey: results["secret_key"],
	}, nil
}

func GetProjectNameFromPath(projectPath string) string {
	projectName := util.GetTargetName(projectPath)
	if projectName == "." || projectName == "/" {
		parent := filepath.Dir(projectPath)
		projectName = util.GetTargetName(parent)
	}
	return projectName
}
