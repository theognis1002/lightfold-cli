package cmd

import (
	"fmt"
	"lightfold/pkg/config"

	"github.com/charmbracelet/lipgloss"
)

// displayProviderSummary shows a formatted summary of the selected provider configuration
func displayProviderSummary(provider string, providerConfig interface{}) {
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	displayName := getProviderDisplayName(provider)
	fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Cloud Provider: %s", displayName)))

	switch provider {
	case "digitalocean":
		if cfg, ok := providerConfig.(*config.DigitalOceanConfig); ok {
			regionDisplay := getDORegionDisplayName(cfg.Region)
			sizeDisplay := getDOSizeDisplayName(cfg.Size)
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("DigitalOcean Region: %s - %s", cfg.Region, regionDisplay)))
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Droplet Size: %s - %s", cfg.Size, sizeDisplay)))
		}
	case "vultr":
		if cfg, ok := providerConfig.(*config.VultrConfig); ok {
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Region: %s", cfg.Region)))
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Plan: %s", cfg.Plan)))
		}
	case "hetzner":
		if cfg, ok := providerConfig.(*config.HetznerConfig); ok {
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Location: %s", cfg.Location)))
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Server Type: %s", cfg.ServerType)))
		}
	case "flyio":
		if cfg, ok := providerConfig.(*config.FlyioConfig); ok {
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Region: %s", cfg.Region)))
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Machine Size: %s", cfg.Size)))
		}
	case "byos":
		if cfg, ok := providerConfig.(*config.DigitalOceanConfig); ok {
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Server IP: %s", cfg.IP)))
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("User: %s", cfg.Username)))
		}
	case "existing":
		if configMap, ok := providerConfig.(map[string]string); ok {
			serverIP := configMap["server_ip"]
			port := configMap["port"]
			fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Server IP: %s", serverIP)))
			if port != "" {
				fmt.Printf("  %s\n", mutedStyle.Render(fmt.Sprintf("Port: %s", port)))
			}
		}
	}
}

// getProviderDisplayName returns the friendly display name for a provider
// Fallback mapping consistent with provider implementations
func getProviderDisplayName(provider string) string {
	names := map[string]string{
		"digitalocean": "DigitalOcean",
		"vultr":        "Vultr",
		"hetzner":      "Hetzner Cloud",
		"flyio":        "fly.io",
		"byos":         "BYOS",
		"existing":     "Existing server",
	}
	if name, ok := names[provider]; ok {
		return name
	}
	return provider
}

// getDORegionDisplayName returns the friendly name for a DigitalOcean region
// Maps are consistent with cmd/ui/sequential/step.go CreateRegionStep
func getDORegionDisplayName(region string) string {
	names := map[string]string{
		"nyc1": "New York City 1",
		"nyc3": "New York City 3",
		"ams3": "Amsterdam 3",
		"sfo3": "San Francisco 3",
		"sgp1": "Singapore 1",
		"lon1": "London 1",
		"fra1": "Frankfurt 1",
		"tor1": "Toronto 1",
		"blr1": "Bangalore 1",
		"syd1": "Sydney 1",
	}
	if name, ok := names[region]; ok {
		return name
	}
	return region
}

// getDOSizeDisplayName returns the friendly name for a DigitalOcean droplet size
// Maps are consistent with cmd/ui/sequential/step.go CreateSizeStep
func getDOSizeDisplayName(size string) string {
	names := map[string]string{
		"s-1vcpu-512mb-10gb": "512 MB RAM, 1 vCPU, 10 GB SSD",
		"s-1vcpu-1gb":        "1 GB RAM, 1 vCPU, 25 GB SSD",
		"s-1vcpu-2gb":        "2 GB RAM, 1 vCPU, 50 GB SSD",
		"s-2vcpu-2gb":        "2 GB RAM, 2 vCPUs, 60 GB SSD",
		"s-2vcpu-4gb":        "4 GB RAM, 2 vCPUs, 80 GB SSD",
		"s-4vcpu-8gb":        "8 GB RAM, 4 vCPUs, 160 GB SSD",
		"s-6vcpu-16gb":       "16 GB RAM, 6 vCPUs, 320 GB SSD",
		"s-8vcpu-32gb":       "32 GB RAM, 8 vCPUs, 640 GB SSD",
	}
	if name, ok := names[size]; ok {
		return name
	}
	return size
}
