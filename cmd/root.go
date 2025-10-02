package cmd

import (
	"encoding/json"
	"fmt"
	"lightfold/cmd/flags"
	"lightfold/cmd/steps"
	"lightfold/cmd/ui/detection"
	"lightfold/cmd/ui/multiInput"
	"lightfold/cmd/ui/sequential"
	"lightfold/cmd/ui/spinner"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const Version = "1.0.0"

var (
	jsonOutput      bool
	skipInteractive bool

	logoStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	tipMsgStyle    = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("190")).Italic(true)
	endingMsgStyle = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("170")).Bold(true)
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
	Run:     runRootCommand,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runRootCommand(cmd *cobra.Command, args []string) {
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

	if jsonOutput || skipInteractive || !isTerminal() {
		detectionResult := detector.DetectFramework(projectPath)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(detectionResult)
		return
	}

	fmt.Printf("%s\n", logoStyle.Render(Logo))

	spinnerProgram := tea.NewProgram(spinner.InitialModel("Detecting framework..."))

	var detectionResult detector.Detection

	// Start spinner in background
	go func() {
		if _, err := spinnerProgram.Run(); err != nil {
			// Suppress the "program was killed" error message since it's expected
			if err.Error() != "program was killed" {
				fmt.Fprintf(os.Stderr, "Error running spinner: %v\n", err)
			}
		}
	}()

	// Run detection
	detectionResult = detector.DetectFramework(projectPath)

	// Stop spinner
	spinnerProgram.Quit()

	wantsDeploy, err := detection.ShowDetectionResults(detectionResult)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error showing detection results: %v\n", err)
		os.Exit(1)
	}

	if !wantsDeploy {
		fmt.Println("Skipping deployment configuration.")
		return
	}

	if err := configureDeployment(projectPath, detectionResult); err != nil {
		fmt.Fprintf(os.Stderr, "Error configuring deployment: %v\n", err)
		os.Exit(1)
	}
}

func configureDeployment(projectPath string, detection detector.Detection) error {
	var flagTarget flags.DeploymentTarget

	stepsData := steps.InitSteps(flagTarget)

	// Step 1: Choose deployment type (BYOS vs Provision)
	deploymentTypeStep := stepsData.Steps["deployment_type"]
	deploymentType, err := multiInput.ShowMenu(deploymentTypeStep.Options, deploymentTypeStep.Headers)
	if err != nil {
		return fmt.Errorf("deployment type selection cancelled: %w", err)
	}

	var target string
	var targetChoice string

	// Step 2: Choose specific provider based on deployment type
	if strings.ToLower(deploymentType) == "byos" {
		byosStep := stepsData.Steps["byos_target"]
		targetChoice, err = multiInput.ShowMenu(byosStep.Options, byosStep.Headers)
		if err != nil {
			return fmt.Errorf("BYOS target selection cancelled: %w", err)
		}
		target = strings.ToLower(targetChoice)
	} else { // "Provision for me"
		provisionStep := stepsData.Steps["provision_target"]
		targetChoice, err = multiInput.ShowMenu(provisionStep.Options, provisionStep.Headers)
		if err != nil {
			return fmt.Errorf("provision target selection cancelled: %w", err)
		}

		target = strings.ToLower(targetChoice)
	}

	fmt.Printf("\n%s\n", endingMsgStyle.Render(fmt.Sprintf("Configuring %s deployment...", targetChoice)))

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectConfig := config.ProjectConfig{
		Framework: detection.Framework,
		Provider:  target,
	}

	projectName := sequential.GetProjectNameFromPath(projectPath)

	// Handle different flows based on deployment type
	if strings.ToLower(deploymentType) == "byos" {
		switch target {
		case "digitalocean":
			doConfig, err := sequential.RunDigitalOceanFlow(projectName)
			if err != nil {
				return fmt.Errorf("DigitalOcean configuration cancelled: %w", err)
			}
			projectConfig.SetProviderConfig("digitalocean", doConfig)

		case "custom server":
			doConfig, err := sequential.RunDigitalOceanFlow(projectName)
			if err != nil {
				return fmt.Errorf("Custom server configuration cancelled: %w", err)
			}
			projectConfig.Provider = "digitalocean" // Use DigitalOcean provider for SSH-based custom servers
			projectConfig.SetProviderConfig("digitalocean", doConfig)

		default:
			return fmt.Errorf("unsupported BYOS target: %s", target)
		}
	} else { // "Provision for me"
		switch target {
		case "digitalocean":
			doConfig, err := sequential.RunProvisionDigitalOceanFlow(projectName)
			if err != nil {
				return fmt.Errorf("DigitalOcean provisioning cancelled: %w", err)
			}
			projectConfig.SetProviderConfig("digitalocean", doConfig)

		case "s3":
			s3Config, err := sequential.RunS3Flow()
			if err != nil {
				return fmt.Errorf("S3 configuration cancelled: %w", err)
			}
			projectConfig.SetProviderConfig("s3", s3Config)

		default:
			return fmt.Errorf("unsupported provision target: %s", target)
		}
	}

	if err := cfg.SetProject(projectPath, projectConfig); err != nil {
		return fmt.Errorf("failed to set project config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n%s\n", endingMsgStyle.Render("✅ Deployment configuration saved to "+config.GetConfigPath()))
	fmt.Printf("%s\n", endingMsgStyle.Render("Run 'lightfold deploy' to deploy your application!"))

	fmt.Printf("\n%s\n", tipMsgStyle.Render("Tip: Use --json flag for CI/automation mode"))

	return nil
}

func isTerminal() bool {
	if os.Getenv("CI") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return os.Getenv("TERM") != ""
}

func init() {
	rootCmd.SetVersionTemplate("lightfold version {{.Version}}\n")

	rootCmd.AddCommand(detectCmd)

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output results as JSON (disables interactive mode)")
	rootCmd.PersistentFlags().BoolVar(&skipInteractive, "no-interactive", false, "Skip interactive prompts (for CI/automation)")
}
