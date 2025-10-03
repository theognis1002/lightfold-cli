package cmd

import (
	"encoding/json"
	"fmt"
	"lightfold/cmd/ui/detection"
	"lightfold/cmd/ui/spinner"
	"lightfold/pkg/detector"
	"os"
	"path/filepath"

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
		fmt.Println("\nSkipping deployment configuration.")
		return
	}

	// Show next steps
	targetName := filepath.Base(projectPath)
	if absPath, err := filepath.Abs(projectPath); err == nil {
		targetName = filepath.Base(absPath)
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Next Steps - Deploy your application:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("1. Create infrastructure:\n")
	fmt.Printf("   BYOS:       lightfold create --target %s --provider byos --ip YOUR_IP --ssh-key ~/.ssh/id_rsa\n", targetName)
	fmt.Printf("   Provision:  lightfold create --target %s --provider do --region nyc1 --size s-1vcpu-1gb\n", targetName)
	fmt.Println()
	fmt.Printf("2. Configure server:\n")
	fmt.Printf("   lightfold configure --target %s\n", targetName)
	fmt.Println()
	fmt.Printf("3. Deploy code:\n")
	fmt.Printf("   lightfold push --target %s\n", targetName)
	fmt.Println()
	fmt.Printf("Or use the orchestrator to run all steps:\n")
	fmt.Printf("   lightfold deploy --target %s\n", targetName)
	fmt.Println()
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
