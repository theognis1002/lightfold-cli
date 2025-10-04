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
	"lightfold/pkg/util"
	"os"
	"strings"

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

		if provisionedID != "" && target.Provider != "" && target.Provider != "byos" {
			fmt.Printf("%s %s\n", destroyWarningStyle.Render("→"), destroyMutedStyle.Render("Destroying VM..."))

			tokens, err := config.LoadTokens()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading tokens: %v\n", err)
				os.Exit(1)
			}

			token := tokens.GetToken(target.Provider)
			if token == "" {
				fmt.Printf("%s %s\n",
					destroyWarningStyle.Render("⚠"), destroyMutedStyle.Render(fmt.Sprintf("No API token found for provider '%s', skipping VM destruction", target.Provider)))
			} else {
				provider, err := providers.GetProvider(target.Provider, token)
				if err != nil {
					fmt.Printf("%s %s\n",
						destroyWarningStyle.Render("⚠"), destroyMutedStyle.Render(fmt.Sprintf("Error getting provider '%s': %v", target.Provider, err)))
					fmt.Printf("%s %s\n", destroyMutedStyle.Render("→"), destroyMutedStyle.Render("Continuing with local cleanup..."))
				} else {
					ctx, cancel := context.WithTimeout(context.Background(), config.DefaultDestroyTimeout)
					defer cancel()

					if err := provider.Destroy(ctx, provisionedID); err != nil {
						fmt.Printf("%s %s\n",
							destroyWarningStyle.Render("⚠"), destroyMutedStyle.Render(fmt.Sprintf("Failed to destroy VM: %v", err)))
						fmt.Printf("%s %s\n", destroyMutedStyle.Render("→"), destroyMutedStyle.Render("Continuing with local cleanup..."))
					} else {
						fmt.Printf("%s %s\n", destroySuccessStyle.Render("✓"), destroyMutedStyle.Render("VM destroyed successfully"))
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

		fmt.Println()

		// Success banner with cleaner bubbletea style (red border for destruction)
		successBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")). // Red border for destruction
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
