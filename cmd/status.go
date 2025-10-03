package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/state"
	sshpkg "lightfold/pkg/ssh"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var statusTargetFlag string

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

		// If no target specified, show all targets
		if statusTargetFlag == "" {
			showAllTargets(cfg)
			return
		}

		// Show detailed status for specific target
		showTargetDetail(cfg, statusTargetFlag)
	},
}

func showAllTargets(cfg *config.Config) {
	if len(cfg.Targets) == 0 {
		fmt.Println("No targets configured yet.")
		fmt.Println("\nCreate your first target:")
		fmt.Println("  lightfold create --target myapp --provider byos --ip YOUR_IP")
		return
	}

	fmt.Printf("Configured Targets (%d):\n", len(cfg.Targets))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for targetName, target := range cfg.Targets {
		fmt.Printf("\n%s\n", targetName)

		// Load state
		targetState, err := state.LoadState(targetName)
		if err != nil {
			fmt.Printf("  Error loading state: %v\n", err)
			continue
		}

		// Show basic info
		fmt.Printf("  Provider:    %s\n", target.Provider)
		fmt.Printf("  Framework:   %s\n", target.Framework)

		// Show IP if available
		var ip string
		var hasIP bool
		switch target.Provider {
		case "digitalocean":
			if doConfig, err := target.GetDigitalOceanConfig(); err == nil {
				ip = doConfig.IP
				hasIP = ip != ""
			}
		case "hetzner":
			if hConfig, err := target.GetHetznerConfig(); err == nil {
				ip = hConfig.IP
				hasIP = ip != ""
			}
		}
		if ip != "" {
			fmt.Printf("  IP:          %s\n", ip)
		}

		// Show last deploy info
		if !targetState.LastDeploy.IsZero() {
			fmt.Printf("  Last Deploy: %s\n", targetState.LastDeploy.Format("2006-01-02 15:04"))
		}

		// Show pipeline status
		fmt.Printf("\n  Pipeline:\n")

		// 1. Detect (always complete if target exists)
		fmt.Printf("    ✓ Detect\n")

		// 2. Create (check both state flag and IP presence)
		if targetState.Created || hasIP {
			fmt.Printf("    ✓ Create\n")
		} else {
			fmt.Printf("    [ ] Create\n")
		}

		// 3. Configure
		if targetState.Configured {
			fmt.Printf("    ✓ Configure\n")
		} else {
			fmt.Printf("    [ ] Configure\n")
		}

		// 4. Push
		if !targetState.LastDeploy.IsZero() {
			fmt.Printf("    ✓ Push\n")
		} else {
			fmt.Printf("    [ ] Push\n")
		}
	}

	fmt.Println("\nFor detailed status: lightfold status --target <name>")
}

func showTargetDetail(cfg *config.Config, targetName string) {
	target, exists := cfg.GetTarget(targetName)
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: Target '%s' not found\n", targetName)
		os.Exit(1)
	}

	// Load state
	targetState, err := state.LoadState(targetName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	// Print status
	fmt.Printf("Target: %s\n", targetName)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Configuration
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Project:   %s\n", target.ProjectPath)
	fmt.Printf("  Framework: %s\n", target.Framework)
	fmt.Printf("  Provider:  %s\n", target.Provider)
	fmt.Println()

	// State
	fmt.Printf("State:\n")
	if targetState.Created {
		fmt.Printf("  Created:    ✓ Yes\n")
	} else {
		fmt.Printf("  Created:    ✗ No\n")
	}
	if targetState.Configured {
		fmt.Printf("  Configured: ✓ Yes\n")
	} else {
		fmt.Printf("  Configured: ✗ No\n")
	}
	if targetState.ProvisionedID != "" {
		fmt.Printf("  Server ID:  %s\n", targetState.ProvisionedID)
	}
	if targetState.LastCommit != "" {
		commitShort := targetState.LastCommit
		if len(commitShort) > 7 {
			commitShort = commitShort[:7]
		}
		fmt.Printf("  Last Commit: %s\n", commitShort)
	}
	if !targetState.LastDeploy.IsZero() {
		fmt.Printf("  Last Deploy: %s\n", targetState.LastDeploy.Format("2006-01-02 15:04:05"))
	}
	if targetState.LastRelease != "" {
		fmt.Printf("  Last Release: %s\n", targetState.LastRelease)
	}
	fmt.Println()

	// Server status (if configured)
	if targetState.Created {
		fmt.Printf("Server Status:\n")

		var providerCfg config.ProviderConfig
		switch target.Provider {
		case "digitalocean":
			providerCfg, err = target.GetDigitalOceanConfig()
		case "hetzner":
			providerCfg, err = target.GetHetznerConfig()
		case "s3":
			fmt.Printf("  Type: S3 (static site)\n")
			s3Config, _ := target.GetS3Config()
			fmt.Printf("  Bucket: %s\n", s3Config.Bucket)
			fmt.Printf("  Region: %s\n", s3Config.Region)
			fmt.Println()
			return
		default:
			fmt.Printf("  Unknown provider: %s\n", target.Provider)
			return
		}

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			return
		}

		fmt.Printf("  IP:        %s\n", providerCfg.GetIP())
		fmt.Printf("  Username:  %s\n", providerCfg.GetUsername())

		// Try to connect and get server status
		if providerCfg.GetIP() != "" {
			sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
			defer sshExecutor.Disconnect()

			// Check service status
			appName := strings.ReplaceAll(targetName, "-", "_")
			result := sshExecutor.Execute(fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo 'not-found'", appName))
			if result.ExitCode == 0 {
				status := strings.TrimSpace(result.Stdout)
				if status == "active" {
					fmt.Printf("  Service:   ✓ Active\n")
				} else if status == "not-found" {
					fmt.Printf("  Service:   - Not configured\n")
				} else {
					fmt.Printf("  Service:   ✗ %s\n", status)
				}
			} else {
				fmt.Printf("  Service:   ? Unable to check\n")
			}

			// Check current release
			result = sshExecutor.Execute(fmt.Sprintf("readlink -f /srv/%s/current 2>/dev/null || echo 'none'", appName))
			if result.ExitCode == 0 {
				currentRelease := strings.TrimSpace(result.Stdout)
				if currentRelease != "none" && currentRelease != "" {
					releaseTimestamp := strings.TrimPrefix(currentRelease, fmt.Sprintf("/srv/%s/releases/", appName))
					fmt.Printf("  Current:   %s\n", releaseTimestamp)
				} else {
					fmt.Printf("  Current:   - No release deployed\n")
				}
			}

			// Check disk usage
			result = sshExecutor.Execute("df -h / | tail -1 | awk '{print $5}'")
			if result.ExitCode == 0 {
				diskUsage := strings.TrimSpace(result.Stdout)
				fmt.Printf("  Disk:      %s used\n", diskUsage)
			}

			// Check uptime
			result = sshExecutor.Execute("uptime -p 2>/dev/null || uptime | awk '{print $3, $4}'")
			if result.ExitCode == 0 {
				uptime := strings.TrimSpace(result.Stdout)
				fmt.Printf("  Uptime:    %s\n", uptime)
			}
		}
		fmt.Println()
	}

	// Next steps
	if !targetState.Created {
		fmt.Printf("Next: lightfold create --target %s\n", targetName)
	} else if !targetState.Configured {
		fmt.Printf("Next: lightfold configure --target %s\n", targetName)
	} else {
		fmt.Printf("Ready to deploy: lightfold push --target %s\n", targetName)
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVar(&statusTargetFlag, "target", "", "Target name (optional - shows all targets if omitted)")
}
