package cmd

import (
	"fmt"
	"lightfold/pkg/deploy"
	"lightfold/pkg/detector"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/util"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	rollbackTargetFlag string
	rollbackForce      bool

	rollbackHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	rollbackSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	rollbackErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	rollbackMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	rollbackValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [PROJECT_PATH]",
	Short: "Rollback to the previous release",
	Long: `Instantly rollback to the previous release.

This command:
- Detects the previous release from /srv/<app>/releases/
- Switches the current symlink to the previous release
- Restarts the systemd service
- Verifies the rollback with a health check

Examples:
  lightfold rollback                    # Rollback current directory
  lightfold rollback --target myapp     # Rollback named target
  lightfold rollback --force            # Skip confirmation prompt`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfigOrExit()

		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		target, targetName := resolveTarget(cfg, rollbackTargetFlag, pathArg)
		projectPath := target.ProjectPath

		if target.Provider == "s3" {
			fmt.Fprintf(os.Stderr, "Error: Rollback is not supported for S3 deployments\n")
			os.Exit(1)
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if providerCfg.GetIP() == "" {
			fmt.Fprintf(os.Stderr, "Error: No server IP found in configuration\n")
			os.Exit(1)
		}

		// Confirmation prompt unless --force is used
		if !rollbackForce {
			fmt.Printf("%s\n", rollbackHeaderStyle.Render("Rollback Confirmation"))
			fmt.Printf("%s\n\n", rollbackMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
			fmt.Printf("Target:  %s\n", rollbackValueStyle.Render(targetName))
			fmt.Printf("Server:  %s\n", rollbackValueStyle.Render(providerCfg.GetIP()))
			fmt.Printf("\n%s\n", rollbackMutedStyle.Render("This will rollback to the previous release and restart the service."))
			fmt.Printf("\n%s", rollbackMutedStyle.Render("Continue? (y/N): "))

			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))

			if response != "y" && response != "yes" {
				fmt.Println(rollbackMutedStyle.Render("Rollback cancelled."))
				os.Exit(0)
			}
			fmt.Println()
		}

		fmt.Printf("%s %s\n\n", rollbackHeaderStyle.Render("Rolling back:"), targetName)

		projectName := util.GetTargetName(projectPath)

		fmt.Printf("Connecting to server at %s...\n", providerCfg.GetIP())
		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		defer sshExecutor.Disconnect()

		detection := detector.DetectFramework(target.ProjectPath)
		executor := deploy.NewExecutor(sshExecutor, projectName, projectPath, &detection)

		if err := executor.RollbackToPreviousRelease(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", rollbackErrorStyle.Render(fmt.Sprintf("✗ Rollback failed: %v", err)))
			os.Exit(1)
		}

		fmt.Printf("\n%s\n", rollbackSuccessStyle.Render("✓ Successfully rolled back to previous release"))
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)

	rollbackCmd.Flags().StringVar(&rollbackTargetFlag, "target", "", "Target name (defaults to current directory)")
	rollbackCmd.Flags().BoolVar(&rollbackForce, "force", false, "Skip confirmation prompt")
}
