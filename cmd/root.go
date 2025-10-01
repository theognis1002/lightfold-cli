package cmd

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	"lightfold/pkg/tui"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const Version = "1.0.0"

var (
	// Flags
	jsonOutput    bool
	skipInteractive bool
)

const Logo = `
██╗     ██╗ ██████╗ ██╗  ██╗████████╗███████╗ ██████╗ ██╗     ██████╗
██║     ██║██╔════╝ ██║  ██║╚══██╔══╝██╔════╝██╔═══██╗██║     ██╔══██╗
██║     ██║██║  ███╗███████║   ██║   █████╗  ██║   ██║██║     ██║  ██║
██║     ██║██║   ██║██╔══██║   ██║   ██╔══╝  ██║   ██║██║     ██║  ██║
███████╗██║╚██████╔╝██║  ██║   ██║   ██║     ╚██████╔╝███████╗██████╔╝
╚══════╝╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚═╝      ╚═════╝ ╚══════╝╚═════╝
`

var rootCmd = &cobra.Command{
	Use:   "lightfold [PROJECT_PATH]",
	Short: "A fast, intelligent project framework detector",
	Long: Logo + `
Lightfold automatically identifies web frameworks and generates optimal build and deployment plans.

Supports 15+ popular frameworks including Next.js, Astro, Django, FastAPI, Express.js, and more.
Advanced package manager detection for JavaScript/TypeScript, Python, PHP, Ruby, Go, Java, C#, and Elixir.`,
	Version: Version,
	Args:    cobra.MaximumNArgs(1),
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

		// Perform framework detection
		detection := detector.DetectFramework(projectPath)

		// Handle output based on flags
		if jsonOutput || skipInteractive || !tui.IsTerminal() {
			// JSON output for CI/automation
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(detection)
			return
		}

		// Interactive mode: show detection results and configure deployment
		showDetectionResults(detection)

		// Ask user if they want to configure deployment
		fmt.Println()
		fmt.Print("Would you like to configure deployment for this project? (y/N): ")
		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
			fmt.Println("Skipping deployment configuration.")
			return
		}

		// Run deployment configuration flow
		if err := configureDeployment(projectPath, detection); err != nil {
			fmt.Fprintf(os.Stderr, "Error configuring deployment: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func showDetectionResults(detection detector.Detection) {
	fmt.Printf("🔍 Framework detected: %s\n", detection.Framework)
	fmt.Printf("📝 Language: %s\n", detection.Language)
	fmt.Printf("⚡ Confidence: %.1f%%\n", detection.Confidence*100)

	if len(detection.Signals) > 0 {
		fmt.Println("🔧 Detection signals:")
		for _, signal := range detection.Signals {
			fmt.Printf("  • %s\n", signal)
		}
	}

	if len(detection.BuildPlan) > 0 {
		fmt.Println("🏗️  Build plan:")
		for i, cmd := range detection.BuildPlan {
			fmt.Printf("  %d. %s\n", i+1, cmd)
		}
	}

	if len(detection.RunPlan) > 0 {
		fmt.Println("🚀 Run plan:")
		for i, cmd := range detection.RunPlan {
			fmt.Printf("  %d. %s\n", i+1, cmd)
		}
	}
}

func configureDeployment(projectPath string, detection detector.Detection) error {
	// Show deployment target selection menu
	target, err := tui.ShowDeploymentMenu()
	if err != nil {
		return fmt.Errorf("deployment selection cancelled: %w", err)
	}

	fmt.Printf("\nConfiguring %s deployment...\n", target)

	// Load or create config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create project config
	projectConfig := config.ProjectConfig{
		Framework: detection.Framework,
		Target:    target,
	}

	// Get target-specific configuration
	switch target {
	case tui.TargetDigitalOcean:
		doConfig, err := tui.ShowDigitalOceanInputs()
		if err != nil {
			return fmt.Errorf("DigitalOcean configuration cancelled: %w", err)
		}
		projectConfig.DigitalOcean = doConfig

	case tui.TargetS3:
		s3Config, err := tui.ShowS3Inputs()
		if err != nil {
			return fmt.Errorf("S3 configuration cancelled: %w", err)
		}
		projectConfig.S3 = s3Config

	default:
		return fmt.Errorf("unsupported deployment target: %s", target)
	}

	// Save configuration
	if err := cfg.SetProject(projectPath, projectConfig); err != nil {
		return fmt.Errorf("failed to set project config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n✅ Deployment configuration saved to %s\n", config.GetConfigPath())
	fmt.Println("Run 'lightfold deploy' to deploy your application!")

	return nil
}

func init() {
	rootCmd.SetVersionTemplate("lightfold version {{.Version}}\n")

	// Add flags
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output results as JSON (disables interactive mode)")
	rootCmd.PersistentFlags().BoolVar(&skipInteractive, "no-interactive", false, "Skip interactive prompts (for CI/automation)")
}