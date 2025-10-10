package cmd

import (
	"bufio"
	"fmt"
	"lightfold/pkg/config"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	configureTargetFlag string
	configureForceFlag  bool
)

var configureCmd = &cobra.Command{
	Use:   "configure [PROJECT_PATH]",
	Short: "Configure an existing VM with your application",
	Long: `Configure an existing server with your application code and dependencies.

This command is idempotent - it checks if the server is already configured and skips if so.

Examples:
  lightfold configure                    # Configure current directory
  lightfold configure ~/Projects/myapp   # Configure specific project
  lightfold configure --target myapp     # Configure named target
  lightfold configure --force            # Force reconfiguration`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfigOrExit()

		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		target, targetName := resolveTarget(cfg, configureTargetFlag, pathArg)

		if err := processConfigureFlags(&target); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing configuration options: %v\n", err)
			os.Exit(1)
		}

		if err := configureTarget(target, targetName, configureForceFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Prompt for optional domain configuration
		promptDomainConfiguration(&target, targetName)

		// Remind user to push their code (only if not called from deploy command)
		if !isCalledFromDeploy {
			fmt.Println()
			hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
			mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			fmt.Printf("%s\n", hintStyle.Render("Next step:"))
			fmt.Printf("%s\n", mutedStyle.Render(fmt.Sprintf("  Run 'lightfold push --target %s' to deploy your application", targetName)))
		}
	},
}

func processConfigureFlags(target *config.TargetConfig) error {
	return target.ProcessDeploymentOptions(envFile, envVars, skipBuild)
}

func promptDomainConfiguration(target *config.TargetConfig, targetName string) {
	if target.Domain != nil && target.Domain.Domain != "" {
		return
	}

	// Skip domain configuration for non-SSH providers (e.g., Fly.io)
	// These providers handle domains differently via their own platform
	if !target.RequiresSSHDeployment() {
		return
	}

	fmt.Println()
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	fmt.Printf("%s", promptStyle.Render("Want to add a custom domain? (y/N): "))

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	// Change the response check to skip by default
	if response == "" || response == "n" || response == "no" {
		domainHint := fmt.Sprintf("NOTE: You can add a domain later with: lightfold domain add --target %s --domain example.com", targetName)
		fmt.Printf("\n%s\n", hintStyle.Render(domainHint))
		return
	}

	fmt.Printf("%s", promptStyle.Render("Domain: "))
	domain, _ := reader.ReadString('\n')
	domain = strings.TrimSpace(domain)

	// If no domain provided, skip gracefully
	if domain == "" {
		domainHint := fmt.Sprintf("💡 You can add a domain later with: lightfold domain add --target %s --domain example.com", targetName)
		fmt.Printf("\n%s\n", hintStyle.Render(domainHint))
		return
	}

	fmt.Printf("%s", promptStyle.Render("Enable SSL with Let's Encrypt? (Y/n): "))
	sslResponse, _ := reader.ReadString('\n')
	sslResponse = strings.TrimSpace(strings.ToLower(sslResponse))
	enableSSL := sslResponse != "n" && sslResponse != "no"

	fmt.Println()
	if err := configureDomainAndSSL(target, targetName, domain, enableSSL); err != nil {
		fmt.Printf("Warning: domain/SSL setup failed: %v\n", err)
		fmt.Println("You can try again with: lightfold domain add --domain", domain)
	}
}

func init() {
	rootCmd.AddCommand(configureCmd)

	configureCmd.Flags().StringVar(&configureTargetFlag, "target", "", "Target name (defaults to current directory)")
	configureCmd.Flags().BoolVarP(&configureForceFlag, "force", "f", false, "Force reconfiguration even if already configured")
	configureCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	configureCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	configureCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during configuration")
}
