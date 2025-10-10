package utils

import (
	"context"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/providers"
	"lightfold/pkg/providers/flyio"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RecoverIPFromProvider is a generic function to recover IP from any provider
func RecoverIPFromProvider(target *config.TargetConfig, targetName, providerName, serverID string) error {
	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	token := tokens.GetToken(providerName)
	if token == "" {
		return fmt.Errorf("%s API token not found", providerName)
	}

	provider, err := providers.GetProvider(providerName, token)
	if err != nil {
		return fmt.Errorf("failed to get %s provider: %w", providerName, err)
	}

	server, err := provider.GetServer(context.Background(), serverID)
	if err != nil {
		return fmt.Errorf("failed to fetch server info: %w", err)
	}

	// Update provider config with recovered IP and server ID
	if err := updateProviderConfigWithIP(target, providerName, server.PublicIPv4, serverID); err != nil {
		return fmt.Errorf("failed to update provider config: %w", err)
	}

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

// updateProviderConfigWithIP updates the provider-specific config with IP and server ID
func updateProviderConfigWithIP(target *config.TargetConfig, providerName, ip, serverID string) error {
	switch providerName {
	case "digitalocean":
		doConfig, err := target.GetDigitalOceanConfig()
		if err != nil {
			return err
		}
		doConfig.IP = ip
		doConfig.DropletID = serverID
		return target.SetProviderConfig("digitalocean", doConfig)
	case "hetzner":
		hetznerConfig, err := target.GetHetznerConfig()
		if err != nil {
			return err
		}
		hetznerConfig.IP = ip
		hetznerConfig.ServerID = serverID
		return target.SetProviderConfig("hetzner", hetznerConfig)
	case "vultr":
		vultrConfig, err := target.GetVultrConfig()
		if err != nil {
			return err
		}
		vultrConfig.IP = ip
		vultrConfig.InstanceID = serverID
		return target.SetProviderConfig("vultr", vultrConfig)
	default:
		return fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// RecoverIPFromFlyio handles Fly.io-specific IP recovery with interactive fallback
func RecoverIPFromFlyio(target *config.TargetConfig, targetName, machineID string) error {
	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	flyioToken := tokens.GetToken("flyio")
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

	fmt.Printf("%s %s\n", successStyle.Render("✓"), mutedStyle.Render(fmt.Sprintf("Recovered IP: %s", ipAddress)))

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
