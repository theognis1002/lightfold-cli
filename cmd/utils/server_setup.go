package utils

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	installers "lightfold/pkg/runtime/installers"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
	"os"
	"path/filepath"
)

// SetupTargetWithExistingServer configures a target to use an existing server
func SetupTargetWithExistingServer(target *config.TargetConfig, serverIP string, userPort int) error {
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
		case "linode":
			linodeConfig := &config.LinodeConfig{
				IP:          serverIP,
				InstanceID:  serverState.ServerID,
				SSHKey:      sshKey,
				Username:    "deploy",
				Provisioned: false,
			}
			target.SetProviderConfig("linode", linodeConfig)
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

// CheckIfRuntimeNeeded determines if a runtime needs to be installed for the current app
// Returns true if the runtime is missing and needs installation
func CheckIfRuntimeNeeded(sshExecutor *sshpkg.Executor, projectPath string) bool {
	detection := detector.DetectFramework(projectPath)

	ctx := &installers.Context{
		SSH:       sshExecutor,
		Detection: &detection,
	}

	needsInstall, err := installers.RuntimeNeedsInstall(ctx)
	if err != nil {
		return true
	}
	return needsInstall
}

// UpdateServerStateFromTarget ensures server state is initialized from target config
func UpdateServerStateFromTarget(target *config.TargetConfig, targetName string) error {
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
