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

	if newToken, ok := results["api_token"]; ok && newToken != "" {
		tokens.SetToken("digitalocean", newToken)
		if err := tokens.SaveTokens(); err != nil {
			return nil, fmt.Errorf("failed to save API token: %w", err)
		}
	}

	keyName := ssh.GetKeyName(projectName)

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
		keysDir, err := ssh.GetKeysDirectory()
		if err != nil {
			return nil, fmt.Errorf("failed to get keys directory: %w", err)
		}
		keyPath = filepath.Join(keysDir, keyName)
	}

	sizeStr := results["size"]
	sizeID := sizeStr
	if idx := strings.Index(sizeStr, " ("); idx > 0 {
		sizeID = sizeStr[:idx]
	}

	return &config.DigitalOceanConfig{
		IP:          "",
		Username:    "deploy",
		SSHKey:      keyPath,
		SSHKeyName:  keyName,
		Region:      results["region"],
		Size:        sizeID,
		Provisioned: true,
	}, nil
}

func CreateProvisionHetznerFlow(projectName string, hasExistingToken bool) *FlowModel {
	var steps []Step

	if !hasExistingToken {
		steps = append(steps, CreateHetznerAPITokenStep("api_token"))
	}

	steps = append(steps,
		CreateHetznerLocationStep("location"),
		CreateHetznerServerTypeStep("server_type"),
	)

	flow := NewFlow("Provision Hetzner Cloud Server", steps)
	flow.SetProjectName(projectName)
	return flow
}

func RunProvisionHetznerFlow(projectName string) (*config.HetznerConfig, error) {
	tokens, err := config.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to load tokens: %w", err)
	}

	existingToken := tokens.GetToken("hetzner")
	hasExistingToken := existingToken != ""

	var activeToken string

	if !hasExistingToken {
		tokenFlow := NewFlow("Configure Hetzner Cloud", []Step{
			CreateHetznerAPITokenStep("api_token"),
		})
		tokenFlow.SetProjectName(projectName)

		p := tea.NewProgram(tokenFlow)
		tokenModel, err := p.Run()
		if err != nil {
			return nil, err
		}

		tokenFinal := tokenModel.(FlowModel)
		if tokenFinal.Cancelled {
			return nil, fmt.Errorf("provisioning cancelled")
		}

		tokenResults := tokenFinal.GetResults()
		activeToken = tokenResults["api_token"]

		tokens.SetToken("hetzner", activeToken)
		if err := tokens.SaveTokens(); err != nil {
			return nil, fmt.Errorf("failed to save API token: %w", err)
		}
	} else {
		activeToken = existingToken
	}

	dynamicSteps := []Step{
		CreateHetznerLocationStepDynamic("location", activeToken),
		CreateHetznerServerTypeStepDynamic("server_type", activeToken, ""),
	}

	flow := NewFlow("Provision Hetzner Cloud Server", dynamicSteps)
	flow.SetProjectName(projectName)

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

	keyName := ssh.GetKeyName(projectName)

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
		keysDir, err := ssh.GetKeysDirectory()
		if err != nil {
			return nil, fmt.Errorf("failed to get keys directory: %w", err)
		}
		keyPath = filepath.Join(keysDir, keyName)
	}

	serverTypeStr := results["server_type"]
	serverTypeID := serverTypeStr
	if idx := strings.Index(serverTypeStr, " ("); idx > 0 {
		serverTypeID = serverTypeStr[:idx]
	}

	return &config.HetznerConfig{
		IP:          "",
		Username:    "deploy",
		SSHKey:      keyPath,
		SSHKeyName:  keyName,
		Location:    results["location"],
		ServerType:  serverTypeID,
		Provisioned: true,
	}, nil
}

func CreateProvisionVultrFlow(projectName string, hasExistingToken bool) *FlowModel {
	var steps []Step

	if !hasExistingToken {
		steps = append(steps, CreateVultrAPITokenStep("api_token"))
	}

	steps = append(steps,
		CreateVultrRegionStep("region"),
		CreateVultrPlanStep("plan"),
	)

	flow := NewFlow("Provision Vultr Instance", steps)
	flow.SetProjectName(projectName)
	return flow
}

func RunProvisionVultrFlow(projectName string) (*config.VultrConfig, error) {
	tokens, err := config.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to load tokens: %w", err)
	}

	existingToken := tokens.GetToken("vultr")
	hasExistingToken := existingToken != ""

	var activeToken string

	if !hasExistingToken {
		tokenFlow := NewFlow("Configure Vultr", []Step{
			CreateVultrAPITokenStep("api_token"),
		})
		tokenFlow.SetProjectName(projectName)

		p := tea.NewProgram(tokenFlow)
		tokenModel, err := p.Run()
		if err != nil {
			return nil, err
		}

		tokenFinal := tokenModel.(FlowModel)
		if tokenFinal.Cancelled {
			return nil, fmt.Errorf("provisioning cancelled")
		}

		tokenResults := tokenFinal.GetResults()
		activeToken = tokenResults["api_token"]

		tokens.SetToken("vultr", activeToken)
		if err := tokens.SaveTokens(); err != nil {
			return nil, fmt.Errorf("failed to save API token: %w", err)
		}
	} else {
		activeToken = existingToken
	}

	dynamicSteps := []Step{
		CreateVultrRegionStepDynamic("region", activeToken),
		CreateVultrPlanStepDynamic("plan", activeToken, ""),
	}

	flow := NewFlow("Provision Vultr Instance", dynamicSteps)
	flow.SetProjectName(projectName)

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

	keyName := ssh.GetKeyName(projectName)

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
		keysDir, err := ssh.GetKeysDirectory()
		if err != nil {
			return nil, fmt.Errorf("failed to get keys directory: %w", err)
		}
		keyPath = filepath.Join(keysDir, keyName)
	}

	planStr := results["plan"]
	planID := planStr
	if idx := strings.Index(planStr, " ("); idx > 0 {
		planID = planStr[:idx]
	}

	return &config.VultrConfig{
		IP:          "",
		Username:    "deploy",
		SSHKey:      keyPath,
		SSHKeyName:  keyName,
		Region:      results["region"],
		Plan:        planID,
		Provisioned: true,
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

func RunProvisionFlyioFlow(projectName string) (*config.FlyioConfig, error) {
	flow := CreateProvisionFlyioFlow(projectName)

	p := tea.NewProgram(flow)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(FlowModel)
	if final.Cancelled {
		return nil, fmt.Errorf("Fly.io configuration cancelled")
	}

	if !final.Completed {
		return nil, fmt.Errorf("Fly.io configuration not completed")
	}

	results := final.GetResults()

	if token, ok := results["api_token"]; ok && token != "" {
		tokens, _ := config.LoadTokens()
		tokens.SetToken("flyio", token)
		tokens.SaveTokens()
	}

	keyName := ssh.GetKeyName(projectName)
	keyPath := generateSSHKeyIfNeeded(keyName)

	sizeStr, hasSize := results["size"]
	if !hasSize || sizeStr == "" {
		sizeStr = "shared-cpu-1x"
	}
	sizeID := extractID(sizeStr)
	if sizeID == "" {
		sizeID = "shared-cpu-1x"
	}

	regionStr := results["region"]
	if regionStr == "" {
		regionStr = "sjc"
	}

	return &config.FlyioConfig{
		IP:          "",
		Username:    "root",
		SSHKey:      keyPath,
		SSHKeyName:  keyName,
		Region:      regionStr,
		Size:        sizeID,
		Provisioned: true,
		AppName:     projectName,
	}, nil
}

func CreateProvisionFlyioFlow(projectName string) *FlowModel {
	tokens, _ := config.LoadTokens()
	existingToken := tokens.GetToken("flyio")
	hasToken := existingToken != ""

	steps := []Step{}

	if !hasToken {
		steps = append(steps, CreateFlyioAPITokenStep("api_token"))
	}

	activeToken := existingToken
	if hasToken {
		steps = append(steps,
			CreateFlyioRegionStepDynamic("region", activeToken),
			CreateFlyioSizeStepDynamic("size", activeToken, ""),
		)
	}

	flow := NewFlow("Configure Fly.io Deployment", steps)
	flow.SetProjectName(projectName)
	return flow
}

func GetProjectNameFromPath(projectPath string) string {
	projectName := util.GetTargetName(projectPath)
	if projectName == "." || projectName == "/" {
		parent := filepath.Dir(projectPath)
		projectName = util.GetTargetName(parent)
	}
	return projectName
}
