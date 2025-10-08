package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	syncTargetFlag string
)

var syncCmd = &cobra.Command{
	Use:   "sync [PROJECT_PATH]",
	Short: "Sync local state and config with actual server state",
	Long: `Sync local configuration and state files with the actual state of the deployed server.

This command is useful when:
- Local state files are out of sync with the server
- Config files have been corrupted or deleted
- You need to recover from a bad state
- Server IP address has changed

The sync command will:
- Verify SSH connectivity to the server
- Sync remote deployment markers to local state
- Recover server IP from provider API (if needed)
- Update deployment information (current release, commit, etc.)
- Preserve user-supplied configuration (domain, env vars, etc.)

Examples:
  lightfold sync                    # Sync current directory
  lightfold sync ~/Projects/myapp   # Sync specific project
  lightfold sync --target myapp     # Sync named target`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfigOrExit()

		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		target, targetName := resolveTarget(cfg, syncTargetFlag, pathArg)

		headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
		fmt.Printf("%s %s\n\n", headerStyle.Render("Syncing target:"), valueStyle.Render(targetName))

		syncedState, err := syncTarget(target, targetName, cfg)
		if err != nil {
			errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			fmt.Fprintf(os.Stderr, "\n%s %v\n", errorStyle.Render("✗ Sync failed:"), err)
			os.Exit(1)
		}

		// Skip summary card for S3 (syncedState will be nil)
		if syncedState == nil {
			return
		}

		fmt.Println()

		// Display synced state summary in a card
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

		// Get provider config for IP
		providerCfg, _ := target.GetSSHProviderConfig()
		serverIP := ""
		if providerCfg != nil {
			serverIP = providerCfg.GetIP()
		}

		// Build summary lines
		summaryLines := []string{
			successStyle.Render(fmt.Sprintf("✓ Successfully synced '%s'", targetName)),
			"",
		}

		// Add server IP if available
		if serverIP != "" {
			summaryLines = append(summaryLines, fmt.Sprintf("%s %s", mutedStyle.Render("Server:"), valueStyle.Render(serverIP)))
		}

		// Add created/configured state
		createdStatus := "✗ No"
		if syncedState.Created {
			createdStatus = "✓ Yes"
		}
		configuredStatus := "✗ No"
		if syncedState.Configured {
			configuredStatus = "✓ Yes"
		}
		summaryLines = append(summaryLines,
			fmt.Sprintf("%s %s", mutedStyle.Render("Created:"), valueStyle.Render(createdStatus)),
			fmt.Sprintf("%s %s", mutedStyle.Render("Configured:"), valueStyle.Render(configuredStatus)),
		)

		// Add deployment info if available
		if syncedState.LastRelease != "" {
			summaryLines = append(summaryLines, fmt.Sprintf("%s %s", mutedStyle.Render("Release:"), valueStyle.Render(syncedState.LastRelease)))
		}

		if syncedState.LastCommit != "" {
			commitShort := syncedState.LastCommit
			if len(commitShort) > 7 {
				commitShort = commitShort[:7]
			}
			summaryLines = append(summaryLines, fmt.Sprintf("%s %s", mutedStyle.Render("Commit:"), valueStyle.Render(commitShort)))
		}

		if !syncedState.LastDeploy.IsZero() {
			summaryLines = append(summaryLines, fmt.Sprintf("%s %s", mutedStyle.Render("Last Deploy:"), valueStyle.Render(syncedState.LastDeploy.Format("2006-01-02 15:04"))))
		}

		successBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("82")).
			Padding(0, 1).
			Render(lipgloss.JoinVertical(lipgloss.Left, summaryLines...))

		fmt.Println(successBox)
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().StringVar(&syncTargetFlag, "target", "", "Target name to sync")
}
