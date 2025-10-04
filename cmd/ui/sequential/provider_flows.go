package sequential

import (
	"fmt"
	"lightfold/pkg/config"

	tea "github.com/charmbracelet/bubbletea"
)

func CreateProviderSelectionStep(id string) Step {
	return NewStep(id, "Select Cloud Provider").
		Description("Choose how you want to deploy your application").
		Type(StepTypeSelect).
		Options(
			"digitalocean",
			"byos",
			"hetzner",
		).
		OptionDescriptions(
			"Auto-provision servers on DigitalOcean",
			"Use an existing server with SSH access",
			"Auto-provision servers on Hetzner Cloud",
		).
		Required().
		Build()
}

func RunProviderSelectionWithConfigFlow(projectName string) (provider string, cfg interface{}, err error) {
	steps := []Step{
		CreateProviderSelectionStep("provider"),
	}

	flow := NewFlow("Configure Deployment", steps)
	flow.SetProjectName(projectName)

	p := tea.NewProgram(flow)
	finalModel, err := p.Run()
	if err != nil {
		return "", nil, fmt.Errorf("provider selection failed: %w", err)
	}

	final := finalModel.(FlowModel)
	if final.Cancelled {
		return "", nil, fmt.Errorf("provider selection cancelled")
	}

	if !final.Completed {
		return "", nil, fmt.Errorf("provider selection not completed")
	}

	results := final.GetResults()
	selectedProvider := results["provider"]

	switch selectedProvider {
	case "digitalocean":
		doConfig, err := RunProvisionDigitalOceanFlow(projectName)
		if err != nil {
			return "", nil, fmt.Errorf("DigitalOcean configuration failed: %w", err)
		}
		return "digitalocean", doConfig, nil

	case "byos":
		byosConfig, err := RunBYOSConfigurationFlow(projectName)
		if err != nil {
			return "", nil, fmt.Errorf("BYOS configuration failed: %w", err)
		}
		return "byos", byosConfig, nil

	case "hetzner":
		hetznerConfig, err := RunProvisionHetznerFlow(projectName)
		if err != nil {
			return "", nil, fmt.Errorf("Hetzner Cloud configuration failed: %w", err)
		}
		return "hetzner", hetznerConfig, nil

	default:
		return "", nil, fmt.Errorf("unsupported provider: %s", selectedProvider)
	}
}

func CreateBYOSConfigurationFlow(projectName string) *FlowModel {
	steps := []Step{
		CreateIPStep("ip", "192.168.1.100"),
		CreateSSHKeyStep("ssh_key"),
		CreateUsernameStep("username", "root"),
	}

	flow := NewFlow("Configure BYOS Deployment", steps)
	flow.SetProjectName(projectName)
	return flow
}

func RunBYOSConfigurationFlow(projectName string) (*config.DigitalOceanConfig, error) {
	flow := CreateBYOSConfigurationFlow(projectName)

	p := tea.NewProgram(flow)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(FlowModel)
	if final.Cancelled {
		return nil, fmt.Errorf("BYOS configuration cancelled")
	}

	if !final.Completed {
		return nil, fmt.Errorf("BYOS configuration not completed")
	}

	results := final.GetResults()
	sshKeyPath, sshKeyName := final.GetSSHKeyInfo("ssh_key")

	return &config.DigitalOceanConfig{
		IP:          results["ip"],
		SSHKey:      sshKeyPath,
		SSHKeyName:  sshKeyName,
		Username:    results["username"],
		Provisioned: false,
	}, nil
}
