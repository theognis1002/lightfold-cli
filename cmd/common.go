package cmd

import (
	"context"
	"fmt"
	"lightfold/cmd/ui/sequential"
	"lightfold/pkg/builders"
	_ "lightfold/pkg/builders/dockerfile"
	_ "lightfold/pkg/builders/native"
	_ "lightfold/pkg/builders/nixpacks"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"lightfold/pkg/detector"
	"lightfold/pkg/providers"
	_ "lightfold/pkg/providers/digitalocean"
	"lightfold/pkg/providers/flyio"
	_ "lightfold/pkg/providers/hetzner"
	_ "lightfold/pkg/providers/vultr"
	"lightfold/pkg/proxy"
	_ "lightfold/pkg/proxy/nginx"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/ssl"
	_ "lightfold/pkg/ssl/certbot"
	"lightfold/pkg/state"
	"lightfold/pkg/util"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tui "lightfold/cmd/ui"

	"github.com/charmbracelet/lipgloss"
)

func loadConfigOrExit() *config.Config {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func loadTargetOrExit(cfg *config.Config, targetName string) config.TargetConfig {
	target, exists := cfg.GetTarget(targetName)
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: Target '%s' not found\n", targetName)
		fmt.Fprintf(os.Stderr, "\nRun 'lightfold status' to list all configured targets\n")
		os.Exit(1)
	}
	return target
}

func resolveTarget(cfg *config.Config, targetFlag string, pathArg string) (config.TargetConfig, string) {
	effectiveTarget := targetFlag
	if effectiveTarget == "" {
		if pathArg != "" {
			effectiveTarget = pathArg
		} else {
			effectiveTarget = "."
		}
	}

	if target, exists := cfg.GetTarget(effectiveTarget); exists {
		return target, effectiveTarget
	}

	projectPath, err := util.ValidateProjectPath(effectiveTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	targetName, target, exists := cfg.FindTargetByPath(projectPath)
	if !exists {
		targetName = util.GetTargetName(projectPath)
		target, exists = cfg.GetTarget(targetName)
		if !exists {
			fmt.Fprintf(os.Stderr, "Error: No target found for this project\n")
			fmt.Fprintf(os.Stderr, "Run 'lightfold create' first\n")
			os.Exit(1)
		}
	}

	return target, targetName
}

func createTarget(targetName, projectPath string, cfg *config.Config) (config.TargetConfig, error) {
	if target, exists := cfg.GetTarget(targetName); exists && state.IsCreated(targetName) {
		skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		fmt.Printf("  %s\n", skipStyle.Render("Infrastructure already created (skipping)"))
		return target, nil
	}

	var err error
	projectPath, err = util.ValidateProjectPath(projectPath)
	if err != nil {
		return config.TargetConfig{}, fmt.Errorf("invalid project path: %w", err)
	}

	detection := detector.DetectFramework(projectPath)

	targetConfig := config.TargetConfig{
		ProjectPath: projectPath,
		Framework:   detection.Framework,
	}

	var provider string
	if providerFlag == "" {
		fmt.Println("") // Blank line for spacing
		selectedProvider, providerConfig, err := sequential.RunProviderSelectionWithConfigFlow(targetName)
		if err != nil {
			return config.TargetConfig{}, fmt.Errorf("interactive configuration failed: %w", err)
		}

		provider = selectedProvider
		// Don't set targetConfig.Provider yet for "existing" - setupTargetWithExistingServer will set it
		if provider != "existing" {
			targetConfig.Provider = provider
		}
		switch provider {
		case "digitalocean":
			if doConfig, ok := providerConfig.(*config.DigitalOceanConfig); ok {
				targetConfig.SetProviderConfig("digitalocean", doConfig)
			} else {
				return config.TargetConfig{}, fmt.Errorf("invalid DigitalOcean configuration")
			}
		case "hetzner":
			if hetznerConfig, ok := providerConfig.(*config.HetznerConfig); ok {
				targetConfig.SetProviderConfig("hetzner", hetznerConfig)
			} else {
				return config.TargetConfig{}, fmt.Errorf("invalid Hetzner configuration")
			}
		case "vultr":
			if vultrConfig, ok := providerConfig.(*config.VultrConfig); ok {
				targetConfig.SetProviderConfig("vultr", vultrConfig)
			} else {
				return config.TargetConfig{}, fmt.Errorf("invalid Vultr configuration")
			}
		case "flyio":
			if flyioConfig, ok := providerConfig.(*config.FlyioConfig); ok {
				targetConfig.SetProviderConfig("flyio", flyioConfig)
			} else {
				return config.TargetConfig{}, fmt.Errorf("invalid Fly.io configuration")
			}
		case "byos":
			if byosConfig, ok := providerConfig.(*config.DigitalOceanConfig); ok {
				targetConfig.SetProviderConfig("byos", byosConfig)
			} else {
				return config.TargetConfig{}, fmt.Errorf("invalid BYOS configuration")
			}
		case "existing":
			if configMap, ok := providerConfig.(map[string]string); ok {
				serverIP := configMap["server_ip"]
				if serverIP == "" {
					return config.TargetConfig{}, fmt.Errorf("server IP is required")
				}

				var userPort int
				if portStr, ok := configMap["port"]; ok && portStr != "" {
					var err error
					userPort, err = strconv.Atoi(portStr)
					if err != nil {
						return config.TargetConfig{}, fmt.Errorf("invalid port number: %w", err)
					}
				}

				if err := setupTargetWithExistingServer(&targetConfig, serverIP, userPort); err != nil {
					return config.TargetConfig{}, fmt.Errorf("failed to setup target with existing server: %w", err)
				}
			} else {
				return config.TargetConfig{}, fmt.Errorf("invalid existing server configuration")
			}
		default:
			return config.TargetConfig{}, fmt.Errorf("unsupported provider: %s", provider)
		}
		if err := cfg.SetTarget(targetName, targetConfig); err != nil {
			return config.TargetConfig{}, fmt.Errorf("failed to save target config: %w", err)
		}

		if err := cfg.SaveConfig(); err != nil {
			return config.TargetConfig{}, fmt.Errorf("failed to save config: %w", err)
		}

		if provider == "byos" || provider == "existing" {
			byosConfig, err := targetConfig.GetSSHProviderConfig()
			if err != nil {
				return config.TargetConfig{}, fmt.Errorf("failed to get BYOS config: %w", err)
			}

			sshExecutor := sshpkg.NewExecutor(byosConfig.GetIP(), "22", byosConfig.GetUsername(), byosConfig.GetSSHKey())
			defer sshExecutor.Disconnect()

			if err := sshExecutor.Connect(3, 2*time.Second); err != nil {
				return config.TargetConfig{}, fmt.Errorf("SSH connection failed: %w", err)
			}

			result := sshExecutor.Execute("echo 'SSH connection successful'")
			if result.Error != nil || result.ExitCode != 0 {
				return config.TargetConfig{}, fmt.Errorf("SSH connection failed: %v", result.Error)
			}

			successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
			mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render("SSH connection validated"))

			// Show port allocation for existing servers
			if targetConfig.Port > 0 {
				fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Allocated to port %d", targetConfig.Port)))
			}

			markerCmd := fmt.Sprintf("sudo mkdir -p %s && echo 'created' | sudo tee %s/%s > /dev/null", config.RemoteLightfoldDir, config.RemoteLightfoldDir, config.RemoteCreatedMarker)
			result = sshExecutor.Execute(markerCmd)
			if result.Error != nil || result.ExitCode != 0 {
				return config.TargetConfig{}, fmt.Errorf("failed to write created marker: %v", result.Error)
			}

			if err := state.MarkCreated(targetName, ""); err != nil {
				return config.TargetConfig{}, fmt.Errorf("failed to update state: %w", err)
			}
			state.ClearCreateFailure(targetName)
		} else {
			projectName := util.GetTargetName(projectPath)
			orchestrator, err := deploy.GetOrchestrator(targetConfig, projectPath, projectName, targetName)
			if err != nil {
				return config.TargetConfig{}, fmt.Errorf("failed to create orchestrator: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), config.DefaultProvisioningTimeout)
			defer cancel()

			result, err := tui.ShowProvisioningProgressWithOrchestrator(ctx, orchestrator)
			if err != nil {
				state.MarkCreateFailed(targetName, err.Error())
				return config.TargetConfig{}, fmt.Errorf("provisioning failed: %w", err)
			}

			if !result.Success {
				state.MarkCreateFailed(targetName, result.Message)
				return config.TargetConfig{}, fmt.Errorf("provisioning failed: %s", result.Message)
			}
			freshCfg, err := config.LoadConfig()
			if err != nil {
				return config.TargetConfig{}, fmt.Errorf("failed to reload config after provisioning: %w", err)
			}
			updatedTarget, exists := freshCfg.GetTarget(targetName)
			if !exists {
				return config.TargetConfig{}, fmt.Errorf("target '%s' not found in config after provisioning", targetName)
			}
			targetConfig = updatedTarget
			serverID := ""
			if result.Server != nil {
				serverID = result.Server.ID
			}
			if err := state.MarkCreated(targetName, serverID); err != nil {
				return config.TargetConfig{}, fmt.Errorf("failed to update state: %w", err)
			}
			state.ClearCreateFailure(targetName)
		}

		return targetConfig, nil
	}

	provider = strings.ToLower(providerFlag)

	fmt.Printf("Using provider: %s (from --provider flag)\n", provider)

	if provider == "byos" {
		if err := handleBYOSWithFlags(&targetConfig, targetName); err != nil {
			return config.TargetConfig{}, err
		}
	} else {
		if err := handleProvisionWithFlags(&targetConfig, targetName, projectPath, provider); err != nil {
			return config.TargetConfig{}, err
		}
	}
	if err := cfg.SetTarget(targetName, targetConfig); err != nil {
		return config.TargetConfig{}, fmt.Errorf("failed to save target config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return config.TargetConfig{}, fmt.Errorf("failed to save config: %w", err)
	}

	return targetConfig, nil
}

func configureTarget(target config.TargetConfig, targetName string, force bool) error {
	// Skip SSH configuration for container providers (e.g., Fly.io)
	tokens, _ := config.LoadTokens()
	if tokens != nil {
		token := tokens.GetToken(target.Provider)
		if token != "" {
			provider, err := providers.GetProvider(target.Provider, token)
			if err == nil && !provider.SupportsSSH() {
				// Container providers don't need SSH configuration
				skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
				fmt.Printf("%s\n", skipStyle.Render("Container provider detected - skipping SSH configuration"))
				return nil
			}
		}
	}

	projectPath := target.ProjectPath
	if target.Provider == "digitalocean" {
		doConfig, err := target.GetDigitalOceanConfig()
		if err == nil {
			dropletID := doConfig.DropletID
			if dropletID == "" {
				if targetState, err := state.LoadState(targetName); err == nil && targetState.ProvisionedID != "" {
					dropletID = targetState.ProvisionedID
				}
			}

			if doConfig.IP == "" && dropletID != "" {
				if err := recoverIPFromDigitalOcean(&target, targetName, dropletID); err != nil {
					state.MarkConfigureFailed(targetName, fmt.Sprintf("Failed to recover IP: %v", err))
					return fmt.Errorf("failed to recover IP: %w", err)
				}
			}
		}
	}

	if target.Provider == "hetzner" {
		hetznerConfig, err := target.GetHetznerConfig()
		if err == nil {
			serverID := hetznerConfig.ServerID
			if serverID == "" {
				if targetState, err := state.LoadState(targetName); err == nil && targetState.ProvisionedID != "" {
					serverID = targetState.ProvisionedID
				}
			}

			if hetznerConfig.IP == "" && serverID != "" {
				if err := recoverIPFromHetzner(&target, targetName, serverID); err != nil {
					state.MarkConfigureFailed(targetName, fmt.Sprintf("Failed to recover IP: %v", err))
					return fmt.Errorf("failed to recover IP: %w", err)
				}
			}
		}
	}

	if target.Provider == "vultr" {
		vultrConfig, err := target.GetVultrConfig()
		if err == nil {
			instanceID := vultrConfig.InstanceID
			if instanceID == "" {
				if targetState, err := state.LoadState(targetName); err == nil && targetState.ProvisionedID != "" {
					instanceID = targetState.ProvisionedID
				}
			}

			if vultrConfig.IP == "" && instanceID != "" {
				if err := recoverIPFromVultr(&target, targetName, instanceID); err != nil {
					state.MarkConfigureFailed(targetName, fmt.Sprintf("Failed to recover IP: %v", err))
					return fmt.Errorf("failed to recover IP: %w", err)
				}
			}
		}
	}

	if target.Provider == "flyio" {
		flyioConfig, err := target.GetFlyioConfig()
		if err == nil {
			machineID := flyioConfig.MachineID
			if machineID == "" {
				if targetState, err := state.LoadState(targetName); err == nil && targetState.ProvisionedID != "" {
					machineID = targetState.ProvisionedID
				}
			}

			if flyioConfig.IP == "" && machineID != "" {
				if err := recoverIPFromFlyio(&target, targetName, machineID); err != nil {
					state.MarkConfigureFailed(targetName, fmt.Sprintf("Failed to recover IP: %v", err))
					return fmt.Errorf("failed to recover IP: %w", err)
				}
			}
		}
	}

	if err := validateConfigureTargetConfig(target); err != nil {
		return fmt.Errorf("invalid target configuration: %w", err)
	}

	// Check if server is already configured
	if !force {
		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			return err
		}

		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		if err := sshExecutor.Connect(3, 2*time.Second); err == nil {
			result := sshExecutor.Execute(fmt.Sprintf("test -f %s/%s && echo 'configured'", config.RemoteLightfoldDir, config.RemoteConfiguredMarker))

			if result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == "configured" {
				// Server is configured, but check if we need to install a new runtime for multi-app scenario
				needsRuntimeInstall := checkIfRuntimeNeeded(sshExecutor, projectPath)
				sshExecutor.Disconnect()

				if !needsRuntimeInstall {
					if err := state.MarkConfigured(targetName); err != nil {
						fmt.Printf("Warning: failed to update local state: %v\n", err)
					}
					skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
					fmt.Printf("%s\n", skipStyle.Render("Server already configured (skipping)"))
					return nil
				}
				// Runtime needed for new app - continue to configuration
				mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
				fmt.Printf("%s\n", mutedStyle.Render("Server configured, but installing runtime for this app..."))
			} else {
				sshExecutor.Disconnect()
			}
		}
	}

	projectName := util.GetTargetName(projectPath)
	orchestrator, err := deploy.GetOrchestrator(target, projectPath, projectName, targetName)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultProvisioningTimeout)
	defer cancel()

	if err := tui.ShowConfigurationProgressWithOrchestrator(ctx, orchestrator, providerCfg); err != nil {
		if markErr := state.MarkConfigureFailed(targetName, err.Error()); markErr != nil {
			fmt.Printf("Warning: failed to mark configure failure in state: %v\n", markErr)
		}
		return fmt.Errorf("configuration failed: %w", err)
	}

	if err := state.ClearConfigureFailure(targetName); err != nil {
		fmt.Printf("Warning: failed to clear configure failure in state: %v\n", err)
	}
	if err := state.MarkConfigured(targetName); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Warning: failed to load config for cleanup: %v\n", err)
	} else {
		detection := detector.DetectFramework(projectPath)
		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		defer sshExecutor.Disconnect()

		executor := deploy.NewExecutor(sshExecutor, projectName, projectPath, &detection)
		if err := executor.CleanupOldReleases(cfg.KeepReleases); err != nil {
			fmt.Printf("Warning: failed to cleanup old releases: %v\n", err)
		}
	}

	return nil
}

func handleBYOSWithFlags(targetConfig *config.TargetConfig, targetName string) error {
	if ipFlag == "" {
		return fmt.Errorf("--ip flag is required for BYOS mode")
	}
	if sshKeyFlag == "" {
		return fmt.Errorf("--ssh-key flag is required for BYOS mode")
	}
	if userFlag == "" {
		userFlag = "root"
	}

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	sshExecutor := sshpkg.NewExecutor(ipFlag, "22", userFlag, sshKeyFlag)
	defer sshExecutor.Disconnect()

	result := sshExecutor.Execute("echo 'SSH connection successful'")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("SSH connection failed: %v", result.Error)
	}

	fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render("SSH connection validated"))

	markerCmd := fmt.Sprintf("sudo mkdir -p %s && echo 'created' | sudo tee %s/%s > /dev/null", config.RemoteLightfoldDir, config.RemoteLightfoldDir, config.RemoteCreatedMarker)
	result = sshExecutor.Execute(markerCmd)
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to write created marker: %v", result.Error)
	}

	targetConfig.Provider = "byos"
	byosConfig := &config.DigitalOceanConfig{
		IP:          ipFlag,
		SSHKey:      sshKeyFlag,
		Username:    userFlag,
		Provisioned: false,
	}
	targetConfig.SetProviderConfig("byos", byosConfig)
	if err := state.MarkCreated(targetName, ""); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	return nil
}

func handleProvisionWithFlags(targetConfig *config.TargetConfig, targetName, projectPath, provider string) error {
	if regionFlag == "" {
		return fmt.Errorf("--region flag is required for provisioning")
	}
	if sizeFlag == "" {
		return fmt.Errorf("--size flag is required for provisioning")
	}
	if imageFlag == "" {
		imageFlag = "ubuntu-22-04-x64"
	}
	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	token := tokens.GetToken(provider)
	if token == "" {
		if provider == "do" || provider == "digitalocean" {
			doConfig, err := sequential.RunProvisionDigitalOceanFlow(targetName)
			if err != nil {
				return fmt.Errorf("failed to get DigitalOcean token: %w", err)
			}
			tokens, _ = config.LoadTokens()
			token = tokens.GetDigitalOceanToken()
			if token == "" {
				return fmt.Errorf("no DigitalOcean API token provided")
			}
			targetConfig.Provider = "digitalocean"
			targetConfig.SetProviderConfig("digitalocean", doConfig)
		} else if provider == "hetzner" {
			hetznerConfig, err := sequential.RunProvisionHetznerFlow(targetName)
			if err != nil {
				return fmt.Errorf("failed to get Hetzner token: %w", err)
			}
			tokens, _ = config.LoadTokens()
			token = tokens.GetToken("hetzner")
			if token == "" {
				return fmt.Errorf("no Hetzner Cloud API token provided")
			}
			targetConfig.Provider = "hetzner"
			targetConfig.SetProviderConfig("hetzner", hetznerConfig)
		} else if provider == "vultr" {
			vultrConfig, err := sequential.RunProvisionVultrFlow(targetName)
			if err != nil {
				return fmt.Errorf("failed to get Vultr token: %w", err)
			}
			tokens, _ = config.LoadTokens()
			token = tokens.GetToken("vultr")
			if token == "" {
				return fmt.Errorf("no Vultr API token provided")
			}
			targetConfig.Provider = "vultr"
			targetConfig.SetProviderConfig("vultr", vultrConfig)
		} else if provider == "flyio" {
			flyioConfig, err := sequential.RunProvisionFlyioFlow(targetName)
			if err != nil {
				return fmt.Errorf("failed to get Fly.io token: %w", err)
			}
			tokens, _ = config.LoadTokens()
			token = tokens.GetToken("flyio")
			if token == "" {
				return fmt.Errorf("no Fly.io API token provided")
			}
			targetConfig.Provider = "flyio"
			targetConfig.SetProviderConfig("flyio", flyioConfig)
		} else {
			return fmt.Errorf("provider %s not supported yet", provider)
		}
	} else {
		switch provider {
		case "do", "digitalocean":
			targetConfig.Provider = "digitalocean"

			sshKeyPath := filepath.Join(os.Getenv("HOME"), config.LocalConfigDir, config.LocalKeysDir, "lightfold_ed25519")
			if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
				publicKeyPath, err := sshpkg.GenerateKeyPair(sshKeyPath)
				if err != nil {
					return fmt.Errorf("failed to generate SSH key: %w", err)
				}
				_ = publicKeyPath
			}

			doConfig := &config.DigitalOceanConfig{
				Region:      regionFlag,
				Size:        sizeFlag,
				SSHKey:      sshKeyPath,
				Username:    "deploy",
				Provisioned: true,
			}
			targetConfig.SetProviderConfig("digitalocean", doConfig)
		case "hetzner":
			targetConfig.Provider = "hetzner"

			sshKeyPath := filepath.Join(os.Getenv("HOME"), config.LocalConfigDir, config.LocalKeysDir, "lightfold_ed25519")
			if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
				publicKeyPath, err := sshpkg.GenerateKeyPair(sshKeyPath)
				if err != nil {
					return fmt.Errorf("failed to generate SSH key: %w", err)
				}
				_ = publicKeyPath
			}

			hetznerConfig := &config.HetznerConfig{
				Location:    regionFlag,
				ServerType:  sizeFlag,
				SSHKey:      sshKeyPath,
				Username:    "deploy",
				Provisioned: true,
			}
			targetConfig.SetProviderConfig("hetzner", hetznerConfig)
		case "vultr":
			targetConfig.Provider = "vultr"

			sshKeyPath := filepath.Join(os.Getenv("HOME"), config.LocalConfigDir, config.LocalKeysDir, "lightfold_ed25519")
			if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
				publicKeyPath, err := sshpkg.GenerateKeyPair(sshKeyPath)
				if err != nil {
					return fmt.Errorf("failed to generate SSH key: %w", err)
				}
				_ = publicKeyPath
			}

			vultrConfig := &config.VultrConfig{
				Region:      regionFlag,
				Plan:        sizeFlag,
				SSHKey:      sshKeyPath,
				Username:    "deploy",
				Provisioned: true,
			}
			targetConfig.SetProviderConfig("vultr", vultrConfig)
		case "flyio":
			targetConfig.Provider = "flyio"

			sshKeyPath := filepath.Join(os.Getenv("HOME"), config.LocalConfigDir, config.LocalKeysDir, "lightfold_ed25519")
			if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
				publicKeyPath, err := sshpkg.GenerateKeyPair(sshKeyPath)
				if err != nil {
					return fmt.Errorf("failed to generate SSH key: %w", err)
				}
				_ = publicKeyPath
			}

			flyioConfig := &config.FlyioConfig{
				Region:      regionFlag,
				Size:        sizeFlag,
				SSHKey:      sshKeyPath,
				Username:    "root",
				Provisioned: true,
			}
			targetConfig.SetProviderConfig("flyio", flyioConfig)
		default:
			return fmt.Errorf("provider %s not supported yet", provider)
		}
	}

	projectName := util.GetTargetName(projectPath)
	orchestrator, err := deploy.GetOrchestrator(*targetConfig, projectPath, projectName, targetName)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultProvisioningTimeout)
	defer cancel()

	result, err := orchestrator.Deploy(ctx)
	if err != nil {
		state.MarkCreateFailed(targetName, err.Error())
		return fmt.Errorf("provisioning failed: %w", err)
	}

	if !result.Success {
		state.MarkCreateFailed(targetName, result.Message)
		return fmt.Errorf("provisioning failed: %s", result.Message)
	}

	cfg, _ := config.LoadConfig()
	updatedTarget, _ := cfg.GetTarget(targetName)
	*targetConfig = updatedTarget

	if result.Server != nil {
		if err := state.MarkCreated(targetName, result.Server.ID); err != nil {
			return fmt.Errorf("failed to update state: %w", err)
		}
		state.ClearCreateFailure(targetName)
	}

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Server provisioned at %s", result.Server.PublicIPv4)))

	return nil
}

var validateConfigureTargetConfig = func(target config.TargetConfig) error {
	if target.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	if target.Provider == "s3" {
		return fmt.Errorf("configure command does not support S3 deployments")
	}

	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		return err
	}

	if providerCfg.GetIP() == "" {
		return fmt.Errorf("IP address is required. Please run 'lightfold create' first")
	}

	if providerCfg.GetUsername() == "" {
		return fmt.Errorf("username is required")
	}

	if providerCfg.GetSSHKey() == "" {
		return fmt.Errorf("SSH key is required")
	}

	return nil
}

var recoverIPFromDigitalOcean = func(target *config.TargetConfig, targetName, dropletID string) error {
	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	doToken := tokens["digitalocean"]
	if doToken == "" {
		return fmt.Errorf("DigitalOcean API token not found")
	}

	provider, err := providers.GetProvider("digitalocean", doToken)
	if err != nil {
		return fmt.Errorf("failed to get DigitalOcean provider: %w", err)
	}

	server, err := provider.GetServer(context.Background(), dropletID)
	if err != nil {
		return fmt.Errorf("failed to fetch droplet info: %w", err)
	}

	doConfig, err := target.GetDigitalOceanConfig()
	if err != nil {
		return fmt.Errorf("failed to get DigitalOcean config: %w", err)
	}

	doConfig.IP = server.PublicIPv4
	doConfig.DropletID = dropletID
	target.SetProviderConfig("digitalocean", doConfig)

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Recovered IP: %s", server.PublicIPv4)))

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetTarget(targetName, *target); err != nil {
		return fmt.Errorf("failed to update target config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

var recoverIPFromHetzner = func(target *config.TargetConfig, targetName, serverID string) error {
	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	hetznerToken := tokens["hetzner"]
	if hetznerToken == "" {
		return fmt.Errorf("Hetzner Cloud API token not found")
	}

	provider, err := providers.GetProvider("hetzner", hetznerToken)
	if err != nil {
		return fmt.Errorf("failed to get Hetzner provider: %w", err)
	}

	server, err := provider.GetServer(context.Background(), serverID)
	if err != nil {
		return fmt.Errorf("failed to fetch server info: %w", err)
	}

	hetznerConfig, err := target.GetHetznerConfig()
	if err != nil {
		return fmt.Errorf("failed to get Hetzner config: %w", err)
	}

	hetznerConfig.IP = server.PublicIPv4
	hetznerConfig.ServerID = serverID
	target.SetProviderConfig("hetzner", hetznerConfig)

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Recovered IP: %s", server.PublicIPv4)))

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetTarget(targetName, *target); err != nil {
		return fmt.Errorf("failed to update target config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

var recoverIPFromVultr = func(target *config.TargetConfig, targetName, instanceID string) error {
	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	vultrToken := tokens["vultr"]
	if vultrToken == "" {
		return fmt.Errorf("Vultr API token not found")
	}

	provider, err := providers.GetProvider("vultr", vultrToken)
	if err != nil {
		return fmt.Errorf("failed to get Vultr provider: %w", err)
	}

	server, err := provider.GetServer(context.Background(), instanceID)
	if err != nil {
		return fmt.Errorf("failed to fetch instance info: %w", err)
	}

	vultrConfig, err := target.GetVultrConfig()
	if err != nil {
		return fmt.Errorf("failed to get Vultr config: %w", err)
	}

	vultrConfig.IP = server.PublicIPv4
	vultrConfig.InstanceID = instanceID
	target.SetProviderConfig("vultr", vultrConfig)

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Recovered IP: %s", server.PublicIPv4)))

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetTarget(targetName, *target); err != nil {
		return fmt.Errorf("failed to update target config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

var recoverIPFromFlyio = func(target *config.TargetConfig, targetName, machineID string) error {
	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	flyioToken := tokens["flyio"]
	if flyioToken == "" {
		return fmt.Errorf("Fly.io API token not found")
	}

	flyioConfig, err := target.GetFlyioConfig()
	if err != nil {
		return fmt.Errorf("failed to get Fly.io config: %w", err)
	}

	if flyioConfig.AppName == "" {
		return fmt.Errorf("Fly.io app name not found in config. This usually means the app wasn't fully created. Please check Fly.io console or run 'lightfold destroy' and try again")
	}

	// Get Fly.io client to fetch IP directly via GraphQL
	provider, err := providers.GetProvider("flyio", flyioToken)
	if err != nil {
		return fmt.Errorf("failed to get Fly.io provider: %w", err)
	}

	flyioClient, ok := provider.(*flyio.Client)
	if !ok {
		return fmt.Errorf("failed to get Fly.io client")
	}

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

	ipAddress, err := flyioClient.GetAppIP(context.Background(), flyioConfig.AppName)
	if err != nil {
		fmt.Println()
		fmt.Printf("%s %s\n", warningStyle.Render("⚠"), warningStyle.Render("Unable to fetch IP automatically from Fly.io API"))
		fmt.Printf("  %s\n", mutedStyle.Render("This is a known Fly.io API propagation issue."))
		fmt.Println()
		linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Underline(true)
		fmt.Printf("  Please get your app's IP address from:\n")
		fmt.Printf("  %s\n", linkStyle.Render(fmt.Sprintf("https://fly.io/apps/%s", flyioConfig.AppName)))
		fmt.Println()

		// Prompt for IP
		fmt.Printf("  Enter IP address: ")
		var userIP string
		if _, scanErr := fmt.Scanln(&userIP); scanErr != nil {
			return fmt.Errorf("failed to read IP address: %w", scanErr)
		}

		// Validate IP format (basic check)
		userIP = strings.TrimSpace(userIP)
		if userIP == "" {
			return fmt.Errorf("IP address cannot be empty")
		}

		// Basic IPv4 validation
		parts := strings.Split(userIP, ".")
		if len(parts) != 4 {
			return fmt.Errorf("invalid IP address format: %s (expected format: xxx.xxx.xxx.xxx)", userIP)
		}

		ipAddress = userIP
		fmt.Printf("  %s\n", successStyle.Render(fmt.Sprintf("✓ Using IP: %s", ipAddress)))
		fmt.Println()
	}

	flyioConfig.IP = ipAddress
	flyioConfig.MachineID = machineID
	target.SetProviderConfig("flyio", flyioConfig)

	if err == nil {
		fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Recovered IP: %s", ipAddress)))
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetTarget(targetName, *target); err != nil {
		return fmt.Errorf("failed to update target config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func resolveBuilder(target config.TargetConfig, projectPath string, detection *detector.Detection, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if target.Builder != "" {
		if builder, err := builders.GetBuilder(target.Builder); err == nil && builder.IsAvailable() {
			return target.Builder
		}
	}

	builderName, err := builders.AutoSelectBuilder(projectPath, detection)
	if err != nil {
		return "native"
	}
	return builderName
}

func configureDomainAndSSL(target *config.TargetConfig, targetName string, domain string, enableSSL bool) error {
	if domain == "" {
		return fmt.Errorf("domain is required")
	}

	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		return fmt.Errorf("failed to get SSH config: %w", err)
	}

	sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
	defer sshExecutor.Disconnect()

	if err := sshExecutor.Connect(3, 2*time.Second); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	if target.Domain == nil {
		target.Domain = &config.DomainConfig{}
	}
	target.Domain.Domain = domain
	target.Domain.SSLEnabled = enableSSL
	target.Domain.ProxyType = "nginx"

	if enableSSL {
		target.Domain.SSLManager = "certbot"

		checkResult := sshExecutor.Execute("which certbot")
		if checkResult.ExitCode != 0 {
			installCmd := "sudo apt-get update && sudo apt-get install -y certbot python3-certbot-nginx"
			installResult := sshExecutor.Execute(installCmd)
			if installResult.Error != nil || installResult.ExitCode != 0 {
				return fmt.Errorf("failed to install certbot: %v", installResult.Error)
			}
		}
	}

	proxyManager, err := proxy.GetManager("nginx")
	if err != nil {
		return fmt.Errorf("failed to get proxy manager: %w", err)
	}

	if nginxMgr, ok := proxyManager.(interface{ SetExecutor(*sshpkg.Executor) }); ok {
		nginxMgr.SetExecutor(sshExecutor)
	}

	// Get or allocate port for this app
	port := target.Port
	if port == 0 {
		port = extractPortFromTarget(target, target.ProjectPath)
	}

	appName := targetName

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	httpOnlyConfig := proxy.ProxyConfig{
		Domain:      domain,
		Port:        port,
		AppName:     appName,
		SSLEnabled:  false, // HTTP only first
		SSLCertPath: "",
		SSLKeyPath:  "",
	}
	if err := proxyManager.Configure(httpOnlyConfig); err != nil {
		return fmt.Errorf("failed to configure proxy: %w", err)
	}

	if err := proxyManager.Reload(); err != nil {
		return fmt.Errorf("failed to reload proxy: %w", err)
	}

	fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render("Configured reverse proxy with domain"))

	if enableSSL {
		sslManager, err := ssl.GetManager("certbot")
		if err != nil {
			return fmt.Errorf("failed to get SSL manager: %w", err)
		}

		if certbotMgr, ok := sslManager.(interface{ SetExecutor(*sshpkg.Executor) }); ok {
			certbotMgr.SetExecutor(sshExecutor)
		}

		email := "noreply@" + domain
		if err := sslManager.IssueCertificate(domain, email); err != nil {
			return fmt.Errorf("failed to issue SSL certificate: %w", err)
		}

		if err := sslManager.EnableAutoRenewal(); err != nil {
			fmt.Printf("Warning: failed to enable auto-renewal: %v\n", err)
		}

		if err := state.MarkSSLConfigured(targetName); err != nil {
			fmt.Printf("Warning: failed to update SSL state: %v\n", err)
		}

		fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Issued SSL certificate for %s", domain)))
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetTarget(targetName, *target); err != nil {
		return fmt.Errorf("failed to update target config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	protocol := "http"
	if enableSSL {
		protocol = "https"
	}

	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	fmt.Printf("\n%s %s\n", successStyle.Render("✓"), successStyle.Render("Domain configured successfully!"))
	fmt.Printf("  %s %s\n\n", mutedStyle.Render("Your app is now available at:"), valueStyle.Render(fmt.Sprintf("%s://%s", protocol, domain)))

	return nil
}

func extractPortFromTarget(target *config.TargetConfig, projectPath string) int {
	detection := detector.DetectFramework(projectPath)

	for _, runCmd := range detection.RunPlan {
		if strings.Contains(runCmd, "-port") || strings.Contains(runCmd, "--port") {
			parts := strings.Fields(runCmd)
			for i, part := range parts {
				if (part == "-port" || part == "--port") && i+1 < len(parts) {
					if port, err := strconv.Atoi(parts[i+1]); err == nil {
						return port
					}
				}
			}
		}

		if strings.Contains(runCmd, "--bind") || strings.Contains(runCmd, "--listen") {
			parts := strings.Fields(runCmd)
			for i, part := range parts {
				if (part == "--bind" || part == "--listen") && i+1 < len(parts) {
					bindAddr := parts[i+1]
					if colonIdx := strings.LastIndex(bindAddr, ":"); colonIdx != -1 {
						portStr := bindAddr[colonIdx+1:]
						if port, err := strconv.Atoi(portStr); err == nil {
							return port
						}
					}
				}
			}
		}
	}

	switch detection.Framework {
	case "Next.js", "Nuxt.js", "Remix", "SvelteKit", "Astro":
		return 3000
	case "Django", "Flask", "FastAPI":
		return 5000
	case "Go", "Fiber":
		return 8080
	case "Express.js":
		return 3000
	case "Ruby on Rails":
		return 3000
	default:
		return 3000
	}
}

func syncTarget(target config.TargetConfig, targetName string, cfg *config.Config) (*state.TargetState, error) {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	if target.Provider == "s3" {
		fmt.Printf("%s\n", mutedStyle.Render("S3 deployments don't require syncing (static files)"))
		return nil, nil
	}

	targetState, err := state.LoadState(targetName)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	changesDetected := false

	if target.Provider == "digitalocean" {
		doConfig, err := target.GetDigitalOceanConfig()
		if err == nil && doConfig.Provisioned {
			dropletID := doConfig.DropletID
			if dropletID == "" && targetState.ProvisionedID != "" {
				dropletID = targetState.ProvisionedID
			}

			if dropletID != "" {
				originalIP := doConfig.IP
				fmt.Printf("%s Recovering server IP from DigitalOcean API...\n", labelStyle.Render("→"))

				if err := recoverIPFromDigitalOcean(&target, targetName, dropletID); err != nil {
					fmt.Printf("%s Failed to recover IP: %v\n", mutedStyle.Render("  ⚠"), err)
				} else {
					updatedCfg, _ := config.LoadConfig()
					if updatedTarget, exists := updatedCfg.GetTarget(targetName); exists {
						target = updatedTarget
						if updatedDoConfig, _ := updatedTarget.GetDigitalOceanConfig(); updatedDoConfig != nil {
							if updatedDoConfig.IP != originalIP {
								fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render(fmt.Sprintf("IP updated: %s → %s", originalIP, updatedDoConfig.IP)))
								changesDetected = true
							}
						}
					}
				}
			}
		}
	} else if target.Provider == "hetzner" {
		hetznerConfig, err := target.GetHetznerConfig()
		if err == nil && hetznerConfig.Provisioned {
			serverID := hetznerConfig.ServerID
			if serverID == "" && targetState.ProvisionedID != "" {
				serverID = targetState.ProvisionedID
			}

			if serverID != "" {
				originalIP := hetznerConfig.IP
				fmt.Printf("%s Recovering server IP from Hetzner API...\n", labelStyle.Render("→"))

				if err := recoverIPFromHetzner(&target, targetName, serverID); err != nil {
					fmt.Printf("%s Failed to recover IP: %v\n", mutedStyle.Render("  ⚠"), err)
				} else {
					updatedCfg, _ := config.LoadConfig()
					if updatedTarget, exists := updatedCfg.GetTarget(targetName); exists {
						target = updatedTarget
						if updatedHetznerConfig, _ := updatedTarget.GetHetznerConfig(); updatedHetznerConfig != nil {
							if updatedHetznerConfig.IP != originalIP {
								fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render(fmt.Sprintf("IP updated: %s → %s", originalIP, updatedHetznerConfig.IP)))
								changesDetected = true
							}
						}
					}
				}
			}
		}
	} else if target.Provider == "vultr" {
		vultrConfig, err := target.GetVultrConfig()
		if err == nil && vultrConfig.Provisioned {
			instanceID := vultrConfig.InstanceID
			if instanceID == "" && targetState.ProvisionedID != "" {
				instanceID = targetState.ProvisionedID
			}

			if instanceID != "" {
				originalIP := vultrConfig.IP
				fmt.Printf("%s Recovering server IP from Vultr API...\n", labelStyle.Render("→"))

				if err := recoverIPFromVultr(&target, targetName, instanceID); err != nil {
					fmt.Printf("%s Failed to recover IP: %v\n", mutedStyle.Render("  ⚠"), err)
				} else {
					updatedCfg, _ := config.LoadConfig()
					if updatedTarget, exists := updatedCfg.GetTarget(targetName); exists {
						target = updatedTarget
						if updatedVultrConfig, _ := updatedTarget.GetVultrConfig(); updatedVultrConfig != nil {
							if updatedVultrConfig.IP != originalIP {
								fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render(fmt.Sprintf("IP updated: %s → %s", originalIP, updatedVultrConfig.IP)))
								changesDetected = true
							}
						}
					}
				}
			}
		}
	}

	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH config: %w", err)
	}

	if providerCfg.GetIP() == "" {
		return nil, fmt.Errorf("server IP address is not configured")
	}

	fmt.Printf("%s Connecting to server at %s...\n", labelStyle.Render("→"), providerCfg.GetIP())

	sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
	if err := sshExecutor.Connect(3, 2*time.Second); err != nil {
		return nil, fmt.Errorf("failed to connect via SSH: %w", err)
	}
	defer sshExecutor.Disconnect()

	fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render("SSH connection established"))

	if !targetState.Created {
		targetState.Created = true
		changesDetected = true
		fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render("Server is created (SSH accessible)"))
	}

	fmt.Printf("%s Syncing remote state markers...\n", labelStyle.Render("→"))

	createdResult := sshExecutor.Execute(fmt.Sprintf("test -f %s/%s && echo 'true' || echo 'false'", config.RemoteLightfoldDir, config.RemoteCreatedMarker))
	if createdResult.ExitCode == 0 {
		remoteCreated := strings.TrimSpace(createdResult.Stdout) == "true"

		if !remoteCreated {
			markerCmd := fmt.Sprintf("sudo mkdir -p %s && echo 'created' | sudo tee %s/%s > /dev/null", config.RemoteLightfoldDir, config.RemoteLightfoldDir, config.RemoteCreatedMarker)
			markerResult := sshExecutor.Execute(markerCmd)
			if markerResult.ExitCode == 0 {
				fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render("Created marker written to server"))
				changesDetected = true
			}
		}
	}

	configuredResult := sshExecutor.Execute(fmt.Sprintf("test -f %s/%s && echo 'true' || echo 'false'", config.RemoteLightfoldDir, config.RemoteConfiguredMarker))
	if configuredResult.ExitCode == 0 {
		remoteConfigured := strings.TrimSpace(configuredResult.Stdout) == "true"
		if targetState.Configured != remoteConfigured {
			targetState.Configured = remoteConfigured
			changesDetected = true
			fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render(fmt.Sprintf("Configured state updated: %v", remoteConfigured)))
		}
	}

	fmt.Printf("%s Syncing deployment information...\n", labelStyle.Render("→"))

	appName := strings.ReplaceAll(targetName, "-", "_")

	currentReleaseResult := sshExecutor.Execute(fmt.Sprintf("readlink -f %s/%s/current 2>/dev/null", config.RemoteAppBaseDir, appName))
	if currentReleaseResult.ExitCode == 0 {
		currentReleasePath := strings.TrimSpace(currentReleaseResult.Stdout)
		if currentReleasePath != "" && !strings.Contains(currentReleasePath, "none") {
			releaseTimestamp := filepath.Base(currentReleasePath)
			if releaseTimestamp != "" && targetState.LastRelease != releaseTimestamp {
				targetState.LastRelease = releaseTimestamp
				changesDetected = true
				fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render(fmt.Sprintf("Current release: %s", releaseTimestamp)))

				gitCommitResult := sshExecutor.Execute(fmt.Sprintf("cat %s/.git-commit 2>/dev/null", currentReleasePath))
				if gitCommitResult.ExitCode == 0 {
					gitCommit := strings.TrimSpace(gitCommitResult.Stdout)
					if gitCommit != "" && targetState.LastCommit != gitCommit {
						targetState.LastCommit = gitCommit
						changesDetected = true
						commitShort := gitCommit
						if len(commitShort) > 7 {
							commitShort = commitShort[:7]
						}
						fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render(fmt.Sprintf("Git commit: %s", commitShort)))
					}
				}

				if !targetState.LastDeploy.IsZero() {
				} else {
					targetState.LastDeploy = time.Now()
					changesDetected = true
				}
			}
		}
	}

	serviceResult := sshExecutor.Execute(fmt.Sprintf("systemctl is-active %s 2>/dev/null", appName))
	if serviceResult.ExitCode == 0 && strings.TrimSpace(serviceResult.Stdout) == "active" {
		fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render("Service is active"))
	}

	if changesDetected {
		fmt.Printf("%s Saving synced state...\n", labelStyle.Render("→"))
		if err := state.SaveState(targetName, targetState); err != nil {
			return nil, fmt.Errorf("failed to save state: %w", err)
		}
		fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render("State saved successfully"))
	} else {
		fmt.Printf("%s %s\n", mutedStyle.Render("ℹ"), mutedStyle.Render("No changes detected - state is already in sync"))
	}

	return targetState, nil
}

// getOrAllocatePort gets the port for a target, allocating one if necessary
func getOrAllocatePort(target *config.TargetConfig, targetName string) (int, error) {
	// If port is already set in target config, use it
	if target.Port > 0 {
		return target.Port, nil
	}

	// If ServerIP is set, use server state for port allocation
	if target.ServerIP != "" {
		// Check if app is already registered with a port
		app, err := state.GetAppFromServer(target.ServerIP, targetName)
		if err == nil && app.Port > 0 {
			return app.Port, nil
		}

		// Allocate a new port
		port, err := state.AllocatePort(target.ServerIP)
		if err != nil {
			return 0, fmt.Errorf("failed to allocate port: %w", err)
		}
		return port, nil
	}

	// Fallback to default port detection
	return extractPortFromTarget(target, target.ProjectPath), nil
}

// registerAppWithServer registers an app in the server state
func registerAppWithServer(target *config.TargetConfig, targetName string, port int, framework string) error {
	if target.ServerIP == "" {
		return nil // Not using server state
	}

	app := state.DeployedApp{
		TargetName: targetName,
		AppName:    targetName,
		Port:       port,
		Framework:  framework,
		LastDeploy: time.Now(),
	}

	// Add domain if configured
	if target.Domain != nil && target.Domain.Domain != "" {
		app.Domain = target.Domain.Domain
	}

	return state.RegisterApp(target.ServerIP, app)
}

// updateServerStateFromTarget ensures server state is initialized from target config
func updateServerStateFromTarget(target *config.TargetConfig, targetName string) error {
	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		return nil // Not an SSH provider
	}

	serverIP := providerCfg.GetIP()
	if serverIP == "" {
		return nil // No IP yet
	}

	// Update target's ServerIP if not set
	if target.ServerIP == "" {
		target.ServerIP = serverIP
	}

	// Get or create server state
	serverState, err := state.GetServerState(serverIP)
	if err != nil {
		return fmt.Errorf("failed to get server state: %w", err)
	}

	// Update server state metadata if empty
	if serverState.Provider == "" {
		serverState.Provider = target.Provider
		serverState.ServerID = providerCfg.GetServerID()

		// Determine proxy type from domain config or default
		if target.Domain != nil && target.Domain.ProxyType != "" {
			serverState.ProxyType = target.Domain.ProxyType
		} else {
			serverState.ProxyType = "nginx" // Default
		}

		// Set root domain if available
		if target.Domain != nil && target.Domain.RootDomain != "" {
			serverState.RootDomain = target.Domain.RootDomain
		}

		if err := state.SaveServerState(serverState); err != nil {
			return fmt.Errorf("failed to save server state: %w", err)
		}
	}

	return nil
}

// setupTargetWithExistingServer configures a target to use an existing server
// checkIfRuntimeNeeded determines if a runtime needs to be installed for the current app
// Returns true if the runtime is missing and needs installation
func checkIfRuntimeNeeded(sshExecutor *sshpkg.Executor, projectPath string) bool {
	detection := detector.DetectFramework(projectPath)

	switch detection.Language {
	case "JavaScript/TypeScript":
		// Check if Node.js is installed
		result := sshExecutor.Execute("/usr/bin/node --version 2>/dev/null || /usr/local/bin/node --version 2>/dev/null || echo 'not-found'")
		nodeVersion := strings.TrimSpace(result.Stdout)
		if nodeVersion == "not-found" || nodeVersion == "" {
			return true // Node.js not installed
		}

		// Check if package manager is installed (if not npm)
		if pm, ok := detection.Meta["package_manager"]; ok && pm != "npm" {
			switch pm {
			case "bun":
				result := sshExecutor.Execute("command -v bun >/dev/null 2>&1 && echo 'found' || echo 'not-found'")
				if strings.TrimSpace(result.Stdout) == "not-found" {
					return true
				}
			case "pnpm":
				result := sshExecutor.Execute("command -v pnpm >/dev/null 2>&1 && echo 'found' || echo 'not-found'")
				if strings.TrimSpace(result.Stdout) == "not-found" {
					return true
				}
			case "yarn":
				result := sshExecutor.Execute("command -v yarn >/dev/null 2>&1 && echo 'found' || echo 'not-found'")
				if strings.TrimSpace(result.Stdout) == "not-found" {
					return true
				}
			}
		}
		return false // Node.js and package manager are installed

	case "Python":
		// Check if Python is installed
		result := sshExecutor.Execute("python3 --version 2>/dev/null || echo 'not-found'")
		pythonVersion := strings.TrimSpace(result.Stdout)
		if pythonVersion == "not-found" || pythonVersion == "" {
			return true // Python not installed
		}

		// Check if package manager is installed (if not pip)
		if pm, ok := detection.Meta["package_manager"]; ok && pm != "pip" {
			switch pm {
			case "poetry":
				result := sshExecutor.Execute("command -v poetry >/dev/null 2>&1 && echo 'found' || echo 'not-found'")
				if strings.TrimSpace(result.Stdout) == "not-found" {
					return true
				}
			case "pipenv":
				result := sshExecutor.Execute("command -v pipenv >/dev/null 2>&1 && echo 'found' || echo 'not-found'")
				if strings.TrimSpace(result.Stdout) == "not-found" {
					return true
				}
			case "uv":
				result := sshExecutor.Execute("command -v uv >/dev/null 2>&1 && echo 'found' || echo 'not-found'")
				if strings.TrimSpace(result.Stdout) == "not-found" {
					return true
				}
			}
		}
		return false // Python and package manager are installed

	case "Go":
		result := sshExecutor.Execute("go version 2>/dev/null || echo 'not-found'")
		goVersion := strings.TrimSpace(result.Stdout)
		return goVersion == "not-found" || goVersion == ""

	case "PHP":
		result := sshExecutor.Execute("php --version 2>/dev/null || echo 'not-found'")
		phpVersion := strings.TrimSpace(result.Stdout)
		return phpVersion == "not-found" || phpVersion == ""

	case "Ruby":
		result := sshExecutor.Execute("ruby --version 2>/dev/null || echo 'not-found'")
		rubyVersion := strings.TrimSpace(result.Stdout)
		return rubyVersion == "not-found" || rubyVersion == ""

	default:
		// Unknown language, assume runtime is needed
		return true
	}
}

func setupTargetWithExistingServer(target *config.TargetConfig, serverIP string, userPort int) error {
	// Verify server exists
	if !state.ServerStateExists(serverIP) {
		return fmt.Errorf("server %s not found. Run 'lightfold server list' to see available servers", serverIP)
	}

	// Get server state
	serverState, err := state.GetServerState(serverIP)
	if err != nil {
		return fmt.Errorf("failed to get server state: %w", err)
	}

	// Get SSH key from existing target on this server
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	targetsOnServer := cfg.GetTargetsByServerIP(serverIP)
	var sshKey string
	for _, existingTarget := range targetsOnServer {
		existingProviderCfg, err := existingTarget.GetSSHProviderConfig()
		if err == nil && existingProviderCfg.GetSSHKey() != "" {
			sshKey = existingProviderCfg.GetSSHKey()
			break
		}
	}

	if sshKey == "" {
		// Default SSH key path
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultKeyPath := filepath.Join(homeDir, config.LocalConfigDir, config.LocalKeysDir, "lightfold_ed25519")
			if _, err := os.Stat(defaultKeyPath); err == nil {
				sshKey = defaultKeyPath
			}
		}

		if sshKey == "" {
			return fmt.Errorf("no SSH key found for server %s. Deploy another app to this server first, or use 'lightfold create' with full provisioning", serverIP)
		}
	}

	// Set target's ServerIP
	target.ServerIP = serverIP

	// Set user-provided port if specified
	if userPort > 0 {
		target.Port = userPort
	}

	// Adopt server's provider configuration if not already set
	if target.Provider == "" || target.Provider == serverState.Provider {
		target.Provider = serverState.Provider

		// Create a basic provider config based on server state
		// This allows the target to deploy without needing full provisioning
		switch serverState.Provider {
		case "digitalocean":
			doConfig := &config.DigitalOceanConfig{
				IP:          serverIP,
				DropletID:   serverState.ServerID,
				SSHKey:      sshKey,
				Username:    "deploy",
				Provisioned: false, // Not provisioned by this target
			}
			target.SetProviderConfig("digitalocean", doConfig)
		case "hetzner":
			hetznerConfig := &config.HetznerConfig{
				IP:          serverIP,
				ServerID:    serverState.ServerID,
				SSHKey:      sshKey,
				Username:    "deploy",
				Provisioned: false,
			}
			target.SetProviderConfig("hetzner", hetznerConfig)
		case "vultr":
			vultrConfig := &config.VultrConfig{
				IP:          serverIP,
				InstanceID:  serverState.ServerID,
				SSHKey:      sshKey,
				Username:    "deploy",
				Provisioned: false,
			}
			target.SetProviderConfig("vultr", vultrConfig)
		case "byos":
			// For BYOS, use the SSH key we found
			doConfig := &config.DigitalOceanConfig{
				IP:          serverIP,
				SSHKey:      sshKey,
				Username:    "deploy",
				Provisioned: false,
			}
			target.SetProviderConfig("byos", doConfig)
		default:
			return fmt.Errorf("unsupported provider: %s", serverState.Provider)
		}
	}

	// Copy domain configuration if server has a root domain
	if serverState.RootDomain != "" && (target.Domain == nil || target.Domain.RootDomain == "") {
		if target.Domain == nil {
			target.Domain = &config.DomainConfig{}
		}
		target.Domain.RootDomain = serverState.RootDomain
		target.Domain.ProxyType = serverState.ProxyType
	}

	return nil
}
