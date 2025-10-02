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
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const Version = "1.0.0"

var (
	// Flags
	jsonOutput    bool
	skipInteractive bool

	// Styles
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

	// Handle JSON output mode
	if jsonOutput || skipInteractive || !isTerminal() {
		detectionResult := detector.DetectFramework(projectPath)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(detectionResult)
		return
	}

	// Interactive mode with TUI
	fmt.Printf("%s\n", logoStyle.Render(Logo))

	// Show spinner during detection
	spinnerProgram := tea.NewProgram(spinner.InitialModel("Detecting framework..."))

	var wg sync.WaitGroup
	var detectionResult detector.Detection

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := spinnerProgram.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running spinner: %v\n", err)
		}
	}()

	// Perform detection
	detectionResult = detector.DetectFramework(projectPath)

	// Stop spinner
	spinnerProgram.Kill()
	wg.Wait()

	// Show detection results and get user confirmation
	wantsDeploy, err := detection.ShowDetectionResults(detectionResult)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error showing detection results: %v\n", err)
		os.Exit(1)
	}

	if !wantsDeploy {
		fmt.Println("Skipping deployment configuration.")
		return
	}

	// Run deployment configuration flow
	if err := configureDeployment(projectPath, detectionResult); err != nil {
		fmt.Fprintf(os.Stderr, "Error configuring deployment: %v\n", err)
		os.Exit(1)
	}
}

func configureDeployment(projectPath string, detection detector.Detection) error {
	var flagTarget flags.DeploymentTarget

	// Initialize steps
	stepsData := steps.InitSteps(flagTarget)
	step := stepsData.Steps["target"]

	// Show deployment target selection menu
	targetChoice, err := multiInput.ShowMenu(step.Options, step.Headers)
	if err != nil {
		return fmt.Errorf("deployment selection cancelled: %w", err)
	}

	target := strings.ToLower(targetChoice)
	fmt.Printf("\n%s\n", endingMsgStyle.Render(fmt.Sprintf("Configuring %s deployment...", targetChoice)))

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

	// Get project name for SSH key naming
	projectName := sequential.GetProjectNameFromPath(projectPath)

	// Get target-specific configuration using sequential flow
	switch target {
	case "digitalocean":
		doConfig, err := sequential.RunDigitalOceanFlow(projectName)
		if err != nil {
			return fmt.Errorf("DigitalOcean configuration cancelled: %w", err)
		}
		projectConfig.DigitalOcean = doConfig

	case "s3":
		s3Config, err := sequential.RunS3Flow()
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

	fmt.Printf("\n%s\n", endingMsgStyle.Render("✅ Deployment configuration saved to "+config.GetConfigPath()))
	fmt.Printf("%s\n", endingMsgStyle.Render("Run 'lightfold deploy' to deploy your application!"))

	// Show tip for non-interactive command
	fmt.Printf("\n%s\n", tipMsgStyle.Render("Tip: Use --json flag for CI/automation mode"))

	return nil
}

// Helper function to check if we're in a terminal (for testing/CI)
func isTerminal() bool {
	// Check if it's a terminal and has a TERM environment variable
	// Also check that we're not in a non-interactive environment
	if os.Getenv("CI") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return os.Getenv("TERM") != ""
}

func init() {
	rootCmd.SetVersionTemplate("lightfold version {{.Version}}\n")

	// Add commands
	rootCmd.AddCommand(detectCmd)
	// deployCmd is already defined in deploy.go

	// Add flags
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output results as JSON (disables interactive mode)")
	rootCmd.PersistentFlags().BoolVar(&skipInteractive, "no-interactive", false, "Skip interactive prompts (for CI/automation)")
}