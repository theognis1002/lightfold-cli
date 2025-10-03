package cmd

import (
	"context"
	"fmt"
	tui "lightfold/cmd/ui"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure [PROJECT_PATH]",
	Short: "Configure an existing VM with your application",
	Long: `Configure an existing server with your application code and dependencies.

This command is useful for:
- Configuring a BYOS (Bring Your Own Server) VM
- Re-configuring a VM where the process has stopped
- Deploying to an existing VM without provisioning a new one

The VM must already be accessible via SSH and have basic connectivity.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		projectPath = filepath.Clean(projectPath)

		info, err := os.Stat(projectPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Cannot access path '%s': %v\n", projectPath, err)
			os.Exit(1)
		}

		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: Path '%s' is not a directory\n", projectPath)
			os.Exit(1)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		project, exists := cfg.GetProject(projectPath)

		if !exists {
			if len(cfg.Projects) == 1 && len(args) == 0 {
				for configuredPath, configuredProject := range cfg.Projects {
					fmt.Printf("No configuration found for current directory.\n")
					fmt.Printf("Using configured project: %s\n", configuredPath)
					projectPath = configuredPath
					project = configuredProject
					exists = true
					break
				}
			}

			if !exists {
				fmt.Println("No configuration found for this project.")
				if len(cfg.Projects) > 0 {
					fmt.Println("Configured projects:")
					for path := range cfg.Projects {
						fmt.Printf("  - %s\n", path)
					}
					fmt.Println("Run 'lightfold configure <PROJECT_PATH>' to configure a specific project,")
					fmt.Println("or 'lightfold <PROJECT_PATH>' to detect and configure this project.")
				} else {
					fmt.Println("Please run 'lightfold .' first to detect and configure the project.")
				}
				os.Exit(1)
			}
		}

		if err := validateConfigureProjectConfig(project); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid project configuration: %v\n", err)
			fmt.Println("Please run 'lightfold .' to reconfigure the project.")
			os.Exit(1)
		}

		// Process deployment flags (env vars, skip-build, etc.)
		if err := processDeploymentFlags(&project, projectPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing configuration options: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Configuring %s project on %s...\n", project.Framework, project.Provider)
		fmt.Println()

		// Create deployment orchestrator
		projectName := filepath.Base(projectPath)
		orchestrator, err := deploy.NewOrchestrator(project, projectPath, projectName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating orchestrator: %v\n", err)
			os.Exit(1)
		}

		// Get provider config
		var providerCfg config.ProviderConfig
		switch project.Provider {
		case "digitalocean":
			providerCfg, err = project.GetDigitalOceanConfig()
		case "hetzner":
			providerCfg, err = project.GetHetznerConfig()
		default:
			fmt.Fprintf(os.Stderr, "Error: Configure command only works with SSH-based providers (not S3)\n")
			os.Exit(1)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting provider configuration: %v\n", err)
			os.Exit(1)
		}

		// Execute configuration with progress display
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := tui.ShowConfigurationProgressWithOrchestrator(ctx, orchestrator, providerCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error during configuration: %v\n", err)
			os.Exit(1)
		}
	},
}

func validateConfigureProjectConfig(project config.ProjectConfig) error {
	// Validate provider is specified
	if project.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	// Configure only works with SSH-based providers
	if project.Provider == "s3" {
		return fmt.Errorf("configure command does not support S3 deployments")
	}

	// Validate provider-specific configuration
	switch project.Provider {
	case "digitalocean":
		doConfig, err := project.GetDigitalOceanConfig()
		if err != nil {
			return fmt.Errorf("DigitalOcean configuration is missing: %w", err)
		}

		// For configure, IP must exist (we're not provisioning)
		if doConfig.IP == "" {
			return fmt.Errorf("IP address is required. Please configure the server first with 'lightfold .'")
		}

		if doConfig.Username == "" {
			return fmt.Errorf("username is required")
		}

		if doConfig.SSHKey == "" {
			return fmt.Errorf("SSH key is required")
		}

	case "hetzner":
		hetznerConfig, err := project.GetHetznerConfig()
		if err != nil {
			return fmt.Errorf("Hetzner configuration is missing: %w", err)
		}

		if hetznerConfig.IP == "" {
			return fmt.Errorf("IP address is required. Please configure the server first with 'lightfold .'")
		}

		if hetznerConfig.Username == "" {
			return fmt.Errorf("username is required")
		}

		if hetznerConfig.SSHKey == "" {
			return fmt.Errorf("SSH key is required")
		}

	default:
		return fmt.Errorf("unknown provider: %s", project.Provider)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(configureCmd)

	// Add same deployment flags as deploy command
	configureCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	configureCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	configureCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during configuration")
}
