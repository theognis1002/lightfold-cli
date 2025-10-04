package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"lightfold/pkg/detector"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
	"lightfold/pkg/util"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	envFile          string
	envVars          []string
	skipBuild        bool
	rollbackFlag     bool
	deployTargetFlag string
	deployForceFlag  bool
	deployDryRun     bool

	deployHeaderStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	deployStepHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	deployStepStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	deploySuccessStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	deployMutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	deployValueStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your application (orchestrates detect → create → configure → push)",
	Long: `Orchestrate a full deployment pipeline by running:
1. detect - Framework detection
2. create - Infrastructure provisioning (if needed)
3. configure - Server configuration (if needed)
4. push - Release deployment

Each step is skipped if already completed. Use --force to rerun all steps.

Examples:
  lightfold deploy --target myapp
  lightfold deploy --target myapp --force
  lightfold deploy --target myapp --dry-run`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if deployTargetFlag == "" {
			fmt.Fprintf(os.Stderr, "Error: --target flag is required\n")
			os.Exit(1)
		}

		cfg := loadConfigOrExit()

		target, exists := cfg.GetTarget(deployTargetFlag)
		var projectPath string
		var targetName string

		if !exists {
			projectPath = filepath.Clean(deployTargetFlag)
			targetName = util.GetTargetName(projectPath)
		} else {
			projectPath = target.ProjectPath
			targetName = deployTargetFlag
		}

		projectPath = filepath.Clean(projectPath)

		if rollbackFlag {
			if !exists {
				fmt.Println("No deployment configuration found for this target.")
				os.Exit(1)
			}
			handleRollback(target, projectPath)
			return
		}

		if deployDryRun {
			fmt.Println("DRY RUN - Deployment plan:")
			fmt.Printf("Target: %s\n", targetName)
			fmt.Printf("Steps:\n")
			if !exists || deployForceFlag {
				fmt.Println("  1. ✓ detect - Framework detection")
			} else {
				fmt.Println("  1. ⊘ detect - Skipped (cached)")
			}
			if !state.IsCreated(targetName) || deployForceFlag {
				fmt.Println("  2. ✓ create - Infrastructure provisioning")
			} else {
				fmt.Println("  2. ⊘ create - Skipped (already created)")
			}
			if !state.IsConfigured(targetName) || deployForceFlag {
				fmt.Println("  3. ✓ configure - Server configuration")
			} else {
				fmt.Println("  3. ⊘ configure - Skipped (already configured)")
			}
			fmt.Println("  4. ✓ push - Release deployment")
			return
		}

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render("Step 1/4: Analyzing app"))
		detection := detector.DetectFramework(projectPath)

		fmt.Printf("%s %s (%s)\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render(detection.Framework), deployMutedStyle.Render(detection.Language))

		if pm, ok := detection.Meta["package_manager"]; ok && pm != "" {
			if len(detection.BuildPlan) > 0 {
				installCmd := detection.BuildPlan[0]
				fmt.Printf("  %s\n", deployMutedStyle.Render(fmt.Sprintf("%s (%s)", installCmd, pm)))
			}
		}

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render("Step 2/4: Create infra"))
		var err error
		target, err = createTarget(targetName, projectPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating infrastructure: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render("Step 3/4: Configure server"))
		if err := configureTarget(target, targetName, deployForceFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error configuring server: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render("Step 4/4: Deploy app"))

		if err := target.ProcessDeploymentOptions(envFile, envVars, skipBuild); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		sshProviderCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		sshExecutor := sshpkg.NewExecutor(sshProviderCfg.GetIP(), "22", sshProviderCfg.GetUsername(), sshProviderCfg.GetSSHKey())
		defer sshExecutor.Disconnect()

		if err := sshExecutor.Connect(3, 2*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
			os.Exit(1)
		}

		projectName := util.GetTargetName(projectPath)
		executor := deploy.NewExecutor(sshExecutor, projectName, projectPath, &detection)

		tmpTarball := fmt.Sprintf("/tmp/lightfold-%s-release.tar.gz", projectName)
		if err := executor.CreateReleaseTarball(tmpTarball); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tarball: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(tmpTarball)
		fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Creating release tarball..."))

		releasePath, err := executor.UploadRelease(tmpTarball)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error uploading release: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Uploading release to server..."))

		if !target.Deploy.SkipBuild {
			// Pass env vars to build (for NEXT_PUBLIC_*, etc.)
			if err := executor.BuildReleaseWithEnv(releasePath, target.Deploy.EnvVars); err != nil {
				fmt.Fprintf(os.Stderr, "Error building release: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Building app..."))
		}

		if len(target.Deploy.EnvVars) > 0 {
			// Also write to shared location for runtime
			if err := executor.WriteEnvironmentFile(target.Deploy.EnvVars); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing environment: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Configuring environment variables..."))
		}

		if err := executor.DeployWithHealthCheck(releasePath, 5, 3*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "Error during deployment: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Deploying and running health checks..."))

		executor.CleanupOldReleases(cfg.KeepReleases)
		fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Cleaning up old releases..."))

		releaseTimestamp := filepath.Base(releasePath)
		currentCommit := getGitCommit(projectPath)
		if err := state.UpdateDeployment(targetName, currentCommit, releaseTimestamp); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		fmt.Println()

		successBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("82")).
			Padding(0, 1).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					deploySuccessStyle.Render(fmt.Sprintf("✓ Successfully deployed '%s'", targetName)),
					"",
					fmt.Sprintf("%s %s", deployMutedStyle.Render("Server:"), deployValueStyle.Render(sshProviderCfg.GetIP())),
					fmt.Sprintf("%s %s", deployMutedStyle.Render("Release:"), deployValueStyle.Render(releaseTimestamp)),
				),
			)

		fmt.Println(successBox)
	},
}

func handleRollback(target config.TargetConfig, projectPath string) {
	fmt.Println("Rolling back to previous release...")

	if target.Provider == "s3" {
		fmt.Fprintf(os.Stderr, "Error: Rollback is not supported for S3 deployments\n")
		os.Exit(1)
	}

	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if providerCfg.GetIP() == "" {
		fmt.Fprintf(os.Stderr, "Error: No server IP found in configuration\n")
		os.Exit(1)
	}

	projectName := util.GetTargetName(projectPath)

	fmt.Printf("Connecting to server at %s...\n", providerCfg.GetIP())
	sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
	defer sshExecutor.Disconnect()

	executor := deploy.NewExecutor(sshExecutor, projectName, projectPath, nil)

	if err := executor.RollbackToPreviousRelease(); err != nil {
		fmt.Fprintf(os.Stderr, "Error during rollback: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Successfully rolled back to previous release")
}

func init() {
	rootCmd.AddCommand(deployCmd)

	deployCmd.Flags().StringVar(&deployTargetFlag, "target", "", "Target name (required)")
	deployCmd.Flags().BoolVar(&deployForceFlag, "force", false, "Force rerun all steps")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show deployment plan without executing")
	deployCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	deployCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	deployCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during deployment")
	deployCmd.Flags().BoolVar(&rollbackFlag, "rollback", false, "Rollback to the previous release")
}
