package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"lightfold/pkg/detector"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
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

	// Styles for deploy command output
	deployHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	deployStepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	deploySuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	deployMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	deployValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
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

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		target, exists := cfg.GetTarget(deployTargetFlag)
		var projectPath string

		if !exists {
			fmt.Printf("Target '%s' not found. Creating...\n", deployTargetFlag)
			projectPath = "."
		} else {
			projectPath = target.ProjectPath
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
			fmt.Printf("Target: %s\n", deployTargetFlag)
			fmt.Printf("Steps:\n")
			if !exists || deployForceFlag {
				fmt.Println("  1. ✓ detect - Framework detection")
			} else {
				fmt.Println("  1. ⊘ detect - Skipped (cached)")
			}
			if !state.IsCreated(deployTargetFlag) || deployForceFlag {
				fmt.Println("  2. ✓ create - Infrastructure provisioning")
			} else {
				fmt.Println("  2. ⊘ create - Skipped (already created)")
			}
			if !state.IsConfigured(deployTargetFlag) || deployForceFlag {
				fmt.Println("  3. ✓ configure - Server configuration")
			} else {
				fmt.Println("  3. ⊘ configure - Skipped (already configured)")
			}
			fmt.Println("  4. ✓ push - Release deployment")
			return
		}

		fmt.Printf("\n%s\n", deployHeaderStyle.Render(fmt.Sprintf("🚀 Deploying target '%s'", deployTargetFlag)))
		fmt.Println(deployMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
		fmt.Println()

		var detection detector.Detection
		if !exists || deployForceFlag {
			fmt.Printf("%s\n", deployStepStyle.Render("Step 1/4: Framework detection"))
			detection = detector.DetectFramework(projectPath)
			fmt.Printf("  %s %s (%s)\n\n", deploySuccessStyle.Render("✓"), deployValueStyle.Render(detection.Framework), deployMutedStyle.Render(detection.Language))

			if !exists {
				target = config.TargetConfig{
					ProjectPath: projectPath,
					Framework:   detection.Framework,
				}
			}
		} else {
			fmt.Printf("%s\n", deployMutedStyle.Render("Step 1/4: Framework detection (cached)"))
			detection = detector.DetectFramework(projectPath)
			fmt.Println()
		}

		providerCfg, _ := target.GetAnyProviderConfig()
		hasIP := providerCfg != nil && providerCfg.GetIP() != ""

		if (!state.IsCreated(deployTargetFlag) && !hasIP) || deployForceFlag {
			fmt.Printf("%s\n", deployStepStyle.Render("Step 2/4: Infrastructure creation"))
			fmt.Fprintf(os.Stderr, "Error: Target not created. Run 'lightfold create --target %s' first\n", deployTargetFlag)
			os.Exit(1)
		} else {
			fmt.Printf("%s\n", deployMutedStyle.Render("Step 2/4: Infrastructure creation (skipped)"))
			fmt.Println()
		}

		if !state.IsConfigured(deployTargetFlag) || deployForceFlag {
			fmt.Printf("%s\n", deployStepStyle.Render("Step 3/4: Server configuration"))
			fmt.Fprintf(os.Stderr, "Error: Target not configured. Run 'lightfold configure --target %s' first\n", deployTargetFlag)
			os.Exit(1)
		} else {
			fmt.Printf("%s\n", deployMutedStyle.Render("Step 3/4: Server configuration (skipped)"))
			fmt.Println()
		}

		fmt.Printf("%s\n", deployStepStyle.Render("Step 4/4: Release deployment"))

		if target.Deploy == nil {
			target.Deploy = &config.DeploymentOptions{
				EnvVars: make(map[string]string),
			}
		}

		if envFile != "" {
			envVarsFromFile, err := loadEnvFile(envFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading env file: %v\n", err)
				os.Exit(1)
			}
			for k, v := range envVarsFromFile {
				target.Deploy.EnvVars[k] = v
			}
		}

		for _, envVar := range envVars {
			parts := splitEnvVar(envVar)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "Error: Invalid env var format '%s', expected KEY=VALUE\n", envVar)
				os.Exit(1)
			}
			target.Deploy.EnvVars[parts[0]] = parts[1]
		}

		target.Deploy.SkipBuild = skipBuild

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

		projectName := filepath.Base(projectPath)
		executor := deploy.NewExecutor(sshExecutor, projectName, projectPath, &detection)

		fmt.Printf("  %s Creating release package...\n", deployMutedStyle.Render("→"))
		tmpTarball := fmt.Sprintf("/tmp/lightfold-%s-release.tar.gz", projectName)
		if err := executor.CreateReleaseTarball(tmpTarball); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tarball: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(tmpTarball)
		fmt.Printf("  %s Created release package\n", deploySuccessStyle.Render("✓"))

		fmt.Printf("  %s Uploading to server...\n", deployMutedStyle.Render("→"))
		releasePath, err := executor.UploadRelease(tmpTarball)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error uploading release: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  %s Uploaded release\n", deploySuccessStyle.Render("✓"))

		if !target.Deploy.SkipBuild {
			fmt.Printf("  %s Building application...\n", deployMutedStyle.Render("→"))
			if err := executor.BuildRelease(releasePath); err != nil {
				fmt.Fprintf(os.Stderr, "Error building release: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("  %s Build completed\n", deploySuccessStyle.Render("✓"))
		}

		if len(target.Deploy.EnvVars) > 0 {
			fmt.Printf("  %s Writing environment variables...\n", deployMutedStyle.Render("→"))
			if err := executor.WriteEnvironmentFile(target.Deploy.EnvVars); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing environment: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("  %s Environment configured\n", deploySuccessStyle.Render("✓"))
		}

		fmt.Printf("  %s Deploying and running health checks...\n", deployMutedStyle.Render("→"))
		if err := executor.DeployWithHealthCheck(releasePath, 5, 3*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "Error during deployment: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  %s Deployment successful\n", deploySuccessStyle.Render("✓"))

		fmt.Printf("  %s Cleaning up old releases...\n", deployMutedStyle.Render("→"))
		executor.CleanupOldReleases(5)
		fmt.Printf("  %s Cleanup complete\n", deploySuccessStyle.Render("✓"))

		releaseTimestamp := filepath.Base(releasePath)
		currentCommit := getGitCommit(projectPath)
		if err := state.UpdateDeployment(deployTargetFlag, currentCommit, releaseTimestamp); err != nil {
			fmt.Printf("Warning: failed to update state: %v\n", err)
		}

		fmt.Println()
		fmt.Printf("%s\n", deploySuccessStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
		fmt.Printf("%s\n", deployHeaderStyle.Render(fmt.Sprintf("✅ Successfully deployed '%s'", deployTargetFlag)))
		fmt.Printf("  %s %s\n", deployMutedStyle.Render("Server:"), deployValueStyle.Render(sshProviderCfg.GetIP()))
		fmt.Printf("  %s %s\n", deployMutedStyle.Render("Release:"), deployValueStyle.Render(releaseTimestamp))
		fmt.Printf("%s\n", deploySuccessStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	},
}

func loadEnvFile(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	envVars := make(map[string]string)
	lines := splitLines(string(data))

	for i, line := range lines {
		line = trimSpace(line)
		if line == "" || startsWithHash(line) {
			continue
		}

		parts := splitEnvVar(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid env var at line %d: %s", i+1, line)
		}

		value := parts[1]
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		envVars[parts[0]] = value
	}

	return envVars, nil
}

func splitEnvVar(s string) []string {
	idx := -1
	for i, c := range s {
		if c == '=' {
			idx = i
			break
		}
	}
	if idx == -1 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

func startsWithHash(s string) bool {
	return len(s) > 0 && s[0] == '#'
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

	projectName := filepath.Base(projectPath)

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

	deployCmd.MarkFlagRequired("target")
}
