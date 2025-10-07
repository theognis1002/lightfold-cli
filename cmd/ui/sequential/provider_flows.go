package sequential

import (
	"fmt"
	"lightfold/pkg/config"
	sshpkg "lightfold/pkg/ssh"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func CreateProviderSelectionStep(id string) Step {
	return NewStep(id, "Cloud Provider").
		Type(StepTypeSelect).
		Options(
			"digitalocean",
			"vultr",
			"hetzner",
			"byos",
		).
		OptionLabels(
			"DigitalOcean",
			"Vultr",
			"Hetzner",
			"BYOS",
		).
		OptionDescriptions(
			"",
			"",
			"",
			"Bring your own server",
		).
		Required().
		Build()
}

func RunProviderSelectionWithConfigFlow(projectName string) (provider string, cfg interface{}, err error) {
	// Create a unified flow with provider selection + provider config steps
	providerStep := CreateProviderSelectionStep("provider")
	steps := []Step{providerStep}

	flow := NewFlow("Configure Deployment", steps)
	flow.SetProjectName(projectName)

	// Run the dynamic flow that adds steps based on provider selection
	p := tea.NewProgram(&DynamicProviderFlow{
		FlowModel:   flow,
		ProjectName: projectName,
	})

	finalModel, err := p.Run()
	if err != nil {
		return "", nil, fmt.Errorf("configuration failed: %w", err)
	}

	final, ok := finalModel.(*DynamicProviderFlow)
	if !ok {
		// Fallback to regular flow model
		regularFinal := finalModel.(FlowModel)
		if regularFinal.Cancelled {
			return "", nil, fmt.Errorf("configuration cancelled")
		}
		return "", nil, fmt.Errorf("configuration not completed")
	}

	if final.Cancelled {
		return "", nil, fmt.Errorf("configuration cancelled")
	}

	if !final.Completed {
		return "", nil, fmt.Errorf("configuration not completed")
	}

	results := final.GetResults()
	selectedProvider := results["provider"]

	// Build config based on provider
	switch selectedProvider {
	case "digitalocean":
		doConfig := buildDigitalOceanConfig(results, final, projectName)
		return "digitalocean", doConfig, nil

	case "byos":
		byosConfig := buildBYOSConfig(results, final)
		return "byos", byosConfig, nil

	case "hetzner":
		hetznerConfig := buildHetznerConfig(results, final, projectName)
		return "hetzner", hetznerConfig, nil

	case "vultr":
		vultrConfig := buildVultrConfig(results, final, projectName)
		return "vultr", vultrConfig, nil

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

// DynamicProviderFlow wraps FlowModel and dynamically adds steps based on provider selection
type DynamicProviderFlow struct {
	*FlowModel
	ProjectName        string
	ProviderSelected   bool
	NeedsDynamicSteps  bool
	ProviderForDynamic string
	TokenStepIndex     int
}

func (m *DynamicProviderFlow) Init() tea.Cmd {
	return m.FlowModel.Init()
}

func (m *DynamicProviderFlow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Intercept Enter key on provider selection step BEFORE delegating to FlowModel
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		// Check if we're on provider selection and haven't added provider steps yet
		if !m.ProviderSelected && m.CurrentStep == 0 {
			currentStep := m.getCurrentStep()
			// For Select steps, get the value from the cursor position
			if currentStep.ID == "provider" && currentStep.Type == StepTypeSelect {
				if currentStep.Cursor >= 0 && currentStep.Cursor < len(currentStep.Options) {
					selectedProvider := currentStep.Options[currentStep.Cursor]
					// Provider was selected, add appropriate steps BEFORE processing Enter
					if err := m.addProviderSteps(selectedProvider); err == nil {
						m.ProviderSelected = true
						// Store the token step index if we need to add dynamic steps later
						if m.NeedsDynamicSteps {
							m.TokenStepIndex = 1 // The token step is right after provider selection
						}
						// Reset Completed flag to ensure flow continues
						m.Completed = false
					}
				}
			}
		}

		// Check if we're on token step and need to add dynamic steps
		if m.NeedsDynamicSteps && m.CurrentStep == m.TokenStepIndex {
			currentStep := m.getCurrentStep()
			if currentStep.Value != "" && currentStep.ID == "api_token" {
				token := currentStep.Value
				// Save the token first
				tokens, _ := config.LoadTokens()
				tokens.SetToken(m.ProviderForDynamic, token)
				tokens.SaveTokens()

				// Add dynamic steps based on provider
				var newSteps []Step
				switch m.ProviderForDynamic {
				case "hetzner":
					newSteps = []Step{
						CreateHetznerLocationStepDynamic("location", token),
						CreateHetznerServerTypeStepDynamic("server_type", token, ""),
					}
				case "vultr":
					newSteps = []Step{
						CreateVultrRegionStepDynamic("region", token),
						CreateVultrPlanStepDynamic("plan", token, ""),
					}
				}

				// Add new steps
				m.Steps = append(m.Steps, newSteps...)
				for i := len(m.StepStates); i < len(m.Steps); i++ {
					m.StepStates[i] = m.Steps[i]
				}

				m.NeedsDynamicSteps = false
				m.Completed = false
			}
		}
	}

	// Delegate to FlowModel's update
	updatedModel, cmd := m.FlowModel.Update(msg)
	if flowModel, ok := updatedModel.(FlowModel); ok {
		*m.FlowModel = flowModel
	}

	return m, cmd
}

func (m *DynamicProviderFlow) View() string {
	return m.FlowModel.View()
}

func (m *DynamicProviderFlow) addProviderSteps(provider string) error {
	var newSteps []Step

	tokens, err := config.LoadTokens()
	if err != nil {
		return err
	}

	switch provider {
	case "digitalocean":
		hasToken := tokens.GetToken("digitalocean") != ""
		if !hasToken {
			newSteps = append(newSteps, CreateAPITokenStep("api_token"))
		}
		newSteps = append(newSteps,
			CreateRegionStep("region"),
			CreateSizeStep("size"),
		)

	case "byos":
		newSteps = []Step{
			CreateIPStep("ip", "192.168.1.100"),
			CreateSSHKeyStep("ssh_key"),
			CreateUsernameStep("username", "root"),
		}

	case "hetzner":
		existingToken := tokens.GetToken("hetzner")
		hasToken := existingToken != ""

		if !hasToken {
			newSteps = append(newSteps, CreateHetznerAPITokenStep("api_token"))
		}

		// For Hetzner, we need dynamic steps that require API token
		// We'll add them after the token is collected or use existing token
		activeToken := existingToken
		if !hasToken {
			// Token will be collected in the next step, so we need to defer adding location/server type
			m.NeedsDynamicSteps = true
			m.ProviderForDynamic = "hetzner"
		} else {
			// We have a token, add dynamic steps now
			newSteps = append(newSteps,
				CreateHetznerLocationStepDynamic("location", activeToken),
				CreateHetznerServerTypeStepDynamic("server_type", activeToken, ""),
			)
		}

	case "vultr":
		existingToken := tokens.GetToken("vultr")
		hasToken := existingToken != ""

		if !hasToken {
			newSteps = append(newSteps, CreateVultrAPITokenStep("api_token"))
		}

		// For Vultr, we need dynamic steps that require API token
		activeToken := existingToken
		if !hasToken {
			// Token will be collected in the next step, so we need to defer adding region/plan
			m.NeedsDynamicSteps = true
			m.ProviderForDynamic = "vultr"
		} else {
			// We have a token, add dynamic steps now
			newSteps = append(newSteps,
				CreateVultrRegionStepDynamic("region", activeToken),
				CreateVultrPlanStepDynamic("plan", activeToken, ""),
			)
		}

	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	// Add new steps to the flow
	m.Steps = append(m.Steps, newSteps...)

	// Update step states for new steps
	for i := len(m.StepStates); i < len(m.Steps); i++ {
		m.StepStates[i] = m.Steps[i]
		if m.Steps[i].Type == StepTypeSSHKey {
			m.SSHHandlers[i] = NewSSHKeyHandler(m.ProjectName)
		}
	}

	return nil
}

// Helper functions to build provider configs from results
func buildDigitalOceanConfig(results map[string]string, flow *DynamicProviderFlow, projectName string) *config.DigitalOceanConfig {
	// Save token if provided
	if token, ok := results["api_token"]; ok && token != "" {
		tokens, _ := config.LoadTokens()
		tokens.SetToken("digitalocean", token)
		tokens.SaveTokens()
	}

	// Generate SSH key
	keyName := sshpkg.GetKeyName(projectName)
	keyPath := generateSSHKeyIfNeeded(keyName)

	// Extract size ID from "size_id (description)" format
	sizeStr, hasSize := results["size"]
	if !hasSize || sizeStr == "" {
		// Default to smallest size if not provided
		sizeStr = "s-1vcpu-512mb-10gb"
	}
	sizeID := extractID(sizeStr)

	// Validate that we have a non-empty size ID
	if sizeID == "" {
		sizeID = "s-1vcpu-512mb-10gb"
	}

	regionStr := results["region"]
	if regionStr == "" {
		regionStr = "nyc1" // Default region
	}

	return &config.DigitalOceanConfig{
		IP:          "",
		Username:    "deploy",
		SSHKey:      keyPath,
		SSHKeyName:  keyName,
		Region:      regionStr,
		Size:        sizeID,
		Provisioned: true,
	}
}

func buildBYOSConfig(results map[string]string, flow *DynamicProviderFlow) *config.DigitalOceanConfig {
	sshKeyPath, sshKeyName := flow.GetSSHKeyInfo("ssh_key")

	return &config.DigitalOceanConfig{
		IP:          results["ip"],
		SSHKey:      sshKeyPath,
		SSHKeyName:  sshKeyName,
		Username:    results["username"],
		Provisioned: false,
	}
}

func buildHetznerConfig(results map[string]string, flow *DynamicProviderFlow, projectName string) *config.HetznerConfig {
	// Token should already be saved by the dynamic step handler, but double-check
	if token, ok := results["api_token"]; ok && token != "" {
		tokens, _ := config.LoadTokens()
		if !tokens.HasToken("hetzner") {
			tokens.SetToken("hetzner", token)
			tokens.SaveTokens()
		}
	}

	// Generate SSH key
	keyName := sshpkg.GetKeyName(projectName)
	keyPath := generateSSHKeyIfNeeded(keyName)

	// Extract server type ID
	serverTypeStr, hasServerType := results["server_type"]
	if !hasServerType || serverTypeStr == "" {
		// This should not happen, but handle gracefully
		serverTypeStr = "cx11" // Default fallback
	}
	serverTypeID := extractID(serverTypeStr)

	locationStr := results["location"]
	if locationStr == "" {
		locationStr = "fsn1" // Default fallback
	}

	return &config.HetznerConfig{
		IP:          "",
		Username:    "deploy",
		SSHKey:      keyPath,
		SSHKeyName:  keyName,
		Location:    locationStr,
		ServerType:  serverTypeID,
		Provisioned: true,
	}
}

func buildVultrConfig(results map[string]string, flow *DynamicProviderFlow, projectName string) *config.VultrConfig {
	// Token should already be saved by the dynamic step handler, but double-check
	if token, ok := results["api_token"]; ok && token != "" {
		tokens, _ := config.LoadTokens()
		if !tokens.HasToken("vultr") {
			tokens.SetToken("vultr", token)
			tokens.SaveTokens()
		}
	}

	// Generate SSH key
	keyName := sshpkg.GetKeyName(projectName)
	keyPath := generateSSHKeyIfNeeded(keyName)

	// Extract plan ID
	planStr, hasPlan := results["plan"]
	if !hasPlan || planStr == "" {
		// This should not happen, but handle gracefully
		planStr = "vc2-1c-1gb" // Default fallback
	}
	planID := extractID(planStr)

	regionStr := results["region"]
	if regionStr == "" {
		regionStr = "ewr" // Default fallback
	}

	return &config.VultrConfig{
		IP:          "",
		Username:    "deploy",
		SSHKey:      keyPath,
		SSHKeyName:  keyName,
		Region:      regionStr,
		Plan:        planID,
		Provisioned: true,
	}
}

func generateSSHKeyIfNeeded(keyName string) string {
	exists, _ := sshpkg.KeyExists(keyName)
	if !exists {
		keyPair, _ := sshpkg.GenerateKeyPair(keyName)
		return keyPair.PrivateKeyPath
	}

	keysDir, _ := sshpkg.GetKeysDirectory()
	return filepath.Join(keysDir, keyName)
}

func extractID(str string) string {
	// Extract ID from "id (description)" format
	for i, r := range str {
		if r == ' ' && i+1 < len(str) && str[i+1] == '(' {
			return str[:i]
		}
	}
	return str
}
