package cmd

import (
	"context"
	"fmt"
	tui "lightfold/cmd/ui"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	sshpkg "lightfold/pkg/ssh"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	envFile      string
	envVars      []string
	skipBuild    bool
	rollbackFlag bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy [PROJECT_PATH]",
	Short: "Deploy your application to the configured target",
	Long: `Deploy your application using the configuration stored in ~/.lightfold/config.json.

If no configuration exists for the project, you'll be prompted to set it up first.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		projectPath = filepath.Clean(projectPath)

		info, err := os.Stat(projectPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Cannot access path '%s': %v\n", projectPath, err)
			os.Exit(1)
		}

		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: Path '%s' is not a directory\n", projectPath)
			os.Exit(1)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		project, exists := cfg.GetProject(projectPath)

		// Handle rollback if requested
		if rollbackFlag {
			if !exists {
				fmt.Println("No deployment configuration found for this project.")
				os.Exit(1)
			}
			handleRollback(project, projectPath)
			return
		}

		if !exists {
			if len(cfg.Projects) == 1 && len(args) == 0 {
				for configuredPath, configuredProject := range cfg.Projects {
					fmt.Printf("No configuration found for current directory.\n")
					fmt.Printf("Using configured project: %s\n", configuredPath)
					projectPath = configuredPath
					project = configuredProject
					exists = true
					break
				}
			}

			if !exists {
				fmt.Println("No deployment configuration found for this project.")
				if len(cfg.Projects) > 0 {
					fmt.Println("Configured projects:")
					for path := range cfg.Projects {
						fmt.Printf("  - %s\n", path)
					}
					fmt.Println("Run 'lightfold deploy <PROJECT_PATH>' to deploy a specific project,")
					fmt.Println("or 'lightfold <PROJECT_PATH>' to configure this project.")
				} else {
					fmt.Println("Please run 'lightfold .' first to detect and configure the project.")
				}
				os.Exit(1)
			}
		}

		if err := validateProjectConfig(project); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid project configuration: %v\n", err)
			fmt.Println("Please run 'lightfold .' to reconfigure the project.")
			os.Exit(1)
		}

		// Process deployment flags
		if err := processDeploymentFlags(&project, projectPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing deployment options: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Deploying %s project to %s...\n", project.Framework, project.Provider)
		fmt.Println()

		// Create deployment orchestrator
		projectName := filepath.Base(projectPath)
		orchestrator, err := deploy.NewOrchestrator(project, projectPath, projectName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating deployment orchestrator: %v\n", err)
			os.Exit(1)
		}

		// Execute deployment with progress display
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := tui.ShowDeploymentProgressWithOrchestrator(ctx, orchestrator); err != nil {
			fmt.Fprintf(os.Stderr, "Error during deployment: %v\n", err)
			os.Exit(1)
		}
	},
}

func validateProjectConfig(project config.ProjectConfig) error {
	// Validate provider is specified
	if project.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	// Validate provider-specific configuration
	switch project.Provider {
	case "digitalocean":
		doConfig, err := project.GetDigitalOceanConfig()
		if err != nil {
			return fmt.Errorf("DigitalOcean configuration is missing: %w", err)
		}

		// For provisioned droplets, IP may be empty (will be created during deployment)
		// For BYOS, IP is required
		if !doConfig.Provisioned && doConfig.IP == "" {
			return fmt.Errorf("IP address is required for BYOS deployments")
		}

		if doConfig.Username == "" {
			return fmt.Errorf("username is required")
		}

		if doConfig.SSHKey == "" {
			return fmt.Errorf("SSH key is required")
		}

		// For provisioned droplets, verify we have region and size
		if doConfig.Provisioned {
			if doConfig.Region == "" {
				return fmt.Errorf("region is required for provisioned deployments")
			}
			if doConfig.Size == "" {
				return fmt.Errorf("size is required for provisioned deployments")
			}
		}

	case "hetzner":
		hetznerConfig, err := project.GetHetznerConfig()
		if err != nil {
			return fmt.Errorf("Hetzner configuration is missing: %w", err)
		}

		if !hetznerConfig.Provisioned && hetznerConfig.IP == "" {
			return fmt.Errorf("IP address is required for BYOS deployments")
		}

		if hetznerConfig.Username == "" {
			return fmt.Errorf("username is required")
		}

		if hetznerConfig.SSHKey == "" {
			return fmt.Errorf("SSH key is required")
		}

		if hetznerConfig.Provisioned {
			if hetznerConfig.Location == "" {
				return fmt.Errorf("location is required for provisioned deployments")
			}
			if hetznerConfig.ServerType == "" {
				return fmt.Errorf("server type is required for provisioned deployments")
			}
		}

	case "s3":
		s3Config, err := project.GetS3Config()
		if err != nil {
			return fmt.Errorf("S3 configuration is missing: %w", err)
		}
		if s3Config.Bucket == "" {
			return fmt.Errorf("S3 bucket name is required")
		}

	default:
		return fmt.Errorf("unknown provider: %s", project.Provider)
	}
	return nil
}

func processDeploymentFlags(project *config.ProjectConfig, projectPath string) error {
	// Initialize deployment options if needed
	if project.Deploy == nil {
		project.Deploy = &config.DeploymentOptions{
			EnvVars: make(map[string]string),
		}
	}

	// Process skip-build flag
	if skipBuild {
		project.Deploy.SkipBuild = true
	}

	// Process env-file flag
	if envFile != "" {
		envVarsFromFile, err := loadEnvFile(envFile)
		if err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}
		for k, v := range envVarsFromFile {
			project.Deploy.EnvVars[k] = v
		}
	}

	// Process env KEY=VALUE flags
	for _, envVar := range envVars {
		parts := splitEnvVar(envVar)
		if len(parts) != 2 {
			return fmt.Errorf("invalid env var format '%s', expected KEY=VALUE", envVar)
		}
		project.Deploy.EnvVars[parts[0]] = parts[1]
	}

	// Save updated config with deployment options
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetProject(projectPath, *project); err != nil {
		return fmt.Errorf("failed to update project config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func loadEnvFile(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	envVars := make(map[string]string)
	lines := splitLines(string(data))

	for i, line := range lines {
		// Skip empty lines and comments
		line = trimSpace(line)
		if line == "" || startsWithHash(line) {
			continue
		}

		parts := splitEnvVar(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid env var at line %d: %s", i+1, line)
		}

		// Remove surrounding quotes if present
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

func handleRollback(project config.ProjectConfig, projectPath string) {
	fmt.Println("Rolling back to previous release...")

	// Rollback only works for SSH-based providers (not S3)
	if project.Provider == "s3" {
		fmt.Fprintf(os.Stderr, "Error: Rollback is not supported for S3 deployments\n")
		os.Exit(1)
	}

	// Get provider config (works for any SSH-based provider)
	var providerCfg config.ProviderConfig
	var err error

	switch project.Provider {
	case "digitalocean":
		providerCfg, err = project.GetDigitalOceanConfig()
	case "hetzner":
		providerCfg, err = project.GetHetznerConfig()
	default:
		fmt.Fprintf(os.Stderr, "Error: Rollback is not supported for provider: %s\n", project.Provider)
		os.Exit(1)
	}

	if err != nil || providerCfg.GetIP() == "" {
		fmt.Fprintf(os.Stderr, "Error: No server IP found in configuration\n")
		os.Exit(1)
	}

	projectName := filepath.Base(projectPath)

	// Connect to server
	fmt.Printf("Connecting to server at %s...\n", providerCfg.GetIP())
	sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
	defer sshExecutor.Disconnect()

	// Create deployment executor
	executor := deploy.NewExecutor(sshExecutor, projectName, projectPath, nil)

	// Perform rollback
	if err := executor.RollbackToPreviousRelease(); err != nil {
		fmt.Fprintf(os.Stderr, "Error during rollback: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Successfully rolled back to previous release")
}

func init() {
	rootCmd.AddCommand(deployCmd)

	// Add deployment flags
	deployCmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file with environment variables")
	deployCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")
	deployCmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip the build step during deployment")
	deployCmd.Flags().BoolVar(&rollbackFlag, "rollback", false, "Rollback to the previous release")
}
