package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	"os"

	"github.com/spf13/cobra"
)

var (
	configureTargetFlag string
	configureForceFlag  bool
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure an existing VM with your application",
	Long: `Configure an existing server with your application code and dependencies.

This command is idempotent - it checks if the server is already configured and skips if so.

Examples:
  lightfold configure --target myapp
  lightfold configure --target myapp --force`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if configureTargetFlag == "" {
			fmt.Fprintf(os.Stderr, "Error: --target flag is required\n")
			os.Exit(1)
		}

		cfg := loadConfigOrExit()
		target := loadTargetOrExit(cfg, configureTargetFlag)

		if err := processConfigureFlags(&target); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing configuration options: %v\n", err)
			os.Exit(1)
		}

		if err := configureTarget(target, configureTargetFlag, configureForceFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func processConfigureFlags(target *config.TargetConfig) error {
	return target.ProcessDeploymentOptions(envFile, envVars, skipBuild)
}

func init() {
	rootCmd.AddCommand(configureCmd)

	configureCmd.Flags().StringVar(&configureTargetFlag, "target", "", "Target name (required)")
	configureCmd.Flags().BoolVar(&configureForceFlag, "force", false, "Force reconfiguration even if already configured")
	configureCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	configureCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	configureCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during configuration")

	configureCmd.MarkFlagRequired("target")
}
