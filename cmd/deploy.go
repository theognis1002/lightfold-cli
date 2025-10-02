package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	tui "lightfold/cmd/ui"
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

		// Use TUI progress bar for deployment
		if err := tui.ShowDeploymentProgress(); err != nil {
			fmt.Fprintf(os.Stderr, "Error showing deployment progress: %v\n", err)
			os.Exit(1)
		}
	},
}

func validateProjectConfig(project config.ProjectConfig) error {
	switch project.Target {
	case "digitalocean":
		if project.DigitalOcean == nil {
			return fmt.Errorf("DigitalOcean configuration is missing")
		}
		if project.DigitalOcean.IP == "" {
			return fmt.Errorf("DigitalOcean IP address is required")
		}
		if project.DigitalOcean.Username == "" {
			return fmt.Errorf("DigitalOcean username is required")
		}
	case "s3":
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


func init() {
	rootCmd.AddCommand(deployCmd)
}