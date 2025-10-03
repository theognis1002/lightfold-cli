package cmd

import (
	"bufio"
	"context"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/providers"
	_ "lightfold/pkg/providers/digitalocean"
	_ "lightfold/pkg/providers/hetzner"
	"lightfold/pkg/state"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	destroyTargetFlag string

	// Styles for destroy command
	destroyWarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	destroyDangerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	destroyMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	destroySuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
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

		// Load config
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Get target config
		target, exists := cfg.GetTarget(destroyTargetFlag)
		if !exists {
			fmt.Printf("Target '%s' not found in configuration.\n", destroyTargetFlag)

			// Check if state file exists
			targetState, _ := state.LoadState(destroyTargetFlag)
			if targetState != nil && (targetState.Created || targetState.Configured) {
				fmt.Println("However, state file exists. Cleaning up state...")
				if err := state.DeleteState(destroyTargetFlag); err != nil {
					fmt.Fprintf(os.Stderr, "Error deleting state: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("✓ Deleted state for target '%s'\n", destroyTargetFlag)
			}
			return
		}

		// Load state
		provisionedID := state.GetProvisionedID(destroyTargetFlag)

		// Get provider config
		providerCfg, _ := target.GetAnyProviderConfig()

		// Display what will be destroyed
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

		// Confirmation prompt
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Type the target name '%s' to confirm: ", destroyTargetFlag)
		confirmation, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading confirmation: %v\n", err)
			os.Exit(1)
		}

		confirmation = strings.TrimSpace(confirmation)
		if confirmation != destroyTargetFlag {
			fmt.Println("\nCancelled. Target name did not match.")
			os.Exit(0)
		}

		fmt.Println()
		fmt.Println(destroyMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
		fmt.Printf("\n%s\n\n", destroyWarningStyle.Render(fmt.Sprintf("Destroying target '%s'...", destroyTargetFlag)))

		// Destroy provisioned VM if exists
		if provisionedID != "" && target.Provider != "" && target.Provider != "byos" {
			fmt.Printf("  %s Destroying VM...\n", destroyMutedStyle.Render("→"))

			// Load API token
			tokens, err := config.LoadTokens()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading tokens: %v\n", err)
				os.Exit(1)
			}

			token := tokens.GetToken(target.Provider)
			if token == "" {
				fmt.Printf("  %s No API token found for provider '%s', skipping VM destruction\n",
					destroyWarningStyle.Render("⚠"), target.Provider)
			} else {
				// Get provider instance
				provider, err := providers.GetProvider(target.Provider, token)
				if err != nil {
					fmt.Printf("  %s Error getting provider '%s': %v\n",
						destroyWarningStyle.Render("⚠"), target.Provider, err)
					fmt.Println("  Continuing with local cleanup...")
				} else {
					// Destroy the VM
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()

					if err := provider.Destroy(ctx, provisionedID); err != nil {
						fmt.Printf("  %s Failed to destroy VM: %v\n",
							destroyWarningStyle.Render("⚠"), err)
						fmt.Println("  Continuing with local cleanup...")
					} else {
						fmt.Printf("  %s VM destroyed successfully\n", destroySuccessStyle.Render("✓"))
					}
				}
			}
		}

		// Delete state file
		if err := state.DeleteState(destroyTargetFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  %s Deleted state file\n", destroySuccessStyle.Render("✓"))

		// Delete target from config
		if err := cfg.DeleteTarget(destroyTargetFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting target config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  %s Removed target from config\n", destroySuccessStyle.Render("✓"))

		fmt.Println()
		fmt.Println(destroyMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
		fmt.Printf("%s\n", destroySuccessStyle.Render(fmt.Sprintf("✅ Target '%s' destroyed successfully", destroyTargetFlag)))
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().StringVar(&destroyTargetFlag, "target", "", "Target name (required)")
	destroyCmd.MarkFlagRequired("target")
}
