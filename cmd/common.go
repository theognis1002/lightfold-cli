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
	_ "lightfold/pkg/providers/hetzner"
	_ "lightfold/pkg/providers/vultr"
	"lightfold/pkg/proxy"
	_ "lightfold/pkg/proxy/nginx"
	"lightfold/pkg/ssl"
	_ "lightfold/pkg/ssl/certbot"
	sshpkg "lightfold/pkg/ssh"
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
		fmt.Printf("%s\n", skipStyle.Render("Infrastructure already created (skipping)"))
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
		targetConfig.Provider = provider
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
		case "byos":
			if byosConfig, ok := providerConfig.(*config.DigitalOceanConfig); ok {
				targetConfig.SetProviderConfig("byos", byosConfig)
			} else {
				return config.TargetConfig{}, fmt.Errorf("invalid BYOS configuration")
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

		if provider == "byos" {
			byosConfig, err := targetConfig.GetSSHProviderConfig()
			if err != nil {
				return config.TargetConfig{}, fmt.Errorf("failed to get BYOS config: %w", err)
			}

			sshExecutor := sshpkg.NewExecutor(byosConfig.GetIP(), "22", byosConfig.GetUsername(), byosConfig.GetSSHKey())
			defer sshExecutor.Disconnect()

			result := sshExecutor.Execute("echo 'SSH connection successful'")
			if result.Error != nil || result.ExitCode != 0 {
				return config.TargetConfig{}, fmt.Errorf("SSH connection failed: %v", result.Error)
			}

			successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
			mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render("SSH connection validated"))

			markerCmd := fmt.Sprintf("sudo mkdir -p %s && echo 'created' | sudo tee %s/%s > /dev/null", config.RemoteLightfoldDir, config.RemoteLightfoldDir, config.RemoteCreatedMarker)
			result = sshExecutor.Execute(markerCmd)
			if result.Error != nil || result.ExitCode != 0 {
				return config.TargetConfig{}, fmt.Errorf("failed to write created marker: %v", result.Error)
			}

			if err := state.MarkCreated(targetName, ""); err != nil {
				return config.TargetConfig{}, fmt.Errorf("failed to update state: %w", err)
			}
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
				return config.TargetConfig{}, fmt.Errorf("provisioning failed: %w", err)
			}

			if !result.Success {
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
					return fmt.Errorf("failed to recover IP: %w", err)
				}
			}
		}
	}

	if err := validateConfigureTargetConfig(target); err != nil {
		return fmt.Errorf("invalid target configuration: %w", err)
	}
	if !force {
		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			return err
		}

		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		if err := sshExecutor.Connect(3, 2*time.Second); err == nil {
			result := sshExecutor.Execute(fmt.Sprintf("test -f %s/%s && echo 'configured'", config.RemoteLightfoldDir, config.RemoteConfiguredMarker))
			sshExecutor.Disconnect()

			if result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == "configured" {
				if err := state.MarkConfigured(targetName); err != nil {
					fmt.Printf("Warning: failed to update local state: %v\n", err)
				}
				skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
				fmt.Printf("%s\n", skipStyle.Render("Server already configured (skipping)"))
				return nil
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
		return fmt.Errorf("configuration failed: %w", err)
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
		return fmt.Errorf("provisioning failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("provisioning failed: %s", result.Message)
	}

	cfg, _ := config.LoadConfig()
	updatedTarget, _ := cfg.GetTarget(targetName)
	*targetConfig = updatedTarget

	if result.Server != nil {
		if err := state.MarkCreated(targetName, result.Server.ID); err != nil {
			return fmt.Errorf("failed to update state: %w", err)
		}
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

// recoverIPFromDigitalOcean fetches the IP address from DigitalOcean API when droplet exists but IP is missing
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

// recoverIPFromHetzner fetches the IP address from Hetzner Cloud API when server exists but IP is missing
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

// recoverIPFromVultr fetches the IP address from Vultr API when instance exists but IP is missing
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

	port := extractPortFromTarget(target, target.ProjectPath)

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
