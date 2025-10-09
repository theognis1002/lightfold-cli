package cmd

import (
	"fmt"
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
	envFile           string
	envVars           []string
	skipBuild         bool
	deployTargetFlag  string
	deployForceFlag   bool
	deployDryRun      bool
	deployBuilderFlag string

	deployStepHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	deploySuccessStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	deployMutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	deployValueStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
)

var deployCmd = &cobra.Command{
	Use:   "deploy [PROJECT_PATH]",
	Short: "Deploy your application (orchestrates detect → create → configure → push)",
	Long: `Orchestrate a full deployment pipeline by running:
1. detect - Framework detection
2. create - Infrastructure provisioning (if needed)
3. configure - Server configuration (if needed)
4. push - Release deployment

Each step is skipped if already completed. Use --force to rerun all steps.

Examples:
  lightfold deploy                           # Deploy current directory
  lightfold deploy ~/Projects/myapp          # Deploy specific project
  lightfold deploy --target myapp            # Deploy named target
  lightfold deploy --target myapp --force    # Force rerun all steps
  lightfold deploy --dry-run                 # Preview deployment plan`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		effectiveTarget := deployTargetFlag
		if effectiveTarget == "" {
			if len(args) > 0 {
				effectiveTarget = args[0]
			} else {
				effectiveTarget = "."
			}
		}

		cfg := loadConfigOrExit()

		target, exists := cfg.GetTarget(effectiveTarget)
		var projectPath string
		var targetName string

		if !exists {
			var err error
			projectPath, err = util.ValidateProjectPath(effectiveTarget)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			targetName = util.GetTargetName(projectPath)
		} else {
			projectPath = target.ProjectPath
			targetName = effectiveTarget
		}

		projectPath = filepath.Clean(projectPath)

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

		builderName := resolveBuilder(target, projectPath, &detection, deployBuilderFlag)

		// Check if Dockerfile exists but dockerfile builder is not available (fallback scenario)
		dockerfilePath := filepath.Join(projectPath, "Dockerfile")
		_, dockerfileExists := os.Stat(dockerfilePath)
		showFallback := dockerfileExists == nil && builderName != "dockerfile"

		if showFallback {
			fmt.Printf("  %s %s %s\n",
				deployMutedStyle.Render("Builder:"),
				deployMutedStyle.Render(builderName),
				deployMutedStyle.Render("(dockerfile not implemented, using "+builderName+")"))
		} else {
			fmt.Printf("  %s %s\n", deployMutedStyle.Render("Builder:"), deployMutedStyle.Render(builderName))
		}

		if target.Builder != builderName {
			target.Builder = builderName
		}

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render("Step 2/4: Create infra"))
		var err error
		target, err = createTarget(targetName, projectPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating infrastructure: %v\n", err)
			os.Exit(1)
		}

		cfg = loadConfigOrExit()
		target = loadTargetOrExit(cfg, targetName)

		target.Builder = builderName
		if err := cfg.SetTarget(targetName, target); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving builder config: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.SaveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render("Step 3/4: Configure server"))
		if err := configureTarget(target, targetName, deployForceFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error configuring server: %v\n", err)
			os.Exit(1)
		}

		cfg = loadConfigOrExit()
		target = loadTargetOrExit(cfg, targetName)

		promptDomainConfiguration(&target, targetName)

		cfg = loadConfigOrExit()
		target = loadTargetOrExit(cfg, targetName)

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

		var executor *deploy.Executor
		if target.Deploy != nil && (len(target.Deploy.BuildCommands) > 0 || len(target.Deploy.RunCommands) > 0) {
			executor = deploy.NewExecutorWithOptions(sshExecutor, projectName, projectPath, &detection, target.Deploy)
		} else {
			executor = deploy.NewExecutor(sshExecutor, projectName, projectPath, &detection)
		}

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
			if err := executor.BuildReleaseWithEnv(releasePath, target.Deploy.EnvVars); err != nil {
				fmt.Fprintf(os.Stderr, "Error building release: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Building app..."))
		}

		if len(target.Deploy.EnvVars) > 0 {
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
		fmt.Println()

		if target.Domain == nil || target.Domain.Domain == "" {
			hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			domainHint := fmt.Sprintf("Have a domain? Run 'lightfold domain add --target %s --domain example.com' to add a custom domain.", targetName)
			fmt.Printf("%s\n", hintStyle.Render(domainHint))
		}
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)

	deployCmd.Flags().StringVar(&deployTargetFlag, "target", "", "Target name (defaults to current directory)")
	deployCmd.Flags().BoolVar(&deployForceFlag, "force", false, "Force rerun all steps")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show deployment plan without executing")
	deployCmd.Flags().StringVar(&deployBuilderFlag, "builder", "", "Builder to use: native, nixpacks, or dockerfile (auto-detected if not specified)")
	deployCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	deployCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	deployCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during deployment")
}
