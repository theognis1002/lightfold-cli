package cmd

import (
	"github.com/spf13/cobra"
)


// detectCmd defines the main detection and configuration command
var detectCmd = &cobra.Command{
	Use:   "detect [PROJECT_PATH]",
	Short: "Detect framework and configure deployment",
	Long: Logo + `
Lightfold automatically identifies web frameworks and generates optimal build and deployment plans.

Supports 15+ popular frameworks including Next.js, Astro, Django, FastAPI, Express.js, and more.
Advanced package manager detection for JavaScript/TypeScript, Python, PHP, Ruby, Go, Java, C#, and Elixir.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runDetect,
}

func runDetect(cmd *cobra.Command, args []string) {
	// Just call the root command function
	runRootCommand(cmd, args)
}


func init() {
	// detectCmd is added to rootCmd in root.go
}