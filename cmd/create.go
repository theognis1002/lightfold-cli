package cmd

import (
	"context"
	"fmt"
	"lightfold/cmd/ui/sequential"
	"lightfold/pkg/config"
	"lightfold/pkg/deploy"
	"lightfold/pkg/detector"
	_ "lightfold/pkg/providers/digitalocean"
	_ "lightfold/pkg/providers/hetzner"
	"lightfold/pkg/state"
	sshpkg "lightfold/pkg/ssh"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	targetName      string
	providerFlag    string
	ipFlag          string
	sshKeyFlag      string
	userFlag        string
	regionFlag      string
	sizeFlag        string
	imageFlag       string
	provisionFlag   bool
)

var createCmd = &cobra.Command{
	Use:   "create [PROJECT_PATH]",
	Short: "Create infrastructure for deployment",
	Long: `Create the necessary infrastructure for your application deployment.

This command supports two modes:

1. BYOS (Bring Your Own Server) - Use existing infrastructure:
   lightfold create --target myapp --provider byos --ip 192.168.1.100 --ssh-key ~/.ssh/id_rsa --user deploy

2. Auto-provision - Create new infrastructure:
   lightfold create --target myapp --provider do --region nyc1 --size s-1vcpu-1gb

If no target name is provided, the current directory name will be used.`,
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

		// Determine target name
		if targetName == "" {
			targetName = filepath.Base(projectPath)
			if absPath, err := filepath.Abs(projectPath); err == nil {
				targetName = filepath.Base(absPath)
			}
		}

		// Check if target already created
		if state.IsCreated(targetName) {
			fmt.Printf("Target '%s' is already created. Use 'lightfold configure' to reconfigure.\n", targetName)
			os.Exit(0)
		}

		// Run framework detection
		fmt.Println("Detecting framework...")
		detection := detector.DetectFramework(projectPath)
		fmt.Printf("Detected: %s (%s)\n\n", detection.Framework, detection.Language)

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		targetConfig := config.TargetConfig{
			ProjectPath: projectPath,
			Framework:   detection.Framework,
		}

		// Determine provider and mode
		if providerFlag == "" {
			fmt.Fprintf(os.Stderr, "Error: --provider flag is required (byos, do, hetzner)\n")
			os.Exit(1)
		}

		provider := strings.ToLower(providerFlag)

		// BYOS mode
		if provider == "byos" {
			if err := handleBYOS(&targetConfig, targetName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Auto-provision mode
			if err := handleProvision(&targetConfig, targetName, projectPath, provider); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

		// Save config
		if err := cfg.SetTarget(targetName, targetConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving target config: %v\n", err)
			os.Exit(1)
		}

		if err := cfg.SaveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✅ Target '%s' created successfully!\n", targetName)
		fmt.Printf("Run 'lightfold configure --target %s' to configure the server.\n", targetName)
	},
}

func handleBYOS(targetConfig *config.TargetConfig, targetName string) error {
	// Validate required flags
	if ipFlag == "" {
		return fmt.Errorf("--ip flag is required for BYOS mode")
	}
	if sshKeyFlag == "" {
		return fmt.Errorf("--ssh-key flag is required for BYOS mode")
	}
	if userFlag == "" {
		userFlag = "root" // Default to root
	}

	fmt.Printf("Creating BYOS target with IP: %s\n", ipFlag)

	// Validate SSH connectivity
	fmt.Println("Validating SSH connection...")
	sshExecutor := sshpkg.NewExecutor(ipFlag, "22", userFlag, sshKeyFlag)
	defer sshExecutor.Disconnect()

	result := sshExecutor.Execute("echo 'SSH connection successful'")
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("SSH connection failed: %v", result.Error)
	}

	fmt.Println("✓ SSH connection validated")

	// Write created marker
	markerCmd := "sudo mkdir -p /etc/lightfold && echo 'created' | sudo tee /etc/lightfold/created > /dev/null"
	result = sshExecutor.Execute(markerCmd)
	if result.Error != nil || result.ExitCode != 0 {
		return fmt.Errorf("failed to write created marker: %v", result.Error)
	}

	// Store BYOS config (use digitalocean provider for SSH-based deployments)
	targetConfig.Provider = "digitalocean"
	doConfig := &config.DigitalOceanConfig{
		IP:          ipFlag,
		SSHKey:      sshKeyFlag,
		Username:    userFlag,
		Provisioned: false,
	}
	targetConfig.SetProviderConfig("digitalocean", doConfig)

	// Mark as created in state
	if err := state.MarkCreated(targetName, ""); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	return nil
}

func handleProvision(targetConfig *config.TargetConfig, targetName, projectPath, provider string) error {
	// Validate required flags for provisioning
	if regionFlag == "" {
		return fmt.Errorf("--region flag is required for provisioning")
	}
	if sizeFlag == "" {
		return fmt.Errorf("--size flag is required for provisioning")
	}
	if imageFlag == "" {
		imageFlag = "ubuntu-22-04-x64"
	}

	// Get API token
	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	token := tokens.GetToken(provider)
	if token == "" {
		// Prompt for token
		if provider == "do" || provider == "digitalocean" {
			doConfig, err := sequential.RunProvisionDigitalOceanFlow(targetName)
			if err != nil {
				return fmt.Errorf("failed to get DigitalOcean token: %w", err)
			}
			// Extract token from config (it's stored in the flow)
			tokens, _ = config.LoadTokens()
			token = tokens.GetDigitalOceanToken()
			if token == "" {
				return fmt.Errorf("no DigitalOcean API token provided")
			}
			// Update target config with region/size from flow
			targetConfig.Provider = "digitalocean"
			targetConfig.SetProviderConfig("digitalocean", doConfig)
		} else {
			return fmt.Errorf("provider %s not supported yet", provider)
		}
	} else {
		// Use existing token, set up config manually
		if provider == "do" || provider == "digitalocean" {
			targetConfig.Provider = "digitalocean"

			// Generate SSH key if not exists
			sshKeyPath := filepath.Join(os.Getenv("HOME"), ".lightfold", "keys", "lightfold_ed25519")
			if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
				publicKeyPath, err := sshpkg.GenerateKeyPair(sshKeyPath)
				if err != nil {
					return fmt.Errorf("failed to generate SSH key: %w", err)
				}
				_ = publicKeyPath  // We use the private key path for config
			}

			doConfig := &config.DigitalOceanConfig{
				Region:      regionFlag,
				Size:        sizeFlag,
				SSHKey:      sshKeyPath,
				Username:    "deploy",
				Provisioned: true,
			}
			targetConfig.SetProviderConfig("digitalocean", doConfig)
		} else {
			return fmt.Errorf("provider %s not supported yet", provider)
		}
	}

	// Create orchestrator and provision
	fmt.Printf("Provisioning %s server...\n", provider)

	orchestrator, err := deploy.GetOrchestrator(*targetConfig, projectPath, targetName)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := orchestrator.Deploy(ctx)
	if err != nil {
		return fmt.Errorf("provisioning failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("provisioning failed: %s", result.Message)
	}

	// Update config with server details
	cfg, _ := config.LoadConfig()
	updatedTarget, _ := cfg.GetTarget(targetName)
	*targetConfig = updatedTarget

	// Mark as created in state
	if result.Server != nil {
		if err := state.MarkCreated(targetName, result.Server.ID); err != nil {
			return fmt.Errorf("failed to update state: %w", err)
		}
	}

	fmt.Printf("✓ Server provisioned at %s\n", result.Server.PublicIPv4)

	return nil
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&targetName, "target", "", "Target name (defaults to current directory name)")
	createCmd.Flags().StringVar(&providerFlag, "provider", "", "Provider: byos, do, hetzner (required)")

	// BYOS flags
	createCmd.Flags().StringVar(&ipFlag, "ip", "", "Server IP address (for BYOS)")
	createCmd.Flags().StringVar(&sshKeyFlag, "ssh-key", "", "SSH private key path (for BYOS)")
	createCmd.Flags().StringVar(&userFlag, "user", "root", "SSH username (for BYOS)")

	// Provision flags
	createCmd.Flags().StringVar(&regionFlag, "region", "", "Region/location (for provisioning)")
	createCmd.Flags().StringVar(&sizeFlag, "size", "", "Server size/type (for provisioning)")
	createCmd.Flags().StringVar(&imageFlag, "image", "ubuntu-22-04-x64", "OS image (for provisioning)")

	createCmd.MarkFlagRequired("provider")
}
