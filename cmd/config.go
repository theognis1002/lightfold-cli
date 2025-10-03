package cmd

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/config"
	"os"
	"path/filepath"
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
				"projects": cfg.Projects,
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

		fmt.Println(configStyle.Render("Configured Projects:"))
		if len(cfg.Projects) == 0 {
			fmt.Println(configMutedStyle.Render("  No projects configured yet"))
		} else {
			for path, project := range cfg.Projects {
				fmt.Printf("\n  %s\n", configLabelStyle.Render(path))
				fmt.Printf("    Framework: %s\n", configValueStyle.Render(project.Framework))
				fmt.Printf("    Provider:  %s\n", configValueStyle.Render(project.Provider))

				// Show provider-specific details
				switch project.Provider {
				case "digitalocean":
					if doConfig, err := project.GetDigitalOceanConfig(); err == nil {
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
					if hConfig, err := project.GetHetznerConfig(); err == nil {
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
					if s3Config, err := project.GetS3Config(); err == nil {
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
		fmt.Println()
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show [PROJECT_PATH]",
	Short: "Show detailed configuration for a specific project",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		project, exists := cfg.GetProject(projectPath)
		if !exists {
			absPath, _ := filepath.Abs(projectPath)
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("No configuration found for: %s", absPath)))
			os.Exit(1)
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(project)
			return
		}

		absPath, _ := filepath.Abs(projectPath)
		fmt.Println(configStyle.Render(fmt.Sprintf("Configuration for: %s", absPath)))
		fmt.Println()

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(project)
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
			// Prompt for token securely (no echo)
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

		// Mask token for security
		maskedToken := maskToken(token)
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

		// Confirmation prompt
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

var configDeleteProjectCmd = &cobra.Command{
	Use:   "delete-project [PROJECT_PATH]",
	Short: "Remove project configuration",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		absPath, err := filepath.Abs(projectPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error resolving path: %v", err)))
			os.Exit(1)
		}

		if _, exists := cfg.Projects[absPath]; !exists {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("No configuration found for: %s", absPath)))
			os.Exit(1)
		}

		// Confirmation prompt
		fmt.Printf("Delete configuration for %s? (y/N): ", absPath)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Cancelled")
			return
		}

		delete(cfg.Projects, absPath)

		if err := cfg.SaveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Printf("%s\n", configSuccessStyle.Render("✓ Project configuration deleted successfully"))
	},
}

var configUpdateProjectCmd = &cobra.Command{
	Use:   "update-project [PROJECT_PATH]",
	Short: "Update specific project settings",
	Long: `Interactively update project configuration settings.

This command allows you to modify provider-specific settings such as:
- IP address (for BYOS deployments)
- SSH username
- Region/location
- Server size/type
- And more based on your provider`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading config: %v", err)))
			os.Exit(1)
		}

		project, exists := cfg.GetProject(projectPath)
		if !exists {
			absPath, _ := filepath.Abs(projectPath)
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("No configuration found for: %s", absPath)))
			os.Exit(1)
		}

		absPath, _ := filepath.Abs(projectPath)
		fmt.Println(configStyle.Render(fmt.Sprintf("Update configuration for: %s", absPath)))
		fmt.Printf("Provider: %s\n\n", configValueStyle.Render(project.Provider))

		updated := false

		switch project.Provider {
		case "digitalocean":
			doConfig, err := project.GetDigitalOceanConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading DigitalOcean config: %v", err)))
				os.Exit(1)
			}

			if newValue := promptUpdate("IP Address", doConfig.IP); newValue != "" {
				doConfig.IP = newValue
				updated = true
			}

			if newValue := promptUpdate("Username", doConfig.Username); newValue != "" {
				doConfig.Username = newValue
				updated = true
			}

			if newValue := promptUpdate("SSH Key Path", doConfig.SSHKey); newValue != "" {
				doConfig.SSHKey = newValue
				updated = true
			}

			if doConfig.Provisioned {
				if newValue := promptUpdate("Region", doConfig.Region); newValue != "" {
					doConfig.Region = newValue
					updated = true
				}

				if newValue := promptUpdate("Size", doConfig.Size); newValue != "" {
					doConfig.Size = newValue
					updated = true
				}
			}

			if updated {
				project.SetProviderConfig("digitalocean", doConfig)
			}

		case "hetzner":
			hConfig, err := project.GetHetznerConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading Hetzner config: %v", err)))
				os.Exit(1)
			}

			if newValue := promptUpdate("IP Address", hConfig.IP); newValue != "" {
				hConfig.IP = newValue
				updated = true
			}

			if newValue := promptUpdate("Username", hConfig.Username); newValue != "" {
				hConfig.Username = newValue
				updated = true
			}

			if newValue := promptUpdate("SSH Key Path", hConfig.SSHKey); newValue != "" {
				hConfig.SSHKey = newValue
				updated = true
			}

			if hConfig.Provisioned {
				if newValue := promptUpdate("Location", hConfig.Location); newValue != "" {
					hConfig.Location = newValue
					updated = true
				}

				if newValue := promptUpdate("Server Type", hConfig.ServerType); newValue != "" {
					hConfig.ServerType = newValue
					updated = true
				}
			}

			if updated {
				project.SetProviderConfig("hetzner", hConfig)
			}

		case "s3":
			s3Config, err := project.GetS3Config()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error loading S3 config: %v", err)))
				os.Exit(1)
			}

			if newValue := promptUpdate("Bucket", s3Config.Bucket); newValue != "" {
				s3Config.Bucket = newValue
				updated = true
			}

			if newValue := promptUpdate("Region", s3Config.Region); newValue != "" {
				s3Config.Region = newValue
				updated = true
			}

			if updated {
				project.SetProviderConfig("s3", s3Config)
			}

		default:
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Unsupported provider: %s", project.Provider)))
			os.Exit(1)
		}

		if !updated {
			fmt.Println("No changes made")
			return
		}

		if err := cfg.SetProject(projectPath, project); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error updating project: %v", err)))
			os.Exit(1)
		}

		if err := cfg.SaveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", configErrorStyle.Render(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Printf("\n%s\n", configSuccessStyle.Render("✓ Project configuration updated successfully"))
	},
}

// Helper function to mask tokens for display
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}

	// Show first 3 and last 3 characters
	prefix := token[:3]
	suffix := token[len(token)-3:]
	masked := prefix + strings.Repeat("*", 15) + suffix

	return masked
}

// Helper function to prompt for updates
func promptUpdate(field, currentValue string) string {
	if currentValue == "" {
		currentValue = configMutedStyle.Render("(not set)")
	} else {
		currentValue = configValueStyle.Render(currentValue)
	}

	fmt.Printf("%s [%s]: ", configLabelStyle.Render(field), currentValue)

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)

	return input
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetTokenCmd)
	configCmd.AddCommand(configGetTokenCmd)
	configCmd.AddCommand(configDeleteTokenCmd)
	configCmd.AddCommand(configDeleteProjectCmd)
	configCmd.AddCommand(configUpdateProjectCmd)
}
