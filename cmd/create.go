package cmd

import (
	"fmt"
	"lightfold/pkg/util"
	"os"

	"github.com/spf13/cobra"
)

var (
	targetName   string
	providerFlag string
	ipFlag       string
	sshKeyFlag   string
	userFlag     string
	regionFlag   string
	sizeFlag     string
	imageFlag    string
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

		var err error
		projectPath, err = util.ValidateProjectPath(projectPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if targetName == "" {
			targetName = util.GetTargetName(projectPath)
		}

		cfg := loadConfigOrExit()

		_, err = createTarget(targetName, projectPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Run 'lightfold configure --target %s' to configure the server.\n", targetName)
	},
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
