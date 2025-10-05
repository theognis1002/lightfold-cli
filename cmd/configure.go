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
	},
}

func processConfigureFlags(target *config.TargetConfig) error {
	return target.ProcessDeploymentOptions(envFile, envVars, skipBuild)
}

func init() {
	rootCmd.AddCommand(configureCmd)

	configureCmd.Flags().StringVar(&configureTargetFlag, "target", "", "Target name (defaults to current directory)")
	configureCmd.Flags().BoolVarP(&configureForceFlag, "force", "f", false, "Force reconfiguration even if already configured")
	configureCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	configureCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	configureCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during configuration")
}
