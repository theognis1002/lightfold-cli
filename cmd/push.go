package cmd

import (
	"context"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"lightfold/pkg/detector"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
	"lightfold/pkg/util"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	pushEnvFile    string
	pushEnvVars    []string
	pushSkipBuild  bool
	pushDryRun     bool
	pushBranch     string
	pushTargetFlag string

	// Styles for push command (matching bubbletea/deploy)
	pushSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	pushMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	pushValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
)

var pushCmd = &cobra.Command{
	Use:   "push [PROJECT_PATH]",
	Short: "Deploy a new release to the target server",
	Long: `Push a new release of your application to the configured target.

This command:
- Creates a release tarball
- Uploads it to the server
- Builds the application
- Performs blue/green deployment with health checks
- Automatically rolls back on failure

Examples:
  lightfold push                         # Push current directory
  lightfold push ~/Projects/myapp        # Push specific project
  lightfold push --target myapp          # Push named target
  lightfold push --dry-run               # Preview deployment`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfigOrExit()

		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		target, targetNameResolved := resolveTarget(cfg, pushTargetFlag, pathArg)
		projectPath := target.ProjectPath

		if !state.IsCreated(targetNameResolved) {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' has not been created\n", targetNameResolved)
			fmt.Fprintf(os.Stderr, "Run 'lightfold create --target %s' first\n", targetNameResolved)
			os.Exit(1)
		}

		// Skip configuration check for container providers (Fly.io)
		if target.Provider != "flyio" && !state.IsConfigured(targetNameResolved) {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' has not been configured\n", targetNameResolved)
			fmt.Fprintf(os.Stderr, "Run 'lightfold configure --target %s' first\n", targetNameResolved)
			os.Exit(1)
		}

		currentCommit := getGitCommit(projectPath)
		lastCommit := state.GetLastCommit(targetNameResolved)

		if currentCommit != "" && currentCommit == lastCommit && !pushDryRun {
			fmt.Printf("No changes detected (commit: %s)\n", currentCommit[:7])
			fmt.Println("Use --force to push anyway")
			os.Exit(0)
		}

		// Process deployment options
		if err := target.ProcessDeploymentOptions(pushEnvFile, pushEnvVars, pushSkipBuild); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if pushDryRun {
			fmt.Println("DRY RUN - No changes will be made")
			fmt.Printf("Target: %s\n", targetNameResolved)
			fmt.Printf("Project: %s\n", target.ProjectPath)
			fmt.Printf("Framework: %s\n", target.Framework)
			fmt.Printf("Provider: %s\n", target.Provider)
			if currentCommit != "" {
				fmt.Printf("Current commit: %s\n", currentCommit[:7])
				if lastCommit != "" {
					fmt.Printf("Last deployed: %s\n", lastCommit[:7])
				}
			}
			fmt.Println("\nWould perform:")
			if target.Provider == "flyio" {
				fmt.Println("1. Generate fly.toml configuration")
				fmt.Println("2. Set secrets via flyctl")
				fmt.Println("3. Deploy with Fly.io nixpacks (remote build)")
				fmt.Println("4. Wait for health checks")
			} else {
				fmt.Println("1. Create release tarball")
				fmt.Println("2. Upload to server")
				fmt.Println("3. Build application")
				fmt.Println("4. Deploy with health check")
				fmt.Println("5. Auto-rollback on failure")
			}
			return
		}

		// Route to Fly.io deployer for container-based deployments
		if target.Provider == "flyio" {
			ctx := context.Background()
			tokens, err := config.LoadTokens()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading tokens: %v\n", err)
				os.Exit(1)
			}

			token := tokens.GetToken("flyio")
			if token == "" {
				fmt.Fprintf(os.Stderr, "Error: Fly.io API token not found\n")
				fmt.Fprintf(os.Stderr, "Run 'lightfold config set-token flyio' first\n")
				os.Exit(1)
			}

			flyioConfig, err := target.GetFlyioConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting Fly.io config: %v\n", err)
				os.Exit(1)
			}

			detection := detector.DetectFramework(target.ProjectPath)
			projectName := util.GetTargetName(target.ProjectPath)

			deployer := deploy.NewFlyioDeployer(projectName, target.ProjectPath, targetNameResolved, &detection, flyioConfig, token)

			fmt.Printf("%s %s\n", pushMutedStyle.Render("→"), pushMutedStyle.Render("Starting Fly.io deployment..."))

			if err := deployer.Deploy(ctx, target.Deploy); err != nil {
				state.MarkPushFailed(targetNameResolved, fmt.Sprintf("Fly.io deployment failed: %v", err))
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Update state
			if err := state.ClearPushFailure(targetNameResolved); err != nil {
				fmt.Printf("Warning: failed to clear push failure in state: %v\n", err)
			}
			if err := state.UpdateDeployment(targetNameResolved, currentCommit, time.Now().Format("20060102150405")); err != nil {
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
						pushSuccessStyle.Render(fmt.Sprintf("✓ Successfully deployed '%s' to Fly.io", targetNameResolved)),
						"",
						fmt.Sprintf("%s %s", pushMutedStyle.Render("App:"), pushValueStyle.Render(flyioConfig.AppName)),
						fmt.Sprintf("%s %s", pushMutedStyle.Render("Region:"), pushValueStyle.Render(flyioConfig.Region)),
					),
				)

			fmt.Println(successBox)
			return
		}

		// SSH-based deployment for traditional VPS providers
		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Ensure server state is initialized
		if err := updateServerStateFromTarget(&target, targetNameResolved); err != nil {
			fmt.Printf("Warning: failed to update server state: %v\n", err)
		}

		// Allocate port if not already set
		if target.Port == 0 {
			port, err := getOrAllocatePort(&target, targetNameResolved)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error allocating port: %v\n", err)
				os.Exit(1)
			}
			target.Port = port

			// Save port to config
			if err := cfg.SetTarget(targetNameResolved, target); err != nil {
				fmt.Printf("Warning: failed to save port to config: %v\n", err)
			}
			if err := cfg.SaveConfig(); err != nil {
				fmt.Printf("Warning: failed to save config: %v\n", err)
			}
		}

		detection := detector.DetectFramework(target.ProjectPath)

		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		defer sshExecutor.Disconnect()

		projectName := util.GetTargetName(target.ProjectPath)

		// Use custom deployment options if available
		var executor *deploy.Executor
		if target.Deploy != nil && (len(target.Deploy.BuildCommands) > 0 || len(target.Deploy.RunCommands) > 0) {
			executor = deploy.NewExecutorWithOptions(sshExecutor, projectName, target.ProjectPath, &detection, target.Deploy)
		} else {
			executor = deploy.NewExecutor(sshExecutor, projectName, target.ProjectPath, &detection)
		}

		tmpTarball := fmt.Sprintf("/tmp/lightfold-%s-release.tar.gz", projectName)
		if err := executor.CreateReleaseTarball(tmpTarball); err != nil {
			state.MarkPushFailed(targetNameResolved, fmt.Sprintf("failed to create tarball: %v", err))
			fmt.Fprintf(os.Stderr, "Error creating tarball: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(tmpTarball)
		fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Creating release tarball..."))

		releasePath, err := executor.UploadRelease(tmpTarball)
		if err != nil {
			state.MarkPushFailed(targetNameResolved, fmt.Sprintf("failed to upload release: %v", err))
			fmt.Fprintf(os.Stderr, "Error uploading release: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Uploading release to server..."))

		releaseTimestamp := filepath.Base(releasePath)

		if !target.Deploy.SkipBuild {
			if err := executor.BuildRelease(releasePath); err != nil {
				state.MarkPushFailed(targetNameResolved, fmt.Sprintf("failed to build release: %v", err))
				fmt.Fprintf(os.Stderr, "Error building release: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Building app..."))
		}

		if len(target.Deploy.EnvVars) > 0 {
			if err := executor.WriteEnvironmentFile(target.Deploy.EnvVars); err != nil {
				state.MarkPushFailed(targetNameResolved, fmt.Sprintf("failed to write environment file: %v", err))
				fmt.Fprintf(os.Stderr, "Error writing environment: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Configuring environment variables..."))
		}

		if err := executor.DeployWithHealthCheck(releasePath, 5, 3*time.Second); err != nil {
			state.MarkPushFailed(targetNameResolved, fmt.Sprintf("deployment failed: %v", err))
			fmt.Fprintf(os.Stderr, "Error during deployment: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Deploying and running health checks..."))

		if err := executor.CleanupOldReleases(cfg.KeepReleases); err != nil {
			fmt.Printf("Warning: failed to cleanup old releases: %v\n", err)
		}
		fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Cleaning up old releases..."))

		// Clear any previous push failure and update deployment state
		if err := state.ClearPushFailure(targetNameResolved); err != nil {
			fmt.Printf("Warning: failed to clear push failure in state: %v\n", err)
		}
		if err := state.UpdateDeployment(targetNameResolved, currentCommit, releaseTimestamp); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		// Register app with server state
		if err := registerAppWithServer(&target, targetNameResolved, target.Port, target.Framework); err != nil {
			fmt.Printf("Warning: failed to register app with server: %v\n", err)
		}

		fmt.Println()

		successBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("82")).
			Padding(0, 1).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					pushSuccessStyle.Render(fmt.Sprintf("✓ Successfully deployed '%s'", targetNameResolved)),
					"",
					fmt.Sprintf("%s %s", pushMutedStyle.Render("Server:"), pushValueStyle.Render(providerCfg.GetIP())),
					fmt.Sprintf("%s %s", pushMutedStyle.Render("Release:"), pushValueStyle.Render(releaseTimestamp)),
				),
			)

		fmt.Println(successBox)
	},
}

func getGitCommit(projectPath string) string {
	cmd := exec.Command("git", "-C", projectPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().StringVar(&pushTargetFlag, "target", "", "Target name (defaults to current directory)")
	pushCmd.Flags().StringVar(&pushEnvFile, "env-file", "", "Path to .env file")
	pushCmd.Flags().StringArrayVar(&pushEnvVars, "env", []string{}, "Environment variables (KEY=VALUE)")
	pushCmd.Flags().BoolVar(&pushSkipBuild, "skip-build", false, "Skip build step")
	pushCmd.Flags().BoolVar(&pushDryRun, "dry-run", false, "Show what would be done without executing")
	pushCmd.Flags().StringVar(&pushBranch, "branch", "main", "Git branch to deploy")
}
