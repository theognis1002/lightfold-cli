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
  lightfold push --target myapp
  lightfold push . --target myapp --env-file .env.production
  lightfold push --target myapp --dry-run`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		// Validate project path
		var err error
		projectPath, err = util.ValidateProjectPath(projectPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cfg := loadConfigOrExit()

		var target config.TargetConfig
		var targetNameResolved string

		if pushTargetFlag != "" {
			target = loadTargetOrExit(cfg, pushTargetFlag)
			targetNameResolved = pushTargetFlag
		} else {
			var exists bool
			targetNameResolved, target, exists = cfg.FindTargetByPath(projectPath)
			if !exists {
				targetNameResolved = util.GetTargetName(projectPath)
				target, exists = cfg.GetTarget(targetNameResolved)
				if !exists {
					fmt.Fprintf(os.Stderr, "Error: No target found for this project\n")
					fmt.Fprintf(os.Stderr, "Run 'lightfold create --target %s' first\n", targetNameResolved)
					os.Exit(1)
				}
			}
		}

		if !state.IsCreated(targetNameResolved) {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' has not been created\n", targetNameResolved)
			fmt.Fprintf(os.Stderr, "Run 'lightfold create --target %s' first\n", targetNameResolved)
			os.Exit(1)
		}

		if !state.IsConfigured(targetNameResolved) {
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
			fmt.Println("1. Create release tarball")
			fmt.Println("2. Upload to server")
			fmt.Println("3. Build application")
			fmt.Println("4. Deploy with health check")
			fmt.Println("5. Auto-rollback on failure")
			return
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		detection := detector.DetectFramework(target.ProjectPath)

		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		defer sshExecutor.Disconnect()

		projectName := util.GetTargetName(target.ProjectPath)
		executor := deploy.NewExecutor(sshExecutor, projectName, target.ProjectPath, &detection)

		tmpTarball := fmt.Sprintf("/tmp/lightfold-%s-release.tar.gz", projectName)
		if err := executor.CreateReleaseTarball(tmpTarball); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tarball: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(tmpTarball)
		fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Creating release tarball..."))

		releasePath, err := executor.UploadRelease(tmpTarball)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error uploading release: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Uploading release to server..."))

		releaseTimestamp := filepath.Base(releasePath)

		if !target.Deploy.SkipBuild {
			if err := executor.BuildRelease(releasePath); err != nil {
				fmt.Fprintf(os.Stderr, "Error building release: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Building app..."))
		}

		if len(target.Deploy.EnvVars) > 0 {
			if err := executor.WriteEnvironmentFile(target.Deploy.EnvVars); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing environment: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Configuring environment variables..."))
		}

		if err := executor.DeployWithHealthCheck(releasePath, 5, 3*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "Error during deployment: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Deploying and running health checks..."))

		if err := executor.CleanupOldReleases(5); err != nil {
			fmt.Printf("Warning: failed to cleanup old releases: %v\n", err)
		}
		fmt.Printf("%s %s\n", pushSuccessStyle.Render("✓"), pushMutedStyle.Render("Cleaning up old releases..."))

		if err := state.UpdateDeployment(targetNameResolved, currentCommit, releaseTimestamp); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		fmt.Println()

		// Success banner with cleaner bubbletea style
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
