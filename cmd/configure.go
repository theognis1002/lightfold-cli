package cmd

import (
	"bufio"
	"fmt"
	"lightfold/pkg/config"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
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

	// Skip domain configuration for non-SSH providers (e.g., fly.io)
	// These providers handle domains differently via their own platform
	if !target.RequiresSSHDeployment() {
		return
	}

	fmt.Println()
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))

	// Check if this is a multi-app deployment
	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		fmt.Printf("\n%s\n", hintStyle.Render(fmt.Sprintf("NOTE: You can add a domain later with: lightfold domain add --target %s --domain example.com", targetName)))
		return
	}

	serverIP := providerCfg.GetIP()
	serverState, err := state.GetServerState(serverIP)
	isMultiApp := err == nil && len(serverState.DeployedApps) > 1

	// Find the port allocated to this app
	var appPort int
	if isMultiApp {
		for _, app := range serverState.DeployedApps {
			if app.TargetName == targetName {
				appPort = app.Port
				break
			}
		}
	}

	fmt.Printf("%s", promptStyle.Render("Want to add a custom domain? (y/N): "))

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	// Change the response check to skip by default
	if response == "" || response == "n" || response == "no" {
		domainHint := fmt.Sprintf("NOTE: You can add a domain later with: lightfold domain add --target %s --domain example.com", targetName)
		fmt.Printf("\n%s\n", hintStyle.Render(domainHint))

		// If multi-app, offer port-based access
		if isMultiApp && appPort > 0 {
			fmt.Println()
			fmt.Printf("%s\n", warningStyle.Render(fmt.Sprintf("ℹ Multi-app deployment detected: This server hosts %d apps", len(serverState.DeployedApps))))
			fmt.Printf("%s\n", hintStyle.Render(fmt.Sprintf("  • App '%s' is running on port %d", targetName, appPort)))
			fmt.Printf("%s\n", hintStyle.Render("  • Without a domain, only the last deployed app is accessible via http://"+serverIP))
			fmt.Println()

			// Prompt to open port
			fmt.Printf("%s", promptStyle.Render(fmt.Sprintf("Open port %d for direct access? (y/N): ", appPort)))
			portResponse, _ := reader.ReadString('\n')
			portResponse = strings.TrimSpace(strings.ToLower(portResponse))

			if portResponse == "y" || portResponse == "yes" {
				fmt.Println()
				fmt.Printf("%s\n", warningStyle.Render("⚠ SECURITY WARNING:"))
				fmt.Printf("%s\n", hintStyle.Render("  Opening application ports directly exposes your app without nginx's security layer."))
				fmt.Printf("%s\n", hintStyle.Render("  Consider using custom domains instead for better security and SSL support."))
				fmt.Println()
				fmt.Printf("%s", promptStyle.Render("Continue? (y/N): "))
				confirmResponse, _ := reader.ReadString('\n')
				confirmResponse = strings.TrimSpace(strings.ToLower(confirmResponse))

				if confirmResponse == "y" || confirmResponse == "yes" {
					if err := openPort(serverIP, appPort, providerCfg); err != nil {
						fmt.Printf("%s\n", warningStyle.Render(fmt.Sprintf("Failed to open port: %v", err)))
					} else {
						successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
						fmt.Printf("\n%s\n", successStyle.Render(fmt.Sprintf("✓ Port %d opened successfully!", appPort)))
						fmt.Printf("%s\n", hintStyle.Render(fmt.Sprintf("  Access your app at: http://%s:%d", serverIP, appPort)))
					}
				}
			} else {
				fmt.Println()
				fmt.Printf("%s\n", hintStyle.Render("  Recommended: Add a custom domain for proper multi-app routing"))
				fmt.Printf("%s\n", hintStyle.Render(fmt.Sprintf("  Run: lightfold domain add --target %s --domain your-app.example.com", targetName)))
			}
		}

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

// openPort opens a firewall port on the server via UFW
func openPort(serverIP string, port int, providerCfg config.ProviderConfig) error {
	// Create SSH executor
	sshExecutor := sshpkg.NewExecutor(serverIP, "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())

	// Open the port with UFW
	cmd := fmt.Sprintf("sudo ufw allow %d/tcp", port)
	result := sshExecutor.Execute(cmd)

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to open port: %s", result.Stderr)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(configureCmd)

	configureCmd.Flags().StringVar(&configureTargetFlag, "target", "", "Target name (defaults to current directory)")
	configureCmd.Flags().BoolVarP(&configureForceFlag, "force", "f", false, "Force reconfiguration even if already configured")
	configureCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	configureCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	configureCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during configuration")
}
