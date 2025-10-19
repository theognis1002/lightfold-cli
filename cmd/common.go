package cmd

import (
	"context"
	"fmt"
	"lightfold/cmd/ui/sequential"
	"lightfold/cmd/utils"
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
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/ssl"
	_ "lightfold/pkg/ssl/certbot"
	"lightfold/pkg/state"
	"lightfold/pkg/util"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tui "lightfold/cmd/ui"

	"github.com/charmbracelet/lipgloss"
)

// Wrapper functions that delegate to utils package
func loadConfigOrExit() *config.Config {
	return utils.LoadConfigOrExit()
}

func loadTargetOrExit(cfg *config.Config, targetName string) config.TargetConfig {
	return utils.LoadTargetOrExit(cfg, targetName)
}

func resolveTarget(cfg *config.Config, targetFlag string, pathArg string) (config.TargetConfig, string) {
	return utils.ResolveTargetOrExit(cfg, targetFlag, pathArg)
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
		fmt.Println("")
		selectedProvider, providerConfig, err := sequential.RunProviderSelectionWithConfigFlow(targetName)
		if err != nil {
			return config.TargetConfig{}, fmt.Errorf("interactive configuration failed: %w", err)
		}

		provider = strings.ToLower(selectedProvider)

		switch provider {
		case "byos":
			cfgValue, ok := providerConfig.(*config.DigitalOceanConfig)
			if !ok {
				return config.TargetConfig{}, fmt.Errorf("invalid BYOS configuration")
			}
			targetConfig.Provider = "byos"
			if err := targetConfig.SetProviderConfig("byos", cfgValue); err != nil {
				return config.TargetConfig{}, fmt.Errorf("failed to set BYOS config: %w", err)
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

				if err := utils.SetupTargetWithExistingServer(&targetConfig, serverIP, userPort); err != nil {
					return config.TargetConfig{}, fmt.Errorf("failed to setup target with existing server: %w", err)
				}
			} else {
				return config.TargetConfig{}, fmt.Errorf("invalid existing server configuration")
			}
		default:
			bootstrap, err := findProviderBootstrap(provider)
			if err != nil {
				return config.TargetConfig{}, err
			}
			if err := bootstrap.applyConfig(&targetConfig, providerConfig); err != nil {
				return config.TargetConfig{}, err
			}
			provider = bootstrap.canonical
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
				return config.TargetConfig{}, fmt.Errorf("SSH connection failed: %w", result.Error)
			}

			successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
			mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render("SSH connection validated"))

			// Allocate port if not already set
			if targetConfig.Port == 0 {
				port, err := utils.GetOrAllocatePort(&targetConfig, targetName)
				if err != nil {
					return config.TargetConfig{}, fmt.Errorf("failed to allocate port: %w", err)
				}
				targetConfig.Port = port

				// Save config with allocated port
				if err := cfg.SetTarget(targetName, targetConfig); err != nil {
					return config.TargetConfig{}, fmt.Errorf("failed to save target config: %w", err)
				}
				if err := cfg.SaveConfig(); err != nil {
					return config.TargetConfig{}, fmt.Errorf("failed to save config: %w", err)
				}
			}

			// Show port allocation for existing servers
			if targetConfig.Port > 0 {
				fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Allocated to port %d", targetConfig.Port)))
			}

			markerCmd := fmt.Sprintf("sudo mkdir -p %s && echo 'created' | sudo tee %s/%s > /dev/null", config.RemoteLightfoldDir, config.RemoteLightfoldDir, config.RemoteCreatedMarker)
			result = sshExecutor.Execute(markerCmd)
			if result.Error != nil || result.ExitCode != 0 {
				return config.TargetConfig{}, fmt.Errorf("failed to write created marker: %w", result.Error)
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
	} else {
		provider = strings.ToLower(providerFlag)

		fmt.Printf("Using provider: %s (from --provider flag)\n", provider)

		if provider == "existing" {
			return config.TargetConfig{}, fmt.Errorf("--provider existing is not supported with flags, use interactive configuration")
		}

		if provider == "byos" {
			if err := handleBYOSWithFlags(&targetConfig, targetName); err != nil {
				return config.TargetConfig{}, err
			}
		} else {
			if err := handleProvisionWithFlags(&targetConfig, targetName, projectPath, provider); err != nil {
				return config.TargetConfig{}, err
			}
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

// isCalledFromDeploy tracks if configureTarget is being called from deploy command
var isCalledFromDeploy bool

func configureTarget(target config.TargetConfig, targetName string, force bool) error {
	// Skip SSH configuration for container providers (e.g., fly.io)
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
	targetState, _ := state.LoadState(targetName)

	if updated, displayName, err := tryRecoverProviderIP(&target, targetName, targetState); err != nil {
		state.MarkConfigureFailed(targetName, fmt.Sprintf("Failed to recover IP for %s: %v", displayName, err))
		return fmt.Errorf("failed to recover IP for %s: %w", displayName, err)
	} else if updated {
		freshCfg, err := config.LoadConfig()
		if err == nil {
			if refreshedTarget, exists := freshCfg.GetTarget(targetName); exists {
				target = refreshedTarget
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
				needsRuntimeInstall := utils.CheckIfRuntimeNeeded(sshExecutor, projectPath)
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
		if err := executor.CleanupOldReleases(cfg.NumReleases); err != nil {
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
		return fmt.Errorf("SSH connection failed: %w", result.Error)
	}

	fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render("SSH connection validated"))

	markerCmd := fmt.Sprintf("sudo mkdir -p %s && echo 'created' | sudo tee %s/%s > /dev/null", config.RemoteLightfoldDir, config.RemoteLightfoldDir, config.RemoteCreatedMarker)
	result = sshExecutor.Execute(markerCmd)
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to write created marker: %w", result.Error)
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
	bootstrap, err := findProviderBootstrap(provider)
	if err != nil {
		return err
	}

	if imageFlag == "" {
		imageFlag = "ubuntu-22-04-x64"
	}

	_, fallbackCfg, err := bootstrap.ensureToken(targetName)
	if err != nil && fallbackCfg == nil {
		return err
	}

	if fallbackCfg != nil {
		if err := bootstrap.applyConfig(targetConfig, fallbackCfg); err != nil {
			return err
		}
	} else {
		cfgFromFlags, cfgErr := bootstrap.prepareConfigFromFlags(targetName, provisionInputs{
			Region: regionFlag,
			Size:   sizeFlag,
			Image:  imageFlag,
		})
		if cfgErr != nil {
			return cfgErr
		}
		if err := bootstrap.applyConfig(targetConfig, cfgFromFlags); err != nil {
			return err
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
				return fmt.Errorf("failed to install certbot: %w", installResult.Error)
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
		port = utils.ExtractPortFromTarget(target, target.ProjectPath)
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

	if handler, ok := providerStateHandlers[target.Provider]; ok {
		providerCfg, err := handler.cfgAccessor(&target)
		if err == nil && providerCfg != nil && providerCfg.IsProvisioned() {
			serverID := providerCfg.GetServerID()
			if serverID == "" && targetState.ProvisionedID != "" {
				serverID = targetState.ProvisionedID
			}

			if providerCfg.GetIP() == "" && serverID != "" {
				originalIP := providerCfg.GetIP()
				fmt.Printf("%s Recovering server IP from %s API...\n", labelStyle.Render("→"), handler.displayName)

				updated, _, recErr := tryRecoverProviderIP(&target, targetName, targetState)
				if recErr != nil {
					fmt.Printf("%s Failed to recover IP: %v\n", mutedStyle.Render("  ⚠"), recErr)
				} else if updated {
					if updatedCfg, loadErr := config.LoadConfig(); loadErr == nil {
						if updatedTarget, exists := updatedCfg.GetTarget(targetName); exists {
							target = updatedTarget
							if refreshedCfg, cfgErr := handler.cfgAccessor(&target); cfgErr == nil && refreshedCfg != nil {
								newIP := refreshedCfg.GetIP()
								if newIP != "" && newIP != originalIP {
									fmt.Printf("%s %s\n", successStyle.Render("  ✓"), mutedStyle.Render(fmt.Sprintf("IP updated: %s → %s", originalIP, newIP)))
									changesDetected = true
								}
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

				if targetState.LastDeploy.IsZero() {
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
	return utils.GetOrAllocatePort(target, targetName)
}

// registerAppWithServer registers an app in the server state
func registerAppWithServer(target *config.TargetConfig, targetName string, port int, framework string) error {
	return utils.RegisterAppWithServer(target, targetName, port, framework)
}

// updateServerStateFromTarget ensures server state is initialized from target config
func updateServerStateFromTarget(target *config.TargetConfig, targetName string) error {
	return utils.UpdateServerStateFromTarget(target, targetName)
}
