package cmd

import (
	"fmt"
	sshpkg "lightfold/pkg/ssh"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	logsTargetFlag string
	logsLines      int
	logsTail       bool

	logsHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	logsMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

var logsCmd = &cobra.Command{
	Use:   "logs [PROJECT_PATH]",
	Short: "Fetch and display application logs from the deployment server",
	Long: `Display recent logs from the deployed application.

By default, shows the last 100 lines. Use --tail to stream logs in real-time.

Examples:
  lightfold logs                          # Show logs for current directory
  lightfold logs --target myapp           # Show logs for named target
  lightfold logs --tail                   # Stream logs in real-time
  lightfold logs --lines 50               # Show last 50 lines
  lightfold logs --target myapp -t -l 200 # Stream last 200 lines`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfigOrExit()

		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		target, targetName := resolveTarget(cfg, logsTargetFlag, pathArg)

		if target.Provider == "s3" {
			fmt.Fprintf(os.Stderr, "Error: Logs are not available for S3 deployments\n")
			os.Exit(1)
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("%s %s\n", logsHeaderStyle.Render("Logs for:"), targetName)
		fmt.Printf("%s %s\n\n", logsMutedStyle.Render("Server:"), providerCfg.GetIP())

		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		defer sshExecutor.Disconnect()

		appName := strings.ReplaceAll(targetName, "-", "_")

		var logsCmd string
		if logsTail {
			logsCmd = fmt.Sprintf("journalctl -u %s -n %d -f", appName, logsLines)
		} else {
			logsCmd = fmt.Sprintf("journalctl -u %s -n %d --no-pager", appName, logsLines)
		}

		result := sshExecutor.Execute(logsCmd)
		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "Error fetching logs: %v\n", result.Error)
			os.Exit(1)
		}

		if result.ExitCode != 0 {
			if strings.Contains(result.Stderr, "No entries") || strings.Contains(result.Stderr, "not found") {
				fmt.Printf("%s\n", logsMutedStyle.Render("No logs available yet. The service may not have been deployed."))
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Stderr)
			os.Exit(1)
		}

		fmt.Println(result.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().StringVar(&logsTargetFlag, "target", "", "Target name (defaults to current directory)")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "l", 100, "Number of lines to show")
	logsCmd.Flags().BoolVarP(&logsTail, "tail", "t", false, "Stream logs in real-time")
}
