package cmd

import (
	"fmt"
	"lightfold/pkg/state"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	serverHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	serverLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	serverValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	serverMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	serverErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage servers and their deployed applications",
	Long: `View and manage servers with their deployed applications.

The server command provides a server-centric view of your deployments,
showing all applications deployed to each server. This is useful when
deploying multiple applications to a single server.

Examples:
  lightfold server list              # List all servers and their apps
  lightfold server show <server-ip>  # Show detailed info for a server`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default to list if no subcommand provided
		cmd.Help()
	},
}

// serverListCmd lists all servers and their deployed applications
var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all servers and their deployed applications",
	Long: `Display a list of all servers tracked by Lightfold along with
the applications deployed to each server.

This provides a server-centric view, which is particularly useful when
managing multiple applications on shared infrastructure.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get all servers from state files
		servers, err := state.ListAllServers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", serverErrorStyle.Render(fmt.Sprintf("Error loading servers: %v", err)))
			os.Exit(1)
		}

		if len(servers) == 0 {
			fmt.Println(serverMutedStyle.Render("No servers found."))
			fmt.Printf("\n%s\n", serverMutedStyle.Render("Deploy an application to create a server:"))
			fmt.Printf("  %s\n", serverValueStyle.Render("lightfold deploy"))
			return
		}

		fmt.Printf("%s\n", serverHeaderStyle.Render(fmt.Sprintf("Servers (%d):", len(servers))))
		fmt.Println(serverMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))

		for _, serverIP := range servers {
			serverState, err := state.GetServerState(serverIP)
			if err != nil {
				fmt.Printf("\n%s: %s\n", serverLabelStyle.Render(serverIP), serverErrorStyle.Render(fmt.Sprintf("Error: %v", err)))
				continue
			}

			fmt.Printf("\n%s\n", serverLabelStyle.Render(serverIP))
			if serverState.Provider != "" {
				fmt.Printf("  Provider:   %s\n", serverValueStyle.Render(serverState.Provider))
			}
			if serverState.ProxyType != "" {
				fmt.Printf("  Proxy:      %s\n", serverValueStyle.Render(serverState.ProxyType))
			}
			if serverState.RootDomain != "" {
				fmt.Printf("  Domain:     %s\n", serverValueStyle.Render(serverState.RootDomain))
			}

			appCount := len(serverState.DeployedApps)
			fmt.Printf("  Apps:       %s\n", serverValueStyle.Render(fmt.Sprintf("%d deployed", appCount)))

			if appCount > 0 {
				fmt.Printf("\n  %s\n", serverLabelStyle.Render("Deployed Applications:"))
				for _, app := range serverState.DeployedApps {
					fmt.Printf("    • %s", serverValueStyle.Render(app.TargetName))
					if app.Framework != "" {
						fmt.Printf(" (%s)", serverMutedStyle.Render(app.Framework))
					}
					if app.Port > 0 {
						fmt.Printf(" - Port %s", serverValueStyle.Render(fmt.Sprintf("%d", app.Port)))
					}
					if app.Domain != "" {
						fmt.Printf(" - %s", serverValueStyle.Render(app.Domain))
					}
					fmt.Println()

					if !app.LastDeploy.IsZero() {
						fmt.Printf("      Last deployed: %s\n", serverMutedStyle.Render(app.LastDeploy.Format("2006-01-02 15:04")))
					}
				}
			}

			// Show port usage statistics
			used, available, err := state.GetPortStatistics(serverIP)
			if err == nil {
				fmt.Printf("\n  Port Usage: %s used, %s available\n",
					serverValueStyle.Render(fmt.Sprintf("%d", used)),
					serverMutedStyle.Render(fmt.Sprintf("%d", available)))
			}

			// Check for conflicts
			conflicts, err := state.DetectPortConflicts(serverIP)
			if err == nil && len(conflicts) > 0 {
				fmt.Printf("\n  %s\n", serverErrorStyle.Render("⚠ Port Conflicts Detected:"))
				for _, conflict := range conflicts {
					fmt.Printf("    %s\n", serverErrorStyle.Render(conflict))
				}
			}
		}

		// Show targets that match these servers
		fmt.Printf("\n%s\n", serverMutedStyle.Render("For detailed server information: lightfold server show <server-ip>"))
	},
}

// serverShowCmd shows detailed information about a specific server
var serverShowCmd = &cobra.Command{
	Use:   "show <server-ip>",
	Short: "Show detailed information about a specific server",
	Long: `Display detailed information about a specific server including:
  • Server metadata (provider, proxy type, domains)
  • All deployed applications
  • Port allocation status
  • Resource usage
  • Associated targets`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serverIP := args[0]
		cfg := loadConfigOrExit()

		// Load server state
		serverState, err := state.GetServerState(serverIP)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", serverErrorStyle.Render(fmt.Sprintf("Error loading server state: %v", err)))
			os.Exit(1)
		}

		if !state.ServerStateExists(serverIP) {
			fmt.Fprintf(os.Stderr, "%s\n", serverErrorStyle.Render(fmt.Sprintf("Server %s not found", serverIP)))
			fmt.Fprintf(os.Stderr, "\nRun 'lightfold server list' to see all servers\n")
			os.Exit(1)
		}

		// Display server information
		fmt.Printf("%s %s\n", serverHeaderStyle.Render("Server:"), serverLabelStyle.Render(serverIP))
		fmt.Printf("%s\n\n", serverMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))

		fmt.Printf("%s\n", serverHeaderStyle.Render("Configuration:"))
		if serverState.Provider != "" {
			fmt.Printf("  Provider:    %s\n", serverValueStyle.Render(serverState.Provider))
		}
		if serverState.ServerID != "" {
			fmt.Printf("  Server ID:   %s\n", serverValueStyle.Render(serverState.ServerID))
		}
		if serverState.ProxyType != "" {
			fmt.Printf("  Proxy:       %s\n", serverValueStyle.Render(serverState.ProxyType))
		}
		if serverState.RootDomain != "" {
			fmt.Printf("  Root Domain: %s\n", serverValueStyle.Render(serverState.RootDomain))
		}
		if !serverState.CreatedAt.IsZero() {
			fmt.Printf("  Created:     %s\n", serverValueStyle.Render(serverState.CreatedAt.Format("2006-01-02 15:04:05")))
		}
		if !serverState.UpdatedAt.IsZero() {
			fmt.Printf("  Updated:     %s\n", serverValueStyle.Render(serverState.UpdatedAt.Format("2006-01-02 15:04:05")))
		}
		fmt.Println()

		// Port statistics
		used, available, err := state.GetPortStatistics(serverIP)
		if err == nil {
			fmt.Printf("%s\n", serverHeaderStyle.Render("Port Allocation:"))
			fmt.Printf("  Range:       %s - %s\n",
				serverValueStyle.Render(fmt.Sprintf("%d", state.PortRangeStart)),
				serverValueStyle.Render(fmt.Sprintf("%d", state.PortRangeEnd)))
			fmt.Printf("  Used:        %s\n", serverValueStyle.Render(fmt.Sprintf("%d", used)))
			fmt.Printf("  Available:   %s\n", serverValueStyle.Render(fmt.Sprintf("%d", available)))
			fmt.Printf("  Next Port:   %s\n", serverValueStyle.Render(fmt.Sprintf("%d", serverState.NextPort)))
			fmt.Println()
		}

		// Check for port conflicts
		conflicts, err := state.DetectPortConflicts(serverIP)
		if err == nil && len(conflicts) > 0 {
			fmt.Printf("%s\n", serverHeaderStyle.Render("Port Conflicts:"))
			for _, conflict := range conflicts {
				fmt.Printf("  %s\n", serverErrorStyle.Render(conflict))
			}
			fmt.Println()
		}

		// Deployed applications
		appCount := len(serverState.DeployedApps)
		fmt.Printf("%s\n", serverHeaderStyle.Render(fmt.Sprintf("Deployed Applications (%d):", appCount)))
		if appCount == 0 {
			fmt.Printf("  %s\n\n", serverMutedStyle.Render("No applications deployed"))
		} else {
			for i, app := range serverState.DeployedApps {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("  %s\n", serverLabelStyle.Render(fmt.Sprintf("%d. %s", i+1, app.TargetName)))
				if app.AppName != app.TargetName {
					fmt.Printf("     App Name:  %s\n", serverValueStyle.Render(app.AppName))
				}
				if app.Framework != "" {
					fmt.Printf("     Framework: %s\n", serverValueStyle.Render(app.Framework))
				}
				fmt.Printf("     Port:      %s\n", serverValueStyle.Render(fmt.Sprintf("%d", app.Port)))
				if app.Domain != "" {
					fmt.Printf("     Domain:    %s\n", serverValueStyle.Render(app.Domain))
				}
				if !app.LastDeploy.IsZero() {
					timeAgo := formatTimeAgo(time.Since(app.LastDeploy))
					fmt.Printf("     Deployed:  %s (%s)\n",
						serverValueStyle.Render(app.LastDeploy.Format("2006-01-02 15:04")),
						serverMutedStyle.Render(timeAgo))
				}
			}
			fmt.Println()
		}

		// Associated targets from config
		targetsOnServer := cfg.GetTargetsByServerIP(serverIP)
		if len(targetsOnServer) > 0 {
			fmt.Printf("%s\n", serverHeaderStyle.Render(fmt.Sprintf("Associated Targets (%d):", len(targetsOnServer))))
			for targetName, target := range targetsOnServer {
				fmt.Printf("  • %s", serverValueStyle.Render(targetName))
				if target.Framework != "" {
					fmt.Printf(" (%s)", serverMutedStyle.Render(target.Framework))
				}
				if target.ProjectPath != "" {
					fmt.Printf("\n    Path: %s", serverMutedStyle.Render(target.ProjectPath))
				}
				fmt.Println()
			}
		}

		fmt.Printf("\n%s\n", serverMutedStyle.Render("To deploy more apps: lightfold deploy --target <name>"))
		fmt.Printf("%s\n", serverMutedStyle.Render("To view app status: lightfold status --target <name>"))
	},
}

// formatTimeAgo formats a duration into a human-readable "time ago" string
func formatTimeAgo(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverListCmd)
	serverCmd.AddCommand(serverShowCmd)
}
