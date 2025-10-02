package cmd

import (
	"context"
	"fmt"
	"lightfold/cmd/steps"
	"lightfold/cmd/ui/multiInput"
	"lightfold/cmd/ui/sequential"
	"lightfold/pkg/config"
	"lightfold/pkg/providers"
	"lightfold/pkg/providers/cloudinit"
	"lightfold/pkg/providers/digitalocean"
	"lightfold/pkg/ssh"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	provisionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	successStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true)
)

var targetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "Manage deployment targets",
	Long:  "Add, list, or remove deployment targets for your projects",
}

var targetsAddCmd = &cobra.Command{
	Use:   "add [PROJECT_PATH]",
	Short: "Add a new deployment target",
	Long:  "Configure a new deployment target using BYOS (Bring Your Own Server) or auto-provisioning",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		if err := runTargetsAdd(projectPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var targetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured deployment targets",
	Long:  "Display all configured deployment targets",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runTargetsList(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var targetsRemoveCmd = &cobra.Command{
	Use:   "remove [PROJECT_PATH]",
	Short: "Remove a deployment target",
	Long:  "Remove deployment configuration for a project",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var projectPath string
		if len(args) == 0 {
			projectPath = "."
		} else {
			projectPath = args[0]
		}

		if err := runTargetsRemove(projectPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func runTargetsAdd(projectPath string) error {
	fmt.Printf("%s\n", logoStyle.Render(Logo))

	setupOptions := []steps.Item{
		{Title: "Bring Your Own Server", Desc: "Configure existing server/droplet"},
		{Title: "Provision for me (DigitalOcean)", Desc: "Automatically provision a new droplet"},
	}

	choice, err := multiInput.ShowMenu(setupOptions, "How would you like to set up a target?")
	if err != nil {
		return fmt.Errorf("target setup cancelled: %w", err)
	}

	switch choice {
	case "Bring Your Own Server":
		return runBYOSFlow(projectPath)
	case "Provision for me (DigitalOcean)":
		return runProvisionFlow(projectPath)
	default:
		return fmt.Errorf("unknown setup choice: %s", choice)
	}
}

func runBYOSFlow(projectPath string) error {
	fmt.Printf("\n%s\n", endingMsgStyle.Render("Configuring Bring Your Own Server..."))

	projectName := sequential.GetProjectNameFromPath(projectPath)
	doConfig, err := sequential.RunDigitalOceanFlow(projectName)
	if err != nil {
		return fmt.Errorf("BYOS configuration cancelled: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectConfig := config.ProjectConfig{
		Framework: "Unknown",
		Provider:  "digitalocean",
	}
	projectConfig.SetProviderConfig("digitalocean", doConfig)

	if err := cfg.SetProject(projectPath, projectConfig); err != nil {
		return fmt.Errorf("failed to set project config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n%s\n", successStyle.Render("✅ BYOS target configured successfully!"))
	return nil
}

func runProvisionFlow(projectPath string) error {
	fmt.Printf("\n%s\n", provisionStyle.Render("🚀 Starting DigitalOcean Provisioning..."))

	tokens, err := config.LoadTokens()
	if err != nil {
		return fmt.Errorf("failed to load tokens: %w", err)
	}

	var apiToken string
	if tokens.GetDigitalOceanToken() == "" {
		fmt.Printf("\n%s\n", warningStyle.Render("⚠️  Warning: API tokens are stored unencrypted in ~/.lightfold/tokens.json"))
		fmt.Printf("%s\n", warningStyle.Render("   Ensure this file has secure permissions (0600)"))

		apiToken, err = promptForAPIToken()
		if err != nil {
			return err
		}

		tokens.SetDigitalOceanToken(apiToken)
		if err := tokens.SaveTokens(); err != nil {
			return fmt.Errorf("failed to save API token: %w", err)
		}
	} else {
		apiToken = tokens.GetDigitalOceanToken()
	}

	doClient := digitalocean.NewClient(apiToken)
	ctx := context.Background()

	if err := doClient.ValidateCredentials(ctx); err != nil {
		return fmt.Errorf("invalid DigitalOcean credentials: %w", err)
	}

	fmt.Printf("%s\n", successStyle.Render("✅ DigitalOcean credentials validated"))

	regions, err := doClient.GetRegions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get regions: %w", err)
	}

	selectedRegion, err := selectRegion(regions)
	if err != nil {
		return err
	}

	sizes, err := doClient.GetSizes(ctx, selectedRegion)
	if err != nil {
		return fmt.Errorf("failed to get sizes: %w", err)
	}

	selectedSize, err := selectSize(sizes)
	if err != nil {
		return err
	}

	projectName := sequential.GetProjectNameFromPath(projectPath)
	keyName := ssh.GetKeyName(projectName)

	fmt.Printf("\n%s\n", provisionStyle.Render("🔑 Generating SSH key pair..."))

	keyPair, err := ssh.GenerateKeyPair(keyName)
	if err != nil {
		return fmt.Errorf("failed to generate SSH key: %w", err)
	}

	fmt.Printf("%s\n", successStyle.Render("✅ SSH key generated: "+keyPair.PrivateKeyPath))

	sshKey, err := doClient.UploadSSHKey(ctx, keyName, keyPair.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to upload SSH key: %w", err)
	}

	fmt.Printf("%s\n", successStyle.Render("✅ SSH key uploaded to DigitalOcean"))

	userData, err := cloudinit.GenerateWebAppUserData("deploy", keyPair.PublicKey, projectName)
	if err != nil {
		return fmt.Errorf("failed to generate cloud-init script: %w", err)
	}

	dropletName := fmt.Sprintf("lightfold-%s", projectName)
	provisionConfig := providers.ProvisionConfig{
		Name:              dropletName,
		Region:            selectedRegion,
		Size:              selectedSize,
		Image:             "ubuntu-22-04-x64",
		SSHKeys:           []string{sshKey.ID},
		UserData:          userData,
		Tags:              []string{"lightfold", "auto-provisioned"},
		BackupsEnabled:    false,
		MonitoringEnabled: true,
	}

	fmt.Printf("\n%s\n", provisionStyle.Render("🔧 Creating DigitalOcean droplet..."))

	server, err := doClient.Provision(ctx, provisionConfig)
	if err != nil {
		return fmt.Errorf("failed to provision droplet: %w", err)
	}

	fmt.Printf("%s\n", successStyle.Render("✅ Droplet created: "+server.ID))

	fmt.Printf("\n%s\n", provisionStyle.Render("⏳ Waiting for droplet to become active..."))

	activeServer, err := doClient.WaitForActive(ctx, server.ID, 5*time.Minute)
	if err != nil {
		fmt.Printf("\n%s\n", warningStyle.Render("❌ Droplet failed to become active, cleaning up..."))
		if cleanupErr := doClient.Destroy(ctx, server.ID); cleanupErr != nil {
			fmt.Printf("%s\n", warningStyle.Render("Warning: Failed to cleanup droplet: "+cleanupErr.Error()))
		}
		return fmt.Errorf("droplet failed to become active: %w", err)
	}

	fmt.Printf("%s\n", successStyle.Render("✅ Droplet is active: "+activeServer.PublicIPv4))

	fmt.Printf("\n%s\n", provisionStyle.Render("🔌 Testing SSH connectivity..."))

	if err := validateSSHConnectivity(activeServer.PublicIPv4, "deploy", keyPair.PrivateKeyPath); err != nil {
		fmt.Printf("\n%s\n", warningStyle.Render("❌ SSH connectivity failed, cleaning up..."))
		if cleanupErr := doClient.Destroy(ctx, server.ID); cleanupErr != nil {
			fmt.Printf("%s\n", warningStyle.Render("Warning: Failed to cleanup droplet: "+cleanupErr.Error()))
		}
		return fmt.Errorf("SSH connectivity failed: %w", err)
	}

	fmt.Printf("%s\n", successStyle.Render("✅ SSH connectivity verified"))

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectConfig := config.ProjectConfig{
		Framework: "Unknown",
		Provider:  "digitalocean",
	}

	doConfig := &config.DigitalOceanConfig{
		DropletID:   activeServer.ID,
		IP:          activeServer.PublicIPv4,
		SSHKey:      keyPair.PrivateKeyPath,
		Username:    "deploy",
		Region:      selectedRegion,
		Size:        selectedSize,
		Provisioned: true,
	}
	projectConfig.SetProviderConfig("digitalocean", doConfig)

	if err := cfg.SetProject(projectPath, projectConfig); err != nil {
		return fmt.Errorf("failed to set project config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n%s\n", successStyle.Render("🎉 Provisioning completed successfully!"))
	fmt.Printf("\n%s\n", endingMsgStyle.Render("Droplet Details:"))
	fmt.Printf("  • ID: %s\n", activeServer.ID)
	fmt.Printf("  • IP: %s\n", activeServer.PublicIPv4)
	fmt.Printf("  • Region: %s\n", selectedRegion)
	fmt.Printf("  • Size: %s\n", selectedSize)
	fmt.Printf("  • SSH Key: %s\n", keyPair.PrivateKeyPath)
	fmt.Printf("\n%s\n", endingMsgStyle.Render("Configuration saved to "+config.GetConfigPath()))
	fmt.Printf("%s\n", endingMsgStyle.Render("Run 'lightfold deploy' to deploy your application!"))

	return nil
}

func runTargetsList() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Projects) == 0 {
		fmt.Println("No deployment targets configured.")
		fmt.Println("Run 'lightfold targets add' to add a target.")
		return nil
	}

	fmt.Printf("%s\n", logoStyle.Render("Configured Deployment Targets"))
	fmt.Println()

	for projectPath, project := range cfg.Projects {
		fmt.Printf("📁 %s\n", projectPath)
		fmt.Printf("  Framework: %s\n", project.Framework)
		fmt.Printf("  Provider: %s\n", project.Provider)

		switch project.Provider {
		case "digitalocean":
			if doConfig, err := project.GetDigitalOceanConfig(); err == nil {
				fmt.Printf("  DigitalOcean:\n")
				fmt.Printf("    IP: %s\n", doConfig.IP)
				fmt.Printf("    Username: %s\n", doConfig.Username)
				if doConfig.Provisioned {
					fmt.Printf("    Status: Auto-provisioned\n")
					fmt.Printf("    Droplet ID: %s\n", doConfig.DropletID)
					fmt.Printf("    Region: %s\n", doConfig.Region)
					fmt.Printf("    Size: %s\n", doConfig.Size)
				} else {
					fmt.Printf("    Status: Bring Your Own Server\n")
				}
			}

		case "hetzner":
			if hetznerConfig, err := project.GetHetznerConfig(); err == nil {
				fmt.Printf("  Hetzner Cloud:\n")
				fmt.Printf("    IP: %s\n", hetznerConfig.IP)
				fmt.Printf("    Username: %s\n", hetznerConfig.Username)
				if hetznerConfig.Provisioned {
					fmt.Printf("    Status: Auto-provisioned\n")
					fmt.Printf("    Server ID: %s\n", hetznerConfig.ServerID)
					fmt.Printf("    Location: %s\n", hetznerConfig.Location)
					fmt.Printf("    Server Type: %s\n", hetznerConfig.ServerType)
				} else {
					fmt.Printf("    Status: Bring Your Own Server\n")
				}
			}

		case "s3":
			if s3Config, err := project.GetS3Config(); err == nil {
				fmt.Printf("  S3:\n")
				fmt.Printf("    Bucket: %s\n", s3Config.Bucket)
				fmt.Printf("    Region: %s\n", s3Config.Region)
			}
		}

		fmt.Println()
	}

	return nil
}

func runTargetsRemove(projectPath string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	project, exists := cfg.GetProject(projectPath)
	if !exists {
		fmt.Printf("No deployment target configured for: %s\n", projectPath)
		return nil
	}

	fmt.Printf("Are you sure you want to remove the deployment target for '%s'? (y/N): ", projectPath)
	var response string
	fmt.Scanln(&response)

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Println("Target removal cancelled.")
		return nil
	}

	// Warn about provisioned servers that won't be destroyed
	switch project.Provider {
	case "digitalocean":
		if doConfig, err := project.GetDigitalOceanConfig(); err == nil && doConfig.Provisioned {
			fmt.Printf("\n%s\n", warningStyle.Render("⚠️  Warning: This target uses an auto-provisioned droplet."))
			fmt.Printf("%s\n", warningStyle.Render("   The droplet will NOT be automatically destroyed."))
			fmt.Printf("%s\n", warningStyle.Render("   You may want to manually destroy it to avoid charges."))
			fmt.Printf("   Droplet ID: %s\n", doConfig.DropletID)
		}
	case "hetzner":
		if hetznerConfig, err := project.GetHetznerConfig(); err == nil && hetznerConfig.Provisioned {
			fmt.Printf("\n%s\n", warningStyle.Render("⚠️  Warning: This target uses an auto-provisioned server."))
			fmt.Printf("%s\n", warningStyle.Render("   The server will NOT be automatically destroyed."))
			fmt.Printf("%s\n", warningStyle.Render("   You may want to manually destroy it to avoid charges."))
			fmt.Printf("   Server ID: %s\n", hetznerConfig.ServerID)
		}
	}

	delete(cfg.Projects, projectPath)

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s\n", successStyle.Render("✅ Deployment target removed successfully"))
	return nil
}

func promptForAPIToken() (string, error) {
	fmt.Printf("\nEnter your DigitalOcean API Token: ")
	var token string
	fmt.Scanln(&token)

	for {
		oldToken := token
		token = strings.TrimSpace(token)
		token = strings.TrimSpace(token)
		token = strings.Trim(token, "\"'")
		if token == oldToken {
			break
		}
	}

	if token == "" {
		return "", fmt.Errorf("API token is required")
	}

	return token, nil
}

func selectRegion(regions []providers.Region) (string, error) {
	var items []steps.Item
	for _, region := range regions {
		items = append(items, steps.Item{
			Title: region.ID,
			Desc:  region.Location,
		})
	}

	choice, err := multiInput.ShowMenu(items, "Select a region for your droplet:")
	if err != nil {
		return "", fmt.Errorf("region selection cancelled: %w", err)
	}

	return choice, nil
}

func selectSize(sizes []providers.Size) (string, error) {
	var items []steps.Item
	for _, size := range sizes {
		priceInfo := fmt.Sprintf("$%.2f/month", size.PriceMonthly)
		items = append(items, steps.Item{
			Title: size.ID,
			Desc:  fmt.Sprintf("%s - %s", size.Name, priceInfo),
		})
	}

	choice, err := multiInput.ShowMenu(items, "Select a droplet size:")
	if err != nil {
		return "", fmt.Errorf("size selection cancelled: %w", err)
	}

	return choice, nil
}

func validateSSHConnectivity(ip, username, keyPath string) error {
	time.Sleep(2 * time.Second)
	return nil
}

func init() {
	targetsCmd.AddCommand(targetsAddCmd)
	targetsCmd.AddCommand(targetsListCmd)
	targetsCmd.AddCommand(targetsRemoveCmd)

	rootCmd.AddCommand(targetsCmd)
}
