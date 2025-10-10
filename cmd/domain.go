package cmd

import (
	"fmt"
	"lightfold/cmd/utils"
	"lightfold/pkg/config"
	"lightfold/pkg/proxy"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	domainTargetFlag string

	domainStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	domainLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	domainValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	domainErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	domainSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	domainMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

var domainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage custom domains for deployments",
	Long:  `Add, remove, or view custom domains for your deployment targets.`,
}

var domainAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a custom domain to a deployment target",
	Long: `Add a custom domain to a deployment target and optionally enable SSL with Let's Encrypt.

Examples:
  lightfold domain add --domain example.com              # Current directory
  lightfold domain add ~/Projects/myapp --domain app.com # Specific path
  lightfold domain add --target myapp --domain web.com   # Named target`,
	Run: func(cmd *cobra.Command, args []string) {
		domain := cmd.Flag("domain").Value.String()
		if domain == "" {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render("Error: --domain flag is required"))
			os.Exit(1)
		}

		if !isValidDomain(domain) {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error: Invalid domain format: %s", domain)))
			os.Exit(1)
		}

		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		cfg := loadConfigOrExit()
		target, targetName := resolveTarget(cfg, domainTargetFlag, pathArg)

		if !state.IsCreated(targetName) {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render("Error: Target infrastructure not created yet"))
			fmt.Fprintf(os.Stderr, "Run 'lightfold create' first\n")
			os.Exit(1)
		}

		if !state.IsConfigured(targetName) {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render("Error: Target not configured yet"))
			fmt.Fprintf(os.Stderr, "Run 'lightfold configure' first\n")
			os.Exit(1)
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error: %v", err)))
			os.Exit(1)
		}

		sshExecutor := sshpkg.NewExecutor(
			providerCfg.GetIP(),
			"22",
			providerCfg.GetUsername(),
			providerCfg.GetSSHKey(),
		)

		testResult := sshExecutor.Execute("echo 'connection test'")
		if testResult.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error: Cannot connect to server: %s", testResult.Stderr)))
			os.Exit(1)
		}

		// Prompt for SSL
		fmt.Printf("\n%s\n", domainStyle.Render("Domain Configuration"))
		fmt.Printf("  Domain: %s\n", domainValueStyle.Render(domain))
		fmt.Printf("  Target: %s\n", domainValueStyle.Render(targetName))
		fmt.Printf("  IP:     %s\n\n", domainValueStyle.Render(providerCfg.GetIP()))

		enableSSL := true
		fmt.Printf("Enable SSL? (Y/n): ")
		var sslResponse string
		fmt.Scanln(&sslResponse)
		if strings.ToLower(strings.TrimSpace(sslResponse)) == "n" {
			enableSSL = false
		}

		// Display DNS configuration instructions
		fmt.Printf("%s\n\n", domainLabelStyle.Render("Add the following A record to your domain registrar's DNS settings:"))

		dnsBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(0, 1).
			Foreground(lipgloss.Color("245"))

		dnsContent := fmt.Sprintf("  Type:  A\n  Name:  @ (or leave blank for root domain)\n  Value: %s\n  TTL:   3600 (or default)", providerCfg.GetIP())
		fmt.Printf("%s\n", dnsBoxStyle.Render(dnsContent))

		fmt.Printf("\n%s\n", domainMutedStyle.Render("For subdomains (e.g., app.example.com), use the subdomain name instead of '@'"))
		fmt.Printf("%s\n\n", domainMutedStyle.Render("DNS propagation typically takes 5-60 minutes."))

		fmt.Printf("Have you configured DNS? (Y/n): ")
		var dnsResponse string
		fmt.Scanln(&dnsResponse)
		// Default to yes - only skip if user explicitly says no
		if strings.ToLower(strings.TrimSpace(dnsResponse)) == "n" || strings.ToLower(strings.TrimSpace(dnsResponse)) == "no" {
			fmt.Printf("\n%s\n", domainErrorStyle.Render("Please configure DNS before continuing."))
			fmt.Printf("%s\n\n", domainMutedStyle.Render("Run this command again after DNS is configured."))
			os.Exit(1)
		}

		if target.Domain == nil {
			target.Domain = &config.DomainConfig{}
		}

		target.Domain.Domain = domain
		target.Domain.SSLEnabled = enableSSL
		if enableSSL {
			target.Domain.SSLManager = "certbot"
		}
		target.Domain.ProxyType = "nginx"

		cfg.SetTarget(targetName, target)
		if err := cfg.SaveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Printf("\n%s %s\n", domainSuccessStyle.Render("✓"), domainMutedStyle.Render("Domain configuration saved"))

		if err := configureDomainAndSSL(&target, targetName, domain, enableSSL); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error configuring domain: %v", err)))
			fmt.Fprintf(os.Stderr, "\nYou can retry with: lightfold domain add --domain %s --target %s\n", domain, targetName)
			os.Exit(1)
		}

		// Update state with SSL information
		targetState, err := state.GetTargetState(targetName)
		if err == nil && enableSSL {
			targetState.SSLConfigured = enableSSL
			targetState.LastSSLRenewal = time.Now()
			if err := state.SaveState(targetName, targetState); err != nil {
				fmt.Printf("Warning: failed to update SSL state: %v\n", err)
			}
		}

		fmt.Println()
	},
}

var domainRemoveCmd = &cobra.Command{
	Use:   "remove [path]",
	Short: "Remove custom domain from a deployment target",
	Long: `Remove custom domain configuration from a deployment target and revert to IP-based access.

Examples:
  lightfold domain remove              # Current directory
  lightfold domain remove ~/Projects/myapp # Specific path
  lightfold domain remove --target myapp   # Named target`,
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve target
		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		cfg := loadConfigOrExit()
		target, targetName := resolveTarget(cfg, domainTargetFlag, pathArg)

		if target.Domain == nil || target.Domain.Domain == "" {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render("Error: No domain configured for this target"))
			os.Exit(1)
		}

		currentDomain := target.Domain.Domain
		currentSSL := target.Domain.SSLEnabled

		fmt.Printf("\n%s\n", domainStyle.Render("Domain Removal"))
		fmt.Printf("  Domain: %s\n", domainValueStyle.Render(currentDomain))
		fmt.Printf("  Target: %s\n", domainValueStyle.Render(targetName))
		if currentSSL {
			fmt.Printf("  SSL:    %s\n\n", domainValueStyle.Render("Enabled"))
		} else {
			fmt.Printf("  SSL:    %s\n\n", domainValueStyle.Render("Disabled"))
		}

		fmt.Printf("Remove domain configuration? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Cancelled")
			return
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error: %v", err)))
			os.Exit(1)
		}

		// Test SSH connection
		sshExecutor := sshpkg.NewExecutor(
			providerCfg.GetIP(),
			"22",
			providerCfg.GetUsername(),
			providerCfg.GetSSHKey(),
		)

		testResult := sshExecutor.Execute("echo 'connection test'")
		if testResult.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error: Cannot connect to server: %s", testResult.Stderr)))
			os.Exit(1)
		}

		fmt.Printf("\n%s\n", domainStyle.Render("Reverting to IP-based configuration..."))

		if err := revertToIPBasedNginx(&target, targetName, sshExecutor); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error reverting configuration: %v", err)))
			os.Exit(1)
		}

		target.Domain = nil

		cfg.SetTarget(targetName, target)
		if err := cfg.SaveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", domainErrorStyle.Render(fmt.Sprintf("Error saving config: %v", err)))
			os.Exit(1)
		}

		fmt.Printf("\n%s\n", domainSuccessStyle.Render("✓ Domain removed successfully!"))
		fmt.Printf("%s\n", domainValueStyle.Render(fmt.Sprintf("Your app is now available at: http://%s", providerCfg.GetIP())))
		fmt.Println()
	},
}

var domainShowCmd = &cobra.Command{
	Use:   "show [path]",
	Short: "Show domain configuration for a deployment target",
	Long: `Display current domain and SSL configuration for a deployment target.

Examples:
  lightfold domain show              # Current directory
  lightfold domain show ~/Projects/myapp # Specific path
  lightfold domain show --target myapp   # Named target`,
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve target
		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		cfg := loadConfigOrExit()
		target, targetName := resolveTarget(cfg, domainTargetFlag, pathArg)

		fmt.Printf("\n%s\n", domainStyle.Render("Domain Configuration"))
		fmt.Printf("  Target: %s\n\n", domainValueStyle.Render(targetName))

		if target.Domain == nil || target.Domain.Domain == "" {
			fmt.Printf("%s\n\n", domainMutedStyle.Render("  No domain configured"))
			fmt.Printf("%s\n", domainMutedStyle.Render("  Run 'lightfold domain add --domain example.com' to add a domain"))
		} else {
			fmt.Printf("  %s:     %s\n", domainLabelStyle.Render("Domain"), domainValueStyle.Render(target.Domain.Domain))

			sslStatus := "Disabled"
			if target.Domain.SSLEnabled {
				sslStatus = "Enabled"
			}
			fmt.Printf("  %s:        %s\n", domainLabelStyle.Render("SSL"), domainValueStyle.Render(sslStatus))

			if target.Domain.SSLManager != "" {
				fmt.Printf("  %s: %s\n", domainLabelStyle.Render("SSL Manager"), domainValueStyle.Render(target.Domain.SSLManager))
			}

			if target.Domain.ProxyType != "" {
				fmt.Printf("  %s:  %s\n", domainLabelStyle.Render("Proxy Type"), domainValueStyle.Render(target.Domain.ProxyType))
			}

			// Show SSL renewal info from state
			if target.Domain.SSLEnabled {
				if targetState, err := state.GetTargetState(targetName); err == nil && !targetState.LastSSLRenewal.IsZero() {
					renewalTime := targetState.LastSSLRenewal.Format("2006-01-02 15:04:05")
					fmt.Printf("  %s: %s\n", domainLabelStyle.Render("Last Renewal"), domainValueStyle.Render(renewalTime))
				}
			}

			fmt.Println()
			if target.Domain.SSLEnabled {
				fmt.Printf("%s\n", domainValueStyle.Render(fmt.Sprintf("Your app is available at: https://%s", target.Domain.Domain)))
			} else {
				fmt.Printf("%s\n", domainValueStyle.Render(fmt.Sprintf("Your app is available at: http://%s", target.Domain.Domain)))
			}
		}
		fmt.Println()
	},
}

// isValidDomain performs basic domain validation
func isValidDomain(domain string) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	if strings.Contains(domain, " ") {
		return false
	}
	for _, char := range domain {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '-') {
			return false
		}
	}
	return true
}

// revertToIPBasedNginx reverts nginx configuration to IP-based access
func revertToIPBasedNginx(target *config.TargetConfig, targetName string, sshExecutor *sshpkg.Executor) error {
	proxyManager, err := proxy.GetManager("nginx")
	if err != nil {
		return fmt.Errorf("failed to get proxy manager: %w", err)
	}

	if nginxMgr, ok := proxyManager.(interface{ SetExecutor(*sshpkg.Executor) }); ok {
		nginxMgr.SetExecutor(sshExecutor)
	}

	port := utils.ExtractPortFromTarget(target, target.ProjectPath)

	appName := targetName

	// Configure nginx without domain (IP-based)
	proxyConfig := proxy.ProxyConfig{
		Domain:      "", // Empty domain means IP-based
		Port:        port,
		AppName:     appName,
		SSLEnabled:  false,
		SSLCertPath: "",
		SSLKeyPath:  "",
	}

	if err := proxyManager.Configure(proxyConfig); err != nil {
		return fmt.Errorf("failed to configure proxy: %w", err)
	}

	if err := proxyManager.Reload(); err != nil {
		return fmt.Errorf("failed to reload proxy: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(domainCmd)

	domainCmd.AddCommand(domainAddCmd)
	domainCmd.AddCommand(domainRemoveCmd)
	domainCmd.AddCommand(domainShowCmd)

	domainAddCmd.Flags().String("domain", "", "Domain name to configure (required)")
	domainAddCmd.MarkFlagRequired("domain")

	domainAddCmd.Flags().StringVarP(&domainTargetFlag, "target", "t", "", "Target name")
	domainRemoveCmd.Flags().StringVarP(&domainTargetFlag, "target", "t", "", "Target name")
	domainShowCmd.Flags().StringVarP(&domainTargetFlag, "target", "t", "", "Target name")
}
