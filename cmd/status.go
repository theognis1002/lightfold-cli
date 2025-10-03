package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/state"
	sshpkg "lightfold/pkg/ssh"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	statusTargetFlag string

	statusHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	statusLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	statusValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	statusMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	statusErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deployment status for targets",
	Long: `Display status information for deployment targets.

Without --target flag: Lists all configured targets with their states
With --target flag: Shows detailed status for a specific target

Examples:
  lightfold status                    # List all targets
  lightfold status --target myapp     # Detailed view`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		if statusTargetFlag == "" {
			showAllTargets(cfg)
			return
		}

		showTargetDetail(cfg, statusTargetFlag)
	},
}

func showAllTargets(cfg *config.Config) {
	if len(cfg.Targets) == 0 {
		fmt.Println(statusMutedStyle.Render("No targets configured yet."))
		fmt.Printf("\n%s\n", statusMutedStyle.Render("Create your first target:"))
		fmt.Printf("  %s\n", statusValueStyle.Render("lightfold create --target myapp --provider byos --ip YOUR_IP"))
		return
	}

	fmt.Printf("%s\n", statusHeaderStyle.Render(fmt.Sprintf("Configured Targets (%d):", len(cfg.Targets))))
	fmt.Println(statusMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))

	for targetName, target := range cfg.Targets {
		fmt.Printf("\n%s\n", statusLabelStyle.Render(targetName))

		targetState, err := state.LoadState(targetName)
		if err != nil {
			fmt.Printf("  %s\n", statusErrorStyle.Render(fmt.Sprintf("Error loading state: %v", err)))
			continue
		}

		fmt.Printf("  Provider:    %s\n", statusValueStyle.Render(target.Provider))
		fmt.Printf("  Framework:   %s\n", statusValueStyle.Render(target.Framework))

		providerCfg, _ := target.GetAnyProviderConfig()
		var ip string
		var hasIP bool
		if providerCfg != nil {
			ip = providerCfg.GetIP()
			hasIP = ip != ""
		}
		if ip != "" {
			fmt.Printf("  IP:          %s\n", statusValueStyle.Render(ip))
		}

		if !targetState.LastDeploy.IsZero() {
			fmt.Printf("  Last Deploy: %s\n", statusValueStyle.Render(targetState.LastDeploy.Format("2006-01-02 15:04")))
		}

		fmt.Printf("\n  %s\n", statusLabelStyle.Render("Pipeline:"))

		fmt.Printf("    %s\n", statusSuccessStyle.Render("✓ Detect"))

		if targetState.Created || hasIP {
			fmt.Printf("    %s\n", statusSuccessStyle.Render("✓ Create"))
		} else {
			fmt.Printf("    %s\n", statusMutedStyle.Render("[ ] Create"))
		}

		if targetState.Configured {
			fmt.Printf("    %s\n", statusSuccessStyle.Render("✓ Configure"))
		} else {
			fmt.Printf("    %s\n", statusMutedStyle.Render("[ ] Configure"))
		}

		if !targetState.LastDeploy.IsZero() {
			fmt.Printf("    %s\n", statusSuccessStyle.Render("✓ Push"))
		} else {
			fmt.Printf("    %s\n", statusMutedStyle.Render("[ ] Push"))
		}
	}

	fmt.Printf("\n%s\n", statusMutedStyle.Render("For detailed status: lightfold status --target <name>"))
}

func showTargetDetail(cfg *config.Config, targetName string) {
	target, exists := cfg.GetTarget(targetName)
	if !exists {
		fmt.Fprintf(os.Stderr, "%s\n", statusErrorStyle.Render(fmt.Sprintf("Error: Target '%s' not found", targetName)))
		os.Exit(1)
	}

	targetState, err := state.LoadState(targetName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", statusErrorStyle.Render(fmt.Sprintf("Error loading state: %v", err)))
		os.Exit(1)
	}

	fmt.Printf("%s %s\n", statusHeaderStyle.Render("Target:"), statusLabelStyle.Render(targetName))
	fmt.Printf("%s\n\n", statusMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))

	fmt.Printf("%s\n", statusHeaderStyle.Render("Configuration:"))
	fmt.Printf("  Project:   %s\n", statusValueStyle.Render(target.ProjectPath))
	fmt.Printf("  Framework: %s\n", statusValueStyle.Render(target.Framework))
	fmt.Printf("  Provider:  %s\n", statusValueStyle.Render(target.Provider))
	fmt.Println()

	fmt.Printf("%s\n", statusHeaderStyle.Render("State:"))
	if targetState.Created {
		fmt.Printf("  Created:    %s\n", statusSuccessStyle.Render("✓ Yes"))
	} else {
		fmt.Printf("  Created:    %s\n", statusMutedStyle.Render("✗ No"))
	}
	if targetState.Configured {
		fmt.Printf("  Configured: %s\n", statusSuccessStyle.Render("✓ Yes"))
	} else {
		fmt.Printf("  Configured: %s\n", statusMutedStyle.Render("✗ No"))
	}
	if targetState.ProvisionedID != "" {
		fmt.Printf("  Server ID:  %s\n", statusValueStyle.Render(targetState.ProvisionedID))
	}
	if targetState.LastCommit != "" {
		commitShort := targetState.LastCommit
		if len(commitShort) > 7 {
			commitShort = commitShort[:7]
		}
		fmt.Printf("  Last Commit: %s\n", statusValueStyle.Render(commitShort))
	}
	if !targetState.LastDeploy.IsZero() {
		fmt.Printf("  Last Deploy: %s\n", statusValueStyle.Render(targetState.LastDeploy.Format("2006-01-02 15:04:05")))
	}
	if targetState.LastRelease != "" {
		fmt.Printf("  Last Release: %s\n", statusValueStyle.Render(targetState.LastRelease))
	}
	fmt.Println()

	if targetState.Created {
		fmt.Printf("%s\n", statusHeaderStyle.Render("Server Status:"))

		if target.Provider == "s3" {
			fmt.Printf("  Type: %s\n", statusValueStyle.Render("S3 (static site)"))
			s3Config, _ := target.GetS3Config()
			fmt.Printf("  Bucket: %s\n", statusValueStyle.Render(s3Config.Bucket))
			fmt.Printf("  Region: %s\n", statusValueStyle.Render(s3Config.Region))
			fmt.Println()
			return
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Printf("  %s\n", statusErrorStyle.Render(fmt.Sprintf("Error: %v", err)))
			return
		}

		fmt.Printf("  IP:        %s\n", statusValueStyle.Render(providerCfg.GetIP()))
		fmt.Printf("  Username:  %s\n", statusValueStyle.Render(providerCfg.GetUsername()))

		if providerCfg.GetIP() != "" {
			sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
			defer sshExecutor.Disconnect()

			appName := strings.ReplaceAll(targetName, "-", "_")
			result := sshExecutor.Execute(fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo 'not-found'", appName))
			if result.ExitCode == 0 {
				status := strings.TrimSpace(result.Stdout)
				if status == "active" {
					fmt.Printf("  Service:   %s\n", statusSuccessStyle.Render("✓ Active"))
				} else if status == "not-found" {
					fmt.Printf("  Service:   %s\n", statusMutedStyle.Render("- Not configured"))
				} else {
					fmt.Printf("  Service:   %s\n", statusErrorStyle.Render(fmt.Sprintf("✗ %s", status)))
				}
			} else {
				fmt.Printf("  Service:   %s\n", statusMutedStyle.Render("? Unable to check"))
			}

			result = sshExecutor.Execute(fmt.Sprintf("readlink -f /srv/%s/current 2>/dev/null || echo 'none'", appName))
			if result.ExitCode == 0 {
				currentRelease := strings.TrimSpace(result.Stdout)
				if currentRelease != "none" && currentRelease != "" {
					releaseTimestamp := strings.TrimPrefix(currentRelease, fmt.Sprintf("/srv/%s/releases/", appName))
					fmt.Printf("  Current:   %s\n", statusValueStyle.Render(releaseTimestamp))
				} else {
					fmt.Printf("  Current:   %s\n", statusMutedStyle.Render("- No release deployed"))
				}
			}

			result = sshExecutor.Execute("df -h / | tail -1 | awk '{print $5}'")
			if result.ExitCode == 0 {
				diskUsage := strings.TrimSpace(result.Stdout)
				fmt.Printf("  Disk:      %s\n", statusValueStyle.Render(diskUsage+" used"))
			}

			result = sshExecutor.Execute("uptime -p 2>/dev/null || uptime | awk '{print $3, $4}'")
			if result.ExitCode == 0 {
				uptime := strings.TrimSpace(result.Stdout)
				fmt.Printf("  Uptime:    %s\n", statusValueStyle.Render(uptime))
			}
		}
		fmt.Println()
	}

	if !targetState.Created {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Next:"), statusValueStyle.Render(fmt.Sprintf("lightfold create --target %s", targetName)))
	} else if !targetState.Configured {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Next:"), statusValueStyle.Render(fmt.Sprintf("lightfold configure --target %s", targetName)))
	} else {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Ready to deploy:"), statusValueStyle.Render(fmt.Sprintf("lightfold push --target %s", targetName)))
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVar(&statusTargetFlag, "target", "", "Target name (optional - shows all targets if omitted)")
}
