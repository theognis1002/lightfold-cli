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

		if err := validateProjectConfig(project); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid project configuration: %v\n", err)
			fmt.Println("Please run 'lightfold .' to reconfigure the project.")
			os.Exit(1)
		}

		fmt.Printf("Deploying %s project to %s...\n", project.Framework, project.Target)
		fmt.Println()

		// Create deployment orchestrator
		projectName := filepath.Base(projectPath)
		orchestrator, err := deploy.NewOrchestrator(project, projectPath, projectName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating deployment orchestrator: %v\n", err)
			os.Exit(1)
		}

		// Execute deployment with progress display
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := tui.ShowDeploymentProgressWithOrchestrator(ctx, orchestrator); err != nil {
			fmt.Fprintf(os.Stderr, "Error during deployment: %v\n", err)
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

		// For provisioned droplets, IP may be empty (will be created during deployment)
		// For BYOS, IP is required
		if !project.DigitalOcean.Provisioned && project.DigitalOcean.IP == "" {
			return fmt.Errorf("DigitalOcean IP address is required for BYOS deployments")
		}

		if project.DigitalOcean.Username == "" {
			return fmt.Errorf("DigitalOcean username is required")
		}

		if project.DigitalOcean.SSHKey == "" {
			return fmt.Errorf("DigitalOcean SSH key is required")
		}

		// For provisioned droplets, verify we have region and size
		if project.DigitalOcean.Provisioned {
			if project.DigitalOcean.Region == "" {
				return fmt.Errorf("DigitalOcean region is required for provisioned deployments")
			}
			if project.DigitalOcean.Size == "" {
				return fmt.Errorf("DigitalOcean size is required for provisioned deployments")
			}
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
