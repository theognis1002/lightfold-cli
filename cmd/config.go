package cmd

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/config"
	"os"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	configStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	configLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	configValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	configMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	configErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	configSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Lightfold configuration",
	Long:  `Manage project configurations and global provider credentials.`,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured projects and global settings",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		tokens, err := config.LoadTokens()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading tokens: %v", err)))
			os.Exit(1)
		}

		if jsonOutput {
			output := map[string]interface{}{
				"targets": cfg.Targets,
				"providers": func() map[string]bool {
					result := make(map[string]bool)
					for provider := range tokens.Tokens {
						result[provider] = true
					}
					return result
				}(),
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(output)
			return
		}

		fmt.Println(configStyle.Render("Configured Targets:"))
		if len(cfg.Targets) == 0 {
			fmt.Println(configMutedStyle.Render("  No targets configured yet"))
		} else {
			for name, target := range cfg.Targets {
				fmt.Printf("\n  %s\n", configLabelStyle.Render(name))
				fmt.Printf("    Project:   %s\n", configValueStyle.Render(target.ProjectPath))
				fmt.Printf("    Framework: %s\n", configValueStyle.Render(target.Framework))
				fmt.Printf("    Provider:  %s\n", configValueStyle.Render(target.Provider))

				switch target.Provider {
				case "digitalocean":
					if doConfig, err := target.GetDigitalOceanConfig(); err == nil {
						if doConfig.IP != "" {
							fmt.Printf("    IP:        %s\n", configValueStyle.Render(doConfig.IP))
						}
						if doConfig.Region != "" {
							fmt.Printf("    Region:    %s\n", configValueStyle.Render(doConfig.Region))
						}
						if doConfig.Provisioned {
							fmt.Printf("    Status:    %s\n", configValueStyle.Render("Provisioned"))
						}
					}
				case "hetzner":
					if hConfig, err := target.GetHetznerConfig(); err == nil {
						if hConfig.IP != "" {
							fmt.Printf("    IP:        %s\n", configValueStyle.Render(hConfig.IP))
						}
						if hConfig.Location != "" {
							fmt.Printf("    Location:  %s\n", configValueStyle.Render(hConfig.Location))
						}
						if hConfig.Provisioned {
							fmt.Printf("    Status:    %s\n", configValueStyle.Render("Provisioned"))
						}
					}
				case "s3":
					if s3Config, err := target.GetS3Config(); err == nil {
						fmt.Printf("    Bucket:    %s\n", configValueStyle.Render(s3Config.Bucket))
						fmt.Printf("    Region:    %s\n", configValueStyle.Render(s3Config.Region))
					}
				}
			}
		}

		fmt.Printf("\n%s\n", configStyle.Render("Provider Tokens:"))
		if len(tokens.Tokens) == 0 {
			fmt.Println(configMutedStyle.Render("  No provider tokens configured"))
		} else {
			for provider := range tokens.Tokens {
				fmt.Printf("  %s %s\n", configLabelStyle.Render("•"), configValueStyle.Render(provider))
			}
		}

		fmt.Printf("\n%s\n", configStyle.Render("Global Settings:"))
		fmt.Printf("  %s: %s\n", configLabelStyle.Render("Keep Releases"), configValueStyle.Render(fmt.Sprintf("%d", cfg.KeepReleases)))
		fmt.Println()
	},
}


var configSetTokenCmd = &cobra.Command{
	Use:   "set-token <provider> [token]",
	Short: "Set or update provider API token",
	Long: `Set or update API token for a cloud provider.

Supported providers: digitalocean, hetzner, s3

If token is not provided as an argument, you will be prompted to enter it securely.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		provider := strings.ToLower(args[0])
		var token string

		if len(args) == 2 {
			token = args[1]
		} else {
			fmt.Printf("Enter API token for %s: ", provider)
			tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error reading token: %v", err)))
				os.Exit(1)
			}
			token = string(tokenBytes)
		}

		if strings.TrimSpace(token) == "" {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render("Token cannot be empty"))
			os.Exit(1)
		}

		tokens, err := config.LoadTokens()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading tokens: %v", err)))
			os.Exit(1)
		}

		tokens.SetToken(provider, token)

		if err := tokens.SaveTokens(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error saving tokens: %v", err)))
			os.Exit(1)
		}

		fmt.Printf("%s\n", configSuccessStyle.Render(fmt.Sprintf("✓ Token for '%s' saved successfully", provider)))
	},
}

var configGetTokenCmd = &cobra.Command{
	Use:   "get-token <provider>",
	Short: "Display provider token (masked)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		provider := strings.ToLower(args[0])

		tokens, err := config.LoadTokens()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading tokens: %v", err)))
			os.Exit(1)
		}

		token := tokens.GetToken(provider)
		if token == "" {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("No token found for provider: %s", provider)))
			os.Exit(1)
		}

		maskedToken := "****" + token[len(token)-4:]
		if len(token) < 10 {
			maskedToken = "********"
		}
		fmt.Printf("%s: %s\n", configLabelStyle.Render(provider), configValueStyle.Render(maskedToken))
	},
}

var configDeleteTokenCmd = &cobra.Command{
	Use:   "delete-token <provider>",
	Short: "Remove provider API token",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		provider := strings.ToLower(args[0])

		tokens, err := config.LoadTokens()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading tokens: %v", err)))
			os.Exit(1)
		}

		if !tokens.HasToken(provider) {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("No token found for provider: %s", provider)))
			os.Exit(1)
		}

		fmt.Printf("Delete token for %s? (y/N): ", provider)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Cancelled")
			return
		}

		delete(tokens.Tokens, provider)

		if err := tokens.SaveTokens(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error saving tokens: %v", err)))
			os.Exit(1)
		}

		fmt.Printf("%s\n", configSuccessStyle.Render(fmt.Sprintf("✓ Token for '%s' deleted successfully", provider)))
	},
}

var configSetKeepReleasesCmd = &cobra.Command{
	Use:   "set-keep-releases <count>",
	Short: "Set the number of releases to keep during cleanup",
	Long: `Set the number of releases to keep during cleanup (default: 5).

Old releases beyond this count will be automatically deleted after each deployment.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var count int
		if _, err := fmt.Sscanf(args[0], "%d", &count); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render("Invalid count: must be a positive integer"))
			os.Exit(1)
		}

		if count < 1 {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render("Count must be at least 1"))
			os.Exit(1)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		cfg.KeepReleases = count

		if err := cfg.SaveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Printf("%s\n", configSuccessStyle.Render(fmt.Sprintf("✓ Keep releases set to %d", count)))
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configSetTokenCmd)
	configCmd.AddCommand(configGetTokenCmd)
	configCmd.AddCommand(configDeleteTokenCmd)
	configCmd.AddCommand(configSetKeepReleasesCmd)
}
