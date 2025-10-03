package cmd

import (
	"context"
	"fmt"
	tui "lightfold/cmd/ui"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"lightfold/pkg/state"
	sshpkg "lightfold/pkg/ssh"
	"os"
	"path/filepath"
	"strings"
	"time"

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

		projectPath := target.ProjectPath

		if err := validateConfigureTargetConfig(target); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid target configuration: %v\n", err)
			os.Exit(1)
		}

		if !configureForceFlag {
			providerCfg, err := target.GetSSHProviderConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
			result := sshExecutor.Execute("test -f /etc/lightfold/configured && echo 'configured'")
			sshExecutor.Disconnect()

			if result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == "configured" {
				fmt.Printf("Target '%s' is already configured.\n", configureTargetFlag)
				fmt.Println("Use --force to reconfigure")
				os.Exit(0)
			}
		}

		if err := processConfigureFlags(&target); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing configuration options: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Configuring target '%s' (%s on %s)...\n", configureTargetFlag, target.Framework, target.Provider)
		fmt.Println()

		projectName := filepath.Base(projectPath)
		orchestrator, err := deploy.GetOrchestrator(target, projectPath, projectName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating orchestrator: %v\n", err)
			os.Exit(1)
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := tui.ShowConfigurationProgressWithOrchestrator(ctx, orchestrator, providerCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error during configuration: %v\n", err)
			os.Exit(1)
		}

		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		result := sshExecutor.Execute("echo 'configured' | sudo tee /etc/lightfold/configured > /dev/null")
		sshExecutor.Disconnect()

		if result.Error != nil || result.ExitCode != 0 {
			fmt.Printf("Warning: failed to write configured marker: %v\n", result.Error)
		}

		if err := state.MarkConfigured(configureTargetFlag); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		fmt.Printf("\n✅ Target '%s' configured successfully!\n", configureTargetFlag)
	},
}

func validateConfigureTargetConfig(target config.TargetConfig) error {
	if target.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	if target.Provider == "s3" {
		return fmt.Errorf("configure command does not support S3 deployments")
	}

	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		return err
	}

	if providerCfg.GetIP() == "" {
		return fmt.Errorf("IP address is required. Please run 'lightfold create' first")
	}

	if providerCfg.GetUsername() == "" {
		return fmt.Errorf("username is required")
	}

	if providerCfg.GetSSHKey() == "" {
		return fmt.Errorf("SSH key is required")
	}

	return nil
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
