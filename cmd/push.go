package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"lightfold/pkg/detector"
	"lightfold/pkg/state"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/util"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	pushEnvFile    string
	pushEnvVars    []string
	pushSkipBuild  bool
	pushDryRun     bool
	pushBranch     string
	pushTargetFlag string
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
				targetNameResolved = filepath.Base(projectPath)
				if absPath, err := filepath.Abs(projectPath); err == nil {
					targetNameResolved = filepath.Base(absPath)
				}
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

		fmt.Printf("Pushing release to target '%s'...\n", targetNameResolved)

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Detecting framework...")
		detection := detector.DetectFramework(target.ProjectPath)

		fmt.Printf("Connecting to %s...\n", providerCfg.GetIP())
		sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
		defer sshExecutor.Disconnect()

		projectName := filepath.Base(target.ProjectPath)
		executor := deploy.NewExecutor(sshExecutor, projectName, target.ProjectPath, &detection)

		fmt.Println("Creating release tarball...")
		tmpTarball := fmt.Sprintf("/tmp/lightfold-%s-release.tar.gz", projectName)
		if err := executor.CreateReleaseTarball(tmpTarball); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tarball: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(tmpTarball)

		fmt.Println("Uploading release...")
		releasePath, err := executor.UploadRelease(tmpTarball)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error uploading release: %v\n", err)
			os.Exit(1)
		}

		releaseTimestamp := filepath.Base(releasePath)

		if !target.Deploy.SkipBuild {
			fmt.Println("Building application...")
			if err := executor.BuildRelease(releasePath); err != nil {
				fmt.Fprintf(os.Stderr, "Error building release: %v\n", err)
				os.Exit(1)
			}
		}

		if len(target.Deploy.EnvVars) > 0 {
			fmt.Println("Writing environment variables...")
			if err := executor.WriteEnvironmentFile(target.Deploy.EnvVars); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing environment: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Println("Deploying with health check...")
		if err := executor.DeployWithHealthCheck(releasePath, 5, 3*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "Error during deployment: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Cleaning up old releases...")
		if err := executor.CleanupOldReleases(5); err != nil {
			fmt.Printf("Warning: failed to cleanup old releases: %v\n", err)
		}

		if err := state.UpdateDeployment(targetNameResolved, currentCommit, releaseTimestamp); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		fmt.Printf("\n✅ Successfully deployed release %s\n", releaseTimestamp)
		fmt.Printf("Server: %s\n", providerCfg.GetIP())
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
