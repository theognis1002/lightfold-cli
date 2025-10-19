package cmd

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/detector"
	"lightfold/pkg/util"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const Version = "0.1.3"

var (
	jsonOutput      bool
	skipInteractive bool

	logoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
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

	detectionResult := detector.DetectFramework(projectPath)

	showDetectionResults(detectionResult)
}

func showDetectionResults(detection detector.Detection) {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	signalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#40BDA3"))
	successCheckStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	targetName := util.GetTargetName(cwd)

	frameworkBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#01FAC6")).
		Padding(1, 2).
		Width(60)

	var content strings.Builder
	content.WriteString(labelStyle.Render("Target: "))
	content.WriteString(valueStyle.Render(targetName))
	content.WriteString("\n")

	content.WriteString(labelStyle.Render("Language:  "))
	content.WriteString(valueStyle.Render(detection.Language))
	content.WriteString("\n")

	content.WriteString(labelStyle.Render("Framework: "))
	content.WriteString(valueStyle.Render(detection.Framework))
	content.WriteString("\n\n")

	if len(detection.Signals) > 0 {
		content.WriteString(labelStyle.Render("Detection signals:"))
		content.WriteString("\n")
		for _, signal := range detection.Signals {
			content.WriteString(successCheckStyle.Render("  ✓ "))
			content.WriteString(signalStyle.Render(signal))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	content.WriteString(labelStyle.Render("Build Commands:"))
	content.WriteString("\n")
	for i, cmd := range detection.BuildPlan {
		content.WriteString("  " + successCheckStyle.Render(fmt.Sprintf("%d.", i+1)) + " " + valueStyle.Render(cmd) + "\n")
	}
	content.WriteString("\n")

	content.WriteString(labelStyle.Render("Run Commands:"))
	content.WriteString("\n")
	for i, cmd := range detection.RunPlan {
		content.WriteString("  " + successCheckStyle.Render(fmt.Sprintf("%d.", i+1)) + " " + valueStyle.Render(cmd) + "\n")
	}

	fmt.Printf("%s\n\n", frameworkBox.Render(content.String()))

	deployStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	fmt.Printf("%s%s%s\n",
		normalStyle.Render("Run "),
		deployStyle.Render("'lightfold deploy'"),
		normalStyle.Render(" to deploy your app to a server"),
	)
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
	rootCmd.AddCommand(autoDeployCmd)

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output results as JSON (disables interactive mode)")
	rootCmd.PersistentFlags().BoolVar(&skipInteractive, "no-interactive", false, "Skip interactive prompts (for CI/automation)")
}
