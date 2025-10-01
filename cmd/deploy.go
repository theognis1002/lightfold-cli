package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/tui"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy [PROJECT_PATH]",
	Short: "Deploy your application to the configured target",
	Long: `Deploy your application using the configuration stored in ~/.lightfold/config.json.

If no configuration exists for the project, you'll be prompted to set it up first.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		// Clean and validate the path
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

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Check if project is configured
		project, exists := cfg.GetProject(projectPath)
		if !exists {
			// If no config found for current path, try to find any configured project
			if len(cfg.Projects) == 1 && len(args) == 0 {
				// If only one project is configured and no path specified, use it
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
				fmt.Println("No deployment configuration found for this project.")
				if len(cfg.Projects) > 0 {
					fmt.Println("Configured projects:")
					for path := range cfg.Projects {
						fmt.Printf("  - %s\n", path)
					}
					fmt.Println("Run 'lightfold deploy <PROJECT_PATH>' to deploy a specific project,")
					fmt.Println("or 'lightfold <PROJECT_PATH>' to configure this project.")
				} else {
					fmt.Println("Please run 'lightfold .' first to detect and configure the project.")
				}
				os.Exit(1)
			}
		}

		// Validate configuration
		if err := validateProjectConfig(project); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid project configuration: %v\n", err)
			fmt.Println("Please run 'lightfold .' to reconfigure the project.")
			os.Exit(1)
		}

		// Show deployment info
		fmt.Printf("Deploying %s project to %s...\n", project.Framework, project.Target)
		fmt.Println()

		// Check if we're in a terminal for TUI
		useInteractive := tui.IsTerminal() && !cmd.Flag("no-interactive").Changed

		if useInteractive {
			// Show progress bar
			if err := tui.ShowDeploymentProgress(); err != nil {
				// If TUI fails, fall back to non-interactive mode
				fmt.Println("Falling back to non-interactive deployment...")
				if err := performDeployment(project, projectPath); err != nil {
					fmt.Fprintf(os.Stderr, "Deployment failed: %v\n", err)
					os.Exit(1)
				}
				tui.ShowQuickSuccess()
				return
			}

			// Show success animation
			if err := tui.ShowRocketAnimation(); err != nil {
				// Fallback to simple success message
				tui.ShowQuickSuccess()
			}
		} else {
			// Non-interactive deployment (CI/automation)
			if err := performDeployment(project, projectPath); err != nil {
				fmt.Fprintf(os.Stderr, "Deployment failed: %v\n", err)
				os.Exit(1)
			}
			tui.ShowQuickSuccess()
		}
	},
}

func validateProjectConfig(project config.ProjectConfig) error {
	switch project.Target {
	case tui.TargetDigitalOcean:
		if project.DigitalOcean == nil {
			return fmt.Errorf("DigitalOcean configuration is missing")
		}
		if project.DigitalOcean.IP == "" {
			return fmt.Errorf("DigitalOcean IP address is required")
		}
		if project.DigitalOcean.Username == "" {
			return fmt.Errorf("DigitalOcean username is required")
		}
	case tui.TargetS3:
		if project.S3 == nil {
			return fmt.Errorf("S3 configuration is missing")
		}
		if project.S3.Bucket == "" {
			return fmt.Errorf("S3 bucket name is required")
		}
	default:
		return fmt.Errorf("unknown deployment target: %s", project.Target)
	}
	return nil
}

func performDeployment(project config.ProjectConfig, projectPath string) error {
	// This is a stub implementation
	// In a real implementation, this would:
	// 1. Build the project using the framework's build commands
	// 2. Upload files to the target (DigitalOcean/S3)
	// 3. Configure the deployment environment
	// 4. Start/restart services

	switch project.Target {
	case tui.TargetDigitalOcean:
		return deployToDigitalOcean(project.DigitalOcean, projectPath)
	case tui.TargetS3:
		return deployToS3(project.S3, projectPath)
	default:
		return fmt.Errorf("unsupported deployment target: %s", project.Target)
	}
}

func deployToDigitalOcean(config *config.DigitalOceanConfig, projectPath string) error {
	// Stub implementation for DigitalOcean deployment
	fmt.Printf("Connecting to %s@%s...\n", config.Username, config.IP)
	fmt.Printf("Using SSH key: %s\n", config.SSHKey)
	fmt.Println("Building and uploading project files...")
	fmt.Println("Configuring server environment...")
	fmt.Println("Starting application services...")
	return nil
}

func deployToS3(config *config.S3Config, projectPath string) error {
	// Stub implementation for S3 deployment
	fmt.Printf("Uploading to S3 bucket: %s\n", config.Bucket)
	fmt.Printf("Region: %s\n", config.Region)
	fmt.Println("Building static site...")
	fmt.Println("Uploading files to S3...")
	fmt.Println("Configuring CloudFront distribution...")
	return nil
}

func init() {
	rootCmd.AddCommand(deployCmd)
}