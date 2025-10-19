package cmd

import (
	"context"
	"fmt"
	"lightfold/cmd/utils"
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
	envFile           string
	envVars           []string
	skipBuild         bool
	deployTargetFlag  string
	deployForceFlag   bool
	deployDryRun      bool
	deployBuilderFlag string
	deployServerIP    string

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

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render(fmt.Sprintf("Step 1/4: Analyzing '%s' app", targetName)))
		detection := detector.DetectFramework(projectPath)

		fmt.Printf("%s %s (%s)\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render(detection.Framework), deployMutedStyle.Render(detection.Language))

		if pm, ok := detection.Meta["package_manager"]; ok && pm != "" {
			if len(detection.BuildPlan) > 0 {
				installCmd := detection.BuildPlan[0]
				fmt.Printf("  %s\n", deployMutedStyle.Render(fmt.Sprintf("%s (%s)", installCmd, pm)))
			}
		}

		builderName := resolveBuilder(target, projectPath, &detection, deployBuilderFlag)

		fmt.Printf("  %s %s\n", deployMutedStyle.Render("Builder:"), deployMutedStyle.Render(builderName))

		if target.Builder != builderName {
			target.Builder = builderName
		}

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render("Step 2/4: Creating infrastructure"))
		var err error

		// If server-ip is provided, setup target with existing server
		if deployServerIP != "" {
			if exists {
				if err := utils.SetupTargetWithExistingServer(&target, deployServerIP, 0); err != nil {
					fmt.Fprintf(os.Stderr, "Error configuring target for existing server: %v\n", err)
					os.Exit(1)
				}
				if err := cfg.SetTarget(targetName, target); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving target config: %v\n", err)
					os.Exit(1)
				}
				if err := cfg.SaveConfig(); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
					os.Exit(1)
				}

				if err := state.MarkCreated(targetName, ""); err != nil {
					fmt.Printf("Warning: failed to update state: %v\n", err)
				}

				skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
				fmt.Printf("  %s\n", skipStyle.Render(fmt.Sprintf("Using existing server %s (skipping provisioning)", deployServerIP)))
			} else {
				target = config.TargetConfig{
					ProjectPath: projectPath,
					Framework:   detection.Framework,
				}

				if err := utils.SetupTargetWithExistingServer(&target, deployServerIP, 0); err != nil {
					fmt.Fprintf(os.Stderr, "Error configuring target for existing server: %v\n", err)
					os.Exit(1)
				}

				if err := cfg.SetTarget(targetName, target); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving target config: %v\n", err)
					os.Exit(1)
				}
				if err := cfg.SaveConfig(); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
					os.Exit(1)
				}

				if err := state.MarkCreated(targetName, ""); err != nil {
					fmt.Printf("Warning: failed to update state: %v\n", err)
				}

				skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
				fmt.Printf("  %s\n", skipStyle.Render(fmt.Sprintf("Using existing server %s (skipping provisioning)", deployServerIP)))
			}
		} else {
			// Normal flow - create infrastructure
			target, err = createTarget(targetName, projectPath, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating infrastructure: %v\n", err)
				os.Exit(1)
			}
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

		fmt.Printf("\n%s\n", deployStepHeaderStyle.Render("Step 3/4: Configuring server"))
		isCalledFromDeploy = true
		if err := configureTarget(target, targetName, deployForceFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error configuring server: %v\n", err)
			os.Exit(1)
		}
		isCalledFromDeploy = false

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

		// Branch on deployment strategy
		if !target.RequiresSSHDeployment() {
			// Container-based deployment (e.g., fly.io)
			if err := deployViaContainer(target, targetName, projectPath, &detection); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// SSH-based deployment (DigitalOcean, Hetzner, Vultr, BYOS)
		sshProviderCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Ensure server state is initialized
		if err := updateServerStateFromTarget(&target, targetName); err != nil {
			fmt.Printf("Warning: failed to update server state: %v\n", err)
		}

		// Allocate port if not already set
		if target.Port == 0 {
			port, err := getOrAllocatePort(&target, targetName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error allocating port: %v\n", err)
				os.Exit(1)
			}
			target.Port = port

			// Save port to config
			if err := cfg.SetTarget(targetName, target); err != nil {
				fmt.Printf("Warning: failed to save port to config: %v\n", err)
			}
			if err := cfg.SaveConfig(); err != nil {
				fmt.Printf("Warning: failed to save config: %v\n", err)
			}
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
			state.MarkPushFailed(targetName, fmt.Sprintf("failed to create tarball: %v", err))
			fmt.Fprintf(os.Stderr, "Error creating tarball: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(tmpTarball)
		fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Creating release tarball..."))

		releasePath, err := executor.UploadRelease(tmpTarball)
		if err != nil {
			state.MarkPushFailed(targetName, fmt.Sprintf("failed to upload release: %v", err))
			fmt.Fprintf(os.Stderr, "Error uploading release: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Uploading release to server..."))

		if !target.Deploy.SkipBuild {
			if err := executor.BuildReleaseWithEnv(releasePath, target.Deploy.EnvVars); err != nil {
				state.MarkPushFailed(targetName, fmt.Sprintf("failed to build release: %v", err))
				fmt.Fprintf(os.Stderr, "Error building release: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Building app..."))
		}

		if len(target.Deploy.EnvVars) > 0 {
			if err := executor.WriteEnvironmentFile(target.Deploy.EnvVars); err != nil {
				state.MarkPushFailed(targetName, fmt.Sprintf("failed to write environment file: %v", err))
				fmt.Fprintf(os.Stderr, "Error writing environment: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Configuring environment variables..."))
		}

		if err := executor.DeployWithHealthCheck(releasePath, target.Port, 5, 3*time.Second); err != nil {
			state.MarkPushFailed(targetName, fmt.Sprintf("deployment failed: %v", err))
			fmt.Fprintf(os.Stderr, "Error during deployment: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Deploying and running health checks..."))

		executor.CleanupOldReleases(cfg.NumReleases)
		fmt.Printf("%s %s\n", deploySuccessStyle.Render("✓"), deployMutedStyle.Render("Cleaning up old releases..."))

		releaseTimestamp := filepath.Base(releasePath)
		currentCommit := getGitCommit(projectPath)
		if err := state.ClearPushFailure(targetName); err != nil {
			fmt.Printf("Warning: failed to clear push failure in state: %v\n", err)
		}
		if err := state.UpdateDeployment(targetName, currentCommit, releaseTimestamp); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		// Register app with server state
		if err := registerAppWithServer(&target, targetName, target.Port, target.Framework); err != nil {
			fmt.Printf("Warning: failed to register app with server: %v\n", err)
		}

		fmt.Println()

		// Build success message lines
		successLines := []string{
			deploySuccessStyle.Render(fmt.Sprintf("✓ Successfully deployed '%s'", targetName)),
			"",
			fmt.Sprintf("%s %s", deployMutedStyle.Render("Server:"), deployValueStyle.Render(sshProviderCfg.GetIP())),
			fmt.Sprintf("%s %s", deployMutedStyle.Render("Release:"), deployValueStyle.Render(releaseTimestamp)),
		}

		// Check if this is a multi-app deployment
		serverState, serverStateErr := state.GetServerState(sshProviderCfg.GetIP())
		isMultiApp := serverStateErr == nil && len(serverState.DeployedApps) > 1

		// Add port and access information
		if target.Port > 0 {
			if target.Domain != nil && target.Domain.Domain != "" {
				// Has custom domain - nginx proxies to app port
				// Don't show port for single-app with domain
				if isMultiApp {
					successLines = append(successLines, fmt.Sprintf("%s %s (proxied via nginx)", deployMutedStyle.Render("Port:"), deployValueStyle.Render(fmt.Sprintf("%d", target.Port))))
				}
			} else if !isMultiApp {
				// Single-app without domain - nginx proxies port 80 to app port
				// Don't show internal port, just the access URL
				successLines = append(successLines, fmt.Sprintf("%s %s", deployMutedStyle.Render("Access:"), deployValueStyle.Render(fmt.Sprintf("http://%s", sshProviderCfg.GetIP()))))
			} else {
				// Multi-app without domain - needs domain or direct port access
				successLines = append(successLines, fmt.Sprintf("%s %s", deployMutedStyle.Render("Port:"), deployValueStyle.Render(fmt.Sprintf("%d", target.Port))))
				successLines = append(successLines, fmt.Sprintf("%s %s", deployMutedStyle.Render("Access:"), deployValueStyle.Render(fmt.Sprintf("http://%s:%d (direct port access)", sshProviderCfg.GetIP(), target.Port))))
			}
		}

		// Add domain information if configured
		if target.Domain != nil && target.Domain.Domain != "" {
			successLines = append(successLines, fmt.Sprintf("%s %s", deployMutedStyle.Render("Domain:"), deployValueStyle.Render(target.Domain.Domain)))
		}

		// List other apps on this server
		targetsOnServer := cfg.GetTargetsByServerIP(sshProviderCfg.GetIP())
		otherApps := []string{}
		for name, t := range targetsOnServer {
			if name != targetName {
				appInfo := name
				if t.Port > 0 {
					appInfo = fmt.Sprintf("%s (port %d)", name, t.Port)
				}
				if t.Domain != nil && t.Domain.Domain != "" {
					appInfo = fmt.Sprintf("%s @ %s", name, t.Domain.Domain)
				}
				otherApps = append(otherApps, appInfo)
			}
		}

		if len(otherApps) > 0 {
			successLines = append(successLines, "")
			successLines = append(successLines, deployMutedStyle.Render("Other apps on this server:"))
			for _, app := range otherApps {
				successLines = append(successLines, fmt.Sprintf("  • %s", deployMutedStyle.Render(app)))
			}
		}

		successBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("82")).
			Padding(0, 1).
			Render(lipgloss.JoinVertical(lipgloss.Left, successLines...))

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
	deployCmd.Flags().StringVar(&deployServerIP, "server-ip", "", "Deploy to an existing server (skips server provisioning)")
	deployCmd.Flags().BoolVar(&deployForceFlag, "force", false, "Force rerun all steps")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show deployment plan without executing")
	deployCmd.Flags().StringVar(&deployBuilderFlag, "builder", "", "Builder to use: native, nixpacks, or dockerfile (auto-detected if not specified)")
	deployCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	deployCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	deployCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during deployment")
}

// deployViaContainer handles deployment for container-based providers (e.g., fly.io)
func deployViaContainer(target config.TargetConfig, targetName, projectPath string, detection *detector.Detection) error {
	ctx := context.Background()

	// Route to fly.io deployer for container-based deployments
	if target.Provider == "flyio" {
		tokens, err := config.LoadTokens()
		if err != nil {
			return fmt.Errorf("failed to load tokens: %w", err)
		}

		token := tokens.GetToken("flyio")
		if token == "" {
			return fmt.Errorf("fly.io API token not found. Run 'lightfold config set-token flyio' first")
		}

		flyioConfig, err := target.GetFlyioConfig()
		if err != nil {
			return fmt.Errorf("failed to get fly.io config: %w", err)
		}

		projectName := util.GetTargetName(projectPath)
		deployer := deploy.NewFlyioDeployer(projectName, projectPath, targetName, detection, flyioConfig, token)

		fmt.Printf("%s %s\n", deployMutedStyle.Render("→"), deployMutedStyle.Render("Starting fly.io deployment..."))
		fmt.Println()

		if err := deployer.Deploy(ctx, target.Deploy); err != nil {
			state.MarkPushFailed(targetName, fmt.Sprintf("fly.io deployment failed: %v", err))
			return fmt.Errorf("deployment failed: %w", err)
		}

		// Update state
		currentCommit := getGitCommit(projectPath)
		if err := state.ClearPushFailure(targetName); err != nil {
			fmt.Printf("Warning: failed to clear push failure in state: %v\n", err)
		}
		if err := state.UpdateDeployment(targetName, currentCommit, time.Now().Format("20060102150405")); err != nil {
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
					deploySuccessStyle.Render(fmt.Sprintf("✓ Successfully deployed '%s' to fly.io", targetName)),
					"",
					fmt.Sprintf("%s %s", deployMutedStyle.Render("App:"), deployValueStyle.Render(flyioConfig.AppName)),
					fmt.Sprintf("%s %s", deployMutedStyle.Render("Region:"), deployValueStyle.Render(flyioConfig.Region)),
				),
			)

		fmt.Println(successBox)
		return nil
	}

	// Fallback for other container providers (if added in future)
	return fmt.Errorf("container deployment not implemented for provider: %s", target.Provider)
}
