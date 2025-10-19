package cmd

import (
	"bufio"
	"context"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/firewall"
	"lightfold/pkg/providers"
	_ "lightfold/pkg/providers/digitalocean"
	_ "lightfold/pkg/providers/flyio"
	_ "lightfold/pkg/providers/hetzner"
	"lightfold/pkg/runtime"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
	"lightfold/pkg/util"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	destroyTargetFlag string

	destroyWarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	destroyDangerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	destroyMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Match bubbletea descStyle
	destroySuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy VM and remove local configuration for a target",
	Long: `Permanently destroy the provisioned VM (if any) and remove all local configuration
and state for a deployment target.

⚠️  WARNING: This action is irreversible!

This command will:
  • Delete the provisioned VM from your cloud provider (if provisioned)
  • Remove the target configuration from ~/.lightfold/config.json
  • Remove the target state from ~/.lightfold/state/<target>.json
  • Preserve API tokens (shared across targets)

For safety, you must type the exact target name to confirm destruction.

Examples:
  lightfold destroy --target myapp-prod`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if destroyTargetFlag == "" {
			fmt.Fprintf(os.Stderr, "Error: --target flag is required\n")
			os.Exit(1)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		target, exists := cfg.GetTarget(destroyTargetFlag)
		if !exists {
			fmt.Println(destroyMutedStyle.Render(fmt.Sprintf("Target '%s' not found in configuration.", destroyTargetFlag)))

			targetState, _ := state.LoadState(destroyTargetFlag)
			if targetState != nil && (targetState.Created || targetState.Configured) {
				fmt.Println(destroyMutedStyle.Render("However, state file exists. Cleaning up state..."))
				if err := state.DeleteState(destroyTargetFlag); err != nil {
					fmt.Fprintf(os.Stderr, "Error deleting state: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render(fmt.Sprintf("Deleted state for target '%s'", destroyTargetFlag)))
			}
			return
		}

		provisionedID := state.GetProvisionedID(destroyTargetFlag)
		providerCfg, _ := target.GetAnyProviderConfig()

		fmt.Printf("\n%s\n", destroyWarningStyle.Render("⚠️  WARNING: This will permanently destroy the following:"))
		fmt.Println()

		if provisionedID != "" && providerCfg != nil {
			fmt.Printf("  %s VM: %s", destroyDangerStyle.Render("•"), provisionedID)
			if providerCfg.GetIP() != "" {
				fmt.Printf(" (%s)", providerCfg.GetIP())
			}
			fmt.Printf(" [Provider: %s]\n", target.Provider)
		} else if providerCfg != nil && providerCfg.GetIP() != "" {
			fmt.Printf("  %s BYOS server (IP: %s) - local config only\n", destroyMutedStyle.Render("•"), providerCfg.GetIP())
		}

		fmt.Println()
		fmt.Printf("%s\n\n", destroyDangerStyle.Render("This action cannot be undone!"))

		targetBaseName := util.GetTargetName(destroyTargetFlag)
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Type the target name '%s' to confirm: ", targetBaseName)
		confirmation, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading confirmation: %v\n", err)
			os.Exit(1)
		}

		confirmation = strings.TrimSpace(confirmation)
		if confirmation != targetBaseName {
			fmt.Println("\nCancelled. Target name did not match.")
			os.Exit(0)
		}

		fmt.Println()

		// Only destroy if: (1) VM was provisioned by this target AND (2) no other apps on the server
		shouldDestroyVM := false
		var otherApps []state.DeployedApp

		if provisionedID != "" && target.Provider != "" && target.Provider != "byos" {
			targetProvisionedVM := false
			if providerCfg != nil {
				targetProvisionedVM = providerCfg.IsProvisioned()
			}

			if target.ServerIP != "" {
				serverState, err := state.GetServerState(target.ServerIP)
				if err == nil {
					for _, app := range serverState.DeployedApps {
						if app.TargetName != destroyTargetFlag {
							otherApps = append(otherApps, app)
						}
					}
				}
			}

			if !targetProvisionedVM {
				shouldDestroyVM = false
			} else if len(otherApps) > 0 {
				shouldDestroyVM = false
				fmt.Printf("%s %s\n", destroyWarningStyle.Render("⚠"), destroyWarningStyle.Render("VM will NOT be destroyed - other apps are deployed to this server:"))
				for _, app := range otherApps {
					fmt.Printf("  %s %s (port %d)\n", destroyMutedStyle.Render("•"), app.AppName, app.Port)
				}
				fmt.Println()
				fmt.Printf("%s %s\n", destroyMutedStyle.Render("ℹ"), destroyMutedStyle.Render("Only removing this app's configuration and local state"))
				fmt.Println()
			} else {
				shouldDestroyVM = true
			}
		}

		if shouldDestroyVM {
			fmt.Printf("%s %s\n", destroyWarningStyle.Render("→"), destroyMutedStyle.Render("Destroying VM..."))

			tokens, err := config.LoadTokens()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading tokens: %v\n", err)
				os.Exit(1)
			}

			token := tokens.GetToken(target.Provider)
			if token == "" {
				fmt.Fprintf(os.Stderr, "\n%s %s\n",
					destroyDangerStyle.Render("✗"), "No API token found for provider '"+target.Provider+"'")
				fmt.Fprintln(os.Stderr, "\nCannot destroy VM without API token. Aborting to prevent orphaned resources.")
				fmt.Fprintln(os.Stderr, "Local config and state preserved. Re-run after adding token with:")
				fmt.Fprintf(os.Stderr, "  lightfold config set-token %s\n\n", target.Provider)
				os.Exit(1)
			}

			provider, err := providers.GetProvider(target.Provider, token)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\n%s %s\n",
					destroyDangerStyle.Render("✗"), fmt.Sprintf("Error getting provider '%s': %v", target.Provider, err))
				fmt.Fprintln(os.Stderr, "\nCannot destroy VM due to provider error. Aborting to prevent orphaned resources.")
				fmt.Fprintln(os.Stderr, "Local config and state preserved.")
				fmt.Fprintln(os.Stderr)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), config.DefaultDestroyTimeout)
			defer cancel()

			if err := provider.Destroy(ctx, provisionedID); err != nil {
				providerErr, ok := err.(*providers.ProviderError)
				isNotFound := false

				if ok {
					isNotFound = providerErr.Code == "not_found" ||
						providerErr.Code == "server_not_found" ||
						providerErr.Code == "machine_not_found" ||
						providerErr.Code == "droplet_not_found"

					if !isNotFound {
						errMsg := strings.ToLower(providerErr.Message)
						isNotFound = strings.Contains(errMsg, "not found") ||
							strings.Contains(errMsg, "404") ||
							strings.Contains(errMsg, "does not exist")
					}
				}

				if isNotFound {
					fmt.Printf("%s %s\n",
						destroyWarningStyle.Render("⚠"), destroyMutedStyle.Render("VM not found (may have been deleted manually) - proceeding with local cleanup"))
				} else {
					fmt.Fprintf(os.Stderr, "\n%s %s\n",
						destroyDangerStyle.Render("✗"), fmt.Sprintf("Failed to destroy VM: %v", err))
					fmt.Fprintln(os.Stderr, "\nVM destruction failed. Aborting to prevent inconsistent state.")
					fmt.Fprintln(os.Stderr, "Local config and state preserved. Please investigate the error and retry.")
					fmt.Fprintln(os.Stderr)
					os.Exit(1)
				}
			} else {
				fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render("VM destroyed successfully"))
			}
		}

		if target.ServerIP != "" {
			if err := state.UnregisterApp(target.ServerIP, destroyTargetFlag); err != nil {
				fmt.Printf("%s %s\n", destroyWarningStyle.Render("⚠"), destroyMutedStyle.Render(fmt.Sprintf("Failed to unregister app from server: %v", err)))
			} else {
				fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render("Unregistered app from server state"))

				serverState, err := state.GetServerState(target.ServerIP)
				if err == nil {
					if len(serverState.DeployedApps) > 0 {
						fmt.Printf("%s %s\n", destroyMutedStyle.Render("ℹ"), destroyMutedStyle.Render(fmt.Sprintf("Server still hosts %d other app(s)", len(serverState.DeployedApps))))
					}
				}

				// Clean up firewall port if not destroying VM (multi-app scenario with no domain)
				if !shouldDestroyVM && len(otherApps) > 0 && target.Port > 0 {
					// Only close firewall port if app had no domain configured
					if target.Domain == nil || target.Domain.Domain == "" {
						providerCfg, err := target.GetAnyProviderConfig()
						if err == nil && providerCfg.GetIP() != "" {
							fmt.Printf("%s %s\n", destroyMutedStyle.Render("→"), destroyMutedStyle.Render(fmt.Sprintf("Closing firewall port %d...", target.Port)))

							sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
							if err := sshExecutor.Connect(3, 10*time.Second); err == nil {
								defer sshExecutor.Disconnect()

								firewallMgr := firewall.GetDefault(sshExecutor)
								if err := firewallMgr.ClosePort(target.Port); err != nil {
									fmt.Printf("%s %s\n", destroyWarningStyle.Render("⚠"), destroyMutedStyle.Render(fmt.Sprintf("Failed to close firewall port: %v", err)))
								} else {
									fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render(fmt.Sprintf("Closed firewall port %d", target.Port)))
								}
							} else {
								fmt.Printf("%s %s\n", destroyMutedStyle.Render("ℹ"), destroyMutedStyle.Render("Skipping firewall cleanup (SSH connection failed)"))
							}
						}
					}
				}

				// Clean up unused runtimes if keeping the VM (only makes sense for multi-app servers)
				if !shouldDestroyVM && len(otherApps) > 0 {
					providerCfg, err := target.GetAnyProviderConfig()
					if err == nil && providerCfg.GetIP() != "" {
						fmt.Printf("%s %s\n", destroyMutedStyle.Render("→"), destroyMutedStyle.Render("Analyzing runtime dependencies..."))

						sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
						if err := sshExecutor.Connect(3, 10*time.Second); err == nil {
							defer sshExecutor.Disconnect()

							if err := runtime.CleanupUnusedRuntimes(sshExecutor, target.ServerIP, destroyTargetFlag); err != nil {
								fmt.Printf("%s %s\n", destroyWarningStyle.Render("⚠"), destroyMutedStyle.Render(fmt.Sprintf("Runtime cleanup warning: %v", err)))
								fmt.Printf("%s %s\n", destroyMutedStyle.Render("  "), destroyMutedStyle.Render("You may manually run: apt-get autoremove -y"))
							} else {
								fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render("Cleaned up unused runtimes"))
							}
						} else {
							fmt.Printf("%s %s\n", destroyMutedStyle.Render("ℹ"), destroyMutedStyle.Render("Skipping runtime cleanup (SSH connection failed)"))
						}
					}
				}
			}
		}

		if err := state.DeleteState(destroyTargetFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render("Deleted state file"))

		if err := cfg.DeleteTarget(destroyTargetFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting target config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render("Removed target from config"))

		keysDeleted, err := sshpkg.CleanupUnusedKeys(cfg.Targets)
		if err != nil {
			fmt.Printf("%s %s\n", destroyWarningStyle.Render("⚠"), destroyMutedStyle.Render(fmt.Sprintf("SSH key cleanup warning: %v", err)))
		} else if keysDeleted > 0 {
			fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render(fmt.Sprintf("Cleaned up %d unused SSH key(s)", keysDeleted)))
		} else {
			fmt.Printf("%s %s\n", destroyMutedStyle.Render("ℹ"), destroyMutedStyle.Render("No unused SSH keys found"))
		}

		fmt.Println()

		successBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(0, 1).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					destroyDangerStyle.Render(fmt.Sprintf("✓ Target '%s' destroyed successfully", destroyTargetFlag)),
					"",
					destroyMutedStyle.Render("All resources and configuration removed"),
				),
			)

		fmt.Println(successBox)
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().StringVar(&destroyTargetFlag, "target", "", "Target name (required)")
	destroyCmd.MarkFlagRequired("target")
}
