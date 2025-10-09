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
			"flyio",
			"byos",
			"existing",
		).
		OptionLabels(
			"DigitalOcean",
			"Vultr",
			"Hetzner",
			"Fly.io",
			"BYOS",
			"Existing server",
		).
		OptionDescriptions(
			"",
			"",
			"",
			"Docker deployment only",
			"Bring your own server",
			"Deploy to existing server",
		).
		Required().
		Build()
}

func RunProviderSelectionWithConfigFlow(projectName string) (provider string, cfg interface{}, err error) {
	providerStep := CreateProviderSelectionStep("provider")
	steps := []Step{providerStep}

	flow := NewFlow("Configure Deployment", steps)
	flow.SetProjectName(projectName)

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

	case "flyio":
		flyioConfig := buildFlyioConfig(results, final, projectName)
		return "flyio", flyioConfig, nil

	case "existing":
		existingConfig := buildExistingServerConfig(results)
		return "existing", existingConfig, nil

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
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		if m.CurrentStep == 0 {
			currentStep := m.getCurrentStep()
			if currentStep.ID == "provider" && currentStep.Type == StepTypeSelect {
				if currentStep.Cursor >= 0 && currentStep.Cursor < len(currentStep.Options) {
					selectedProvider := currentStep.Options[currentStep.Cursor]
					m.clearProviderSteps()
					if err := m.addProviderSteps(selectedProvider); err == nil {
						m.ProviderSelected = true
						if m.NeedsDynamicSteps {
							m.TokenStepIndex = 1
						}
						m.Completed = false
					}
				}
			}
		}

		if m.NeedsDynamicSteps && m.CurrentStep == m.TokenStepIndex {
			currentStep := m.getCurrentStep()
			if currentStep.Value != "" && currentStep.ID == "api_token" {
				token := currentStep.Value
				tokens, _ := config.LoadTokens()
				tokens.SetToken(m.ProviderForDynamic, token)
				tokens.SaveTokens()

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
				case "flyio":
					newSteps = []Step{
						CreateFlyioRegionStepDynamic("region", token),
						CreateFlyioSizeStepDynamic("size", token, ""),
					}
				}

				m.Steps = append(m.Steps, newSteps...)
				for i := len(m.StepStates); i < len(m.Steps); i++ {
					m.StepStates[i] = m.Steps[i]
				}

				m.NeedsDynamicSteps = false
				m.Completed = false
			}
		}

		// Handle existing server flow - add port step after server selection
		if m.NeedsDynamicSteps && m.ProviderForDynamic == "existing" && m.CurrentStep == m.TokenStepIndex {
			currentStep := m.getCurrentStep()
			if currentStep.Value != "" && currentStep.ID == "server_ip" {
				serverIP := currentStep.Value

				// Create port step with used ports information
				portStep := CreatePortStepWithUsedPorts("port", serverIP)

				m.Steps = append(m.Steps, portStep)
				for i := len(m.StepStates); i < len(m.Steps); i++ {
					m.StepStates[i] = m.Steps[i]
				}

				m.NeedsDynamicSteps = false
				m.Completed = false
			}
		}
	}

	updatedModel, cmd := m.FlowModel.Update(msg)
	if flowModel, ok := updatedModel.(FlowModel); ok {
		*m.FlowModel = flowModel
	}

	return m, cmd
}

func (m *DynamicProviderFlow) View() string {
	return m.FlowModel.View()
}

func (m *DynamicProviderFlow) clearProviderSteps() {
	if len(m.Steps) > 1 {
		m.Steps = m.Steps[:1]
		newStepStates := make(map[int]Step)
		newSSHHandlers := make(map[int]*SSHKeyHandler)
		for i := 0; i < len(m.Steps); i++ {
			if state, exists := m.StepStates[i]; exists {
				newStepStates[i] = state
			}
			if handler, exists := m.SSHHandlers[i]; exists {
				newSSHHandlers[i] = handler
			}
		}
		m.StepStates = newStepStates
		m.SSHHandlers = newSSHHandlers
	}
	m.ProviderSelected = false
	m.NeedsDynamicSteps = false
	m.ProviderForDynamic = ""
	m.TokenStepIndex = 0
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

		activeToken := existingToken
		if !hasToken {
			m.NeedsDynamicSteps = true
			m.ProviderForDynamic = "hetzner"
		} else {
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

		activeToken := existingToken
		if !hasToken {
			m.NeedsDynamicSteps = true
			m.ProviderForDynamic = "vultr"
		} else {
			newSteps = append(newSteps,
				CreateVultrRegionStepDynamic("region", activeToken),
				CreateVultrPlanStepDynamic("plan", activeToken, ""),
			)
		}

	case "flyio":
		existingToken := tokens.GetToken("flyio")
		hasToken := existingToken != ""

		if !hasToken {
			newSteps = append(newSteps, CreateFlyioAPITokenStep("api_token"))
		}

		activeToken := existingToken
		if !hasToken {
			m.NeedsDynamicSteps = true
			m.ProviderForDynamic = "flyio"
		} else {
			newSteps = append(newSteps,
				CreateFlyioRegionStepDynamic("region", activeToken),
				CreateFlyioSizeStepDynamic("size", activeToken, ""),
			)
		}

	case "existing":
		newSteps = []Step{
			CreateExistingServerStep("server_ip"),
			// Port step will be added dynamically after server selection
		}
		m.NeedsDynamicSteps = true
		m.ProviderForDynamic = "existing"

	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	m.Steps = append(m.Steps, newSteps...)

	for i := len(m.StepStates); i < len(m.Steps); i++ {
		m.StepStates[i] = m.Steps[i]
		if m.Steps[i].Type == StepTypeSSHKey {
			m.SSHHandlers[i] = NewSSHKeyHandler(m.ProjectName)
		}
	}

	return nil
}

func buildDigitalOceanConfig(results map[string]string, _ *DynamicProviderFlow, projectName string) *config.DigitalOceanConfig {
	if token, ok := results["api_token"]; ok && token != "" {
		tokens, _ := config.LoadTokens()
		tokens.SetToken("digitalocean", token)
		tokens.SaveTokens()
	}
	keyName := sshpkg.GetKeyName(projectName)
	keyPath := generateSSHKeyIfNeeded(keyName)

	sizeStr, hasSize := results["size"]
	if !hasSize || sizeStr == "" {
		sizeStr = "s-1vcpu-512mb-10gb"
	}
	sizeID := extractID(sizeStr)

	if sizeID == "" {
		sizeID = "s-1vcpu-512mb-10gb"
	}

	regionStr := results["region"]
	if regionStr == "" {
		regionStr = "nyc1"
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

func buildHetznerConfig(results map[string]string, _ *DynamicProviderFlow, projectName string) *config.HetznerConfig {
	if token, ok := results["api_token"]; ok && token != "" {
		tokens, _ := config.LoadTokens()
		if !tokens.HasToken("hetzner") {
			tokens.SetToken("hetzner", token)
			tokens.SaveTokens()
		}
	}

	keyName := sshpkg.GetKeyName(projectName)
	keyPath := generateSSHKeyIfNeeded(keyName)

	serverTypeStr, hasServerType := results["server_type"]
	if !hasServerType || serverTypeStr == "" {
		serverTypeStr = "cx11"
	}
	serverTypeID := extractID(serverTypeStr)

	locationStr := results["location"]
	if locationStr == "" {
		locationStr = "fsn1"
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

func buildVultrConfig(results map[string]string, _ *DynamicProviderFlow, projectName string) *config.VultrConfig {
	if token, ok := results["api_token"]; ok && token != "" {
		tokens, _ := config.LoadTokens()
		if !tokens.HasToken("vultr") {
			tokens.SetToken("vultr", token)
			tokens.SaveTokens()
		}
	}

	keyName := sshpkg.GetKeyName(projectName)
	keyPath := generateSSHKeyIfNeeded(keyName)

	planStr, hasPlan := results["plan"]
	if !hasPlan || planStr == "" {
		planStr = "vc2-1c-1gb"
	}
	planID := extractID(planStr)

	regionStr := results["region"]
	if regionStr == "" {
		regionStr = "ewr"
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

func buildFlyioConfig(results map[string]string, _ *DynamicProviderFlow, projectName string) *config.FlyioConfig {
	if token, ok := results["api_token"]; ok && token != "" {
		tokens, _ := config.LoadTokens()
		if !tokens.HasToken("flyio") {
			tokens.SetToken("flyio", token)
			tokens.SaveTokens()
		}
	}

	keyName := sshpkg.GetKeyName(projectName)
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
		AppName:     "", // Will be set after app creation via SDK
	}
}

func buildExistingServerConfig(results map[string]string) map[string]string {
	// Return server IP and optional port from results
	// The actual setup will be done in createTarget() using setupTargetWithExistingServer()
	config := map[string]string{
		"server_ip": results["server_ip"],
	}

	// Add port if provided
	if port, ok := results["port"]; ok && port != "" {
		config["port"] = port
	}

	return config
}

func extractID(str string) string {
	for i, r := range str {
		if r == ' ' && i+1 < len(str) && str[i+1] == '(' {
			return str[:i]
		}
	}
	return str
}
