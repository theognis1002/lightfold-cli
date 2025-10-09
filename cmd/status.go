package cmd

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/config"
	sshpkg "lightfold/pkg/ssh"
	"lightfold/pkg/state"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	statusTargetFlag string
	statusJSONFlag   bool

	statusHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#01FAC6")).Bold(true)
	statusLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	statusValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	statusMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	statusErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// StatusOutput represents the JSON structure for status output
type StatusOutput struct {
	Target          string             `json:"target"`
	ProjectPath     string             `json:"project_path"`
	Framework       string             `json:"framework"`
	Provider        string             `json:"provider"`
	Created         bool               `json:"created"`
	Configured      bool               `json:"configured"`
	CreateFailed    bool               `json:"create_failed,omitempty"`
	CreateError     string             `json:"create_error,omitempty"`
	ConfigureFailed bool               `json:"configure_failed,omitempty"`
	ConfigureError  string             `json:"configure_error,omitempty"`
	PushFailed      bool               `json:"push_failed,omitempty"`
	PushError       string             `json:"push_error,omitempty"`
	LastFailure     string             `json:"last_failure,omitempty"`
	LastCommit      string             `json:"last_commit,omitempty"`
	LastDeploy      string             `json:"last_deploy,omitempty"`
	LastRelease     string             `json:"last_release,omitempty"`
	ServerIP        string             `json:"server_ip,omitempty"`
	ServerID        string             `json:"server_id,omitempty"`
	ServiceStatus   string             `json:"service_status,omitempty"`
	ServiceUptime   string             `json:"service_uptime,omitempty"`
	CurrentRelease  string             `json:"current_release,omitempty"`
	DiskUsage       string             `json:"disk_usage,omitempty"`
	ServerUptime    string             `json:"server_uptime,omitempty"`
	HealthCheck     *HealthCheckStatus `json:"health_check,omitempty"`
}

// HealthCheckStatus represents health check information
type HealthCheckStatus struct {
	Status       string `json:"status"`
	HTTPCode     int    `json:"http_code,omitempty"`
	ResponseTime int64  `json:"response_time_ms,omitempty"`
	Error        string `json:"error,omitempty"`
}

var statusCmd = &cobra.Command{
	Use:   "status [PROJECT_PATH]",
	Short: "Show deployment status for targets",
	Long: `Display status information for deployment targets.

Without arguments: Lists all configured targets with their states
With path/target: Shows detailed status for a specific target

Examples:
  lightfold status                    # List all targets
  lightfold status .                  # Status for current directory
  lightfold status ~/Projects/myapp   # Status for specific project
  lightfold status --target myapp     # Status for named target
  lightfold status --json             # JSON output`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfigOrExit()

		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		// If no flag and no path arg, show all targets
		if statusTargetFlag == "" && pathArg == "" {
			showAllTargets(cfg)
			return
		}

		_, targetName := resolveTarget(cfg, statusTargetFlag, pathArg)
		showTargetDetail(cfg, targetName)
	},
}

func showAllTargets(cfg *config.Config) {
	if len(cfg.Targets) == 0 {
		fmt.Println(statusMutedStyle.Render("No targets configured yet!"))
		fmt.Printf("\n%s\n", statusMutedStyle.Render("Deploy your first project:"))
		fmt.Printf("  %s  %s\n", statusValueStyle.Render("lightfold deploy"), statusMutedStyle.Render("# in project directory"))
		fmt.Printf("  %s\n", statusValueStyle.Render("lightfold deploy --target myapp"))

		return
	}

	fmt.Printf("%s\n", statusHeaderStyle.Render(fmt.Sprintf("Configured Targets (%d):", len(cfg.Targets))))
	fmt.Println(statusMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))

	for targetName, target := range cfg.Targets {
		fmt.Printf("\n%s\n", statusLabelStyle.Render(targetName))

		targetState, err := state.LoadState(targetName)
		if err != nil {
			fmt.Printf("  %s\n", statusErrorStyle.Render(fmt.Sprintf("Error loading state: %v", err)))
			continue
		}

		fmt.Printf("  Provider:    %s\n", statusValueStyle.Render(target.Provider))
		fmt.Printf("  Framework:   %s\n", statusValueStyle.Render(target.Framework))

		if target.Provider == "flyio" {
			if flyConfig, err := target.GetFlyioConfig(); err == nil && flyConfig.AppName != "" {
				fmt.Printf("  App Name:    %s\n", statusValueStyle.Render(flyConfig.AppName))
			}
		}

		providerCfg, _ := target.GetAnyProviderConfig()
		var ip string
		var hasIP bool
		if providerCfg != nil {
			ip = providerCfg.GetIP()
			hasIP = ip != ""
		}
		if ip != "" {
			fmt.Printf("  IP:          %s\n", statusValueStyle.Render(ip))
		}

		if !targetState.LastDeploy.IsZero() {
			fmt.Printf("  Last Deploy: %s\n", statusValueStyle.Render(targetState.LastDeploy.Format("2006-01-02 15:04")))
		}

		fmt.Printf("\n  %s\n", statusLabelStyle.Render("Pipeline:"))

		fmt.Printf("    %s\n", statusSuccessStyle.Render("✓ Detect"))

		if targetState.Created || hasIP {
			fmt.Printf("    %s\n", statusSuccessStyle.Render("✓ Create"))
		} else if targetState.CreateFailed {
			fmt.Printf("    %s\n", statusErrorStyle.Render("✗ Create (failed)"))
		} else {
			fmt.Printf("    %s\n", statusMutedStyle.Render("[ ] Create"))
		}

		if targetState.Configured {
			fmt.Printf("    %s\n", statusSuccessStyle.Render("✓ Configure"))
		} else if targetState.ConfigureFailed {
			fmt.Printf("    %s\n", statusErrorStyle.Render("✗ Configure (failed)"))
		} else {
			fmt.Printf("    %s\n", statusMutedStyle.Render("[ ] Configure"))
		}

		if !targetState.LastDeploy.IsZero() {
			fmt.Printf("    %s\n", statusSuccessStyle.Render("✓ Push"))
		} else if targetState.PushFailed {
			fmt.Printf("    %s\n", statusErrorStyle.Render("✗ Push (failed)"))
		} else {
			fmt.Printf("    %s\n", statusMutedStyle.Render("[ ] Push"))
		}
	}

	fmt.Printf("\n%s\n", statusMutedStyle.Render("For detailed status: lightfold status --target <name>"))
}

func showTargetDetail(cfg *config.Config, targetName string) {
	target, exists := cfg.GetTarget(targetName)
	if !exists {
		fmt.Fprintf(os.Stderr, "%s\n", statusErrorStyle.Render(fmt.Sprintf("Error: Target '%s' not found", targetName)))
		os.Exit(1)
	}

	targetState, err := state.LoadState(targetName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", statusErrorStyle.Render(fmt.Sprintf("Error loading state: %v", err)))
		os.Exit(1)
	}

	// Collect status data
	statusData := collectStatusData(cfg, targetName, target, targetState)

	// Output in JSON format if flag is set
	if statusJSONFlag {
		jsonData, err := json.MarshalIndent(statusData, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
		return
	}

	// Human-readable output
	fmt.Printf("%s %s\n", statusHeaderStyle.Render("Target:"), statusLabelStyle.Render(targetName))
	fmt.Printf("%s\n\n", statusMutedStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))

	fmt.Printf("%s\n", statusHeaderStyle.Render("Configuration:"))
	fmt.Printf("  Project:   %s\n", statusValueStyle.Render(target.ProjectPath))
	fmt.Printf("  Framework: %s\n", statusValueStyle.Render(target.Framework))
	fmt.Printf("  Provider:  %s\n", statusValueStyle.Render(target.Provider))
	fmt.Println()

	fmt.Printf("%s\n", statusHeaderStyle.Render("State:"))
	if targetState.Created {
		fmt.Printf("  Created:    %s\n", statusSuccessStyle.Render("✓ Yes"))
	} else if targetState.CreateFailed {
		fmt.Printf("  Created:    %s\n", statusErrorStyle.Render("✗ Failed"))
		if targetState.CreateError != "" {
			errorMsg := targetState.CreateError
			if len(errorMsg) > 80 {
				errorMsg = errorMsg[:77] + "..."
			}
			fmt.Printf("  Error:      %s\n", statusMutedStyle.Render(errorMsg))
		}
	} else {
		fmt.Printf("  Created:    %s\n", statusMutedStyle.Render("✗ No"))
	}
	if targetState.Configured {
		fmt.Printf("  Configured: %s\n", statusSuccessStyle.Render("✓ Yes"))
	} else if targetState.ConfigureFailed {
		fmt.Printf("  Configured: %s\n", statusErrorStyle.Render("✗ Failed"))
		if targetState.ConfigureError != "" {
			errorMsg := targetState.ConfigureError
			if len(errorMsg) > 80 {
				errorMsg = errorMsg[:77] + "..."
			}
			fmt.Printf("  Error:      %s\n", statusMutedStyle.Render(errorMsg))
		}
	} else {
		fmt.Printf("  Configured: %s\n", statusMutedStyle.Render("✗ No"))
	}
	if targetState.ProvisionedID != "" {
		fmt.Printf("  Server ID:  %s\n", statusValueStyle.Render(targetState.ProvisionedID))
	}
	if targetState.LastCommit != "" {
		commitShort := targetState.LastCommit
		if len(commitShort) > 7 {
			commitShort = commitShort[:7]
		}
		fmt.Printf("  Last Commit: %s\n", statusValueStyle.Render(commitShort))
	}
	if !targetState.LastDeploy.IsZero() {
		fmt.Printf("  Last Deploy: %s\n", statusValueStyle.Render(targetState.LastDeploy.Format("2006-01-02 15:04:05")))
	} else if targetState.PushFailed {
		fmt.Printf("  Last Deploy: %s\n", statusErrorStyle.Render("✗ Failed"))
		if targetState.PushError != "" {
			errorMsg := targetState.PushError
			if len(errorMsg) > 80 {
				errorMsg = errorMsg[:77] + "..."
			}
			fmt.Printf("  Error:      %s\n", statusMutedStyle.Render(errorMsg))
		}
	}
	if targetState.LastRelease != "" {
		fmt.Printf("  Last Release: %s\n", statusValueStyle.Render(targetState.LastRelease))
	}
	if !targetState.LastFailure.IsZero() {
		fmt.Printf("  Last Failure: %s\n", statusErrorStyle.Render(targetState.LastFailure.Format("2006-01-02 15:04:05")))
	}
	fmt.Println()

	if targetState.Created {
		fmt.Printf("%s\n", statusHeaderStyle.Render("Server Status:"))

		if target.Provider == "s3" {
			fmt.Printf("  Type: %s\n", statusValueStyle.Render("S3 (static site)"))
			s3Config, _ := target.GetS3Config()
			fmt.Printf("  Bucket: %s\n", statusValueStyle.Render(s3Config.Bucket))
			fmt.Printf("  Region: %s\n", statusValueStyle.Render(s3Config.Region))
			fmt.Println()
			return
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Printf("  %s\n", statusErrorStyle.Render(fmt.Sprintf("Error: %v", err)))
			return
		}

		fmt.Printf("  IP:        %s\n", statusValueStyle.Render(providerCfg.GetIP()))
		fmt.Printf("  Username:  %s\n", statusValueStyle.Render(providerCfg.GetUsername()))

		if providerCfg.GetIP() != "" {
			sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
			defer sshExecutor.Disconnect()

			appName := strings.ReplaceAll(targetName, "-", "_")
			result := sshExecutor.Execute(fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo 'not-found'", appName))
			if result.ExitCode == 0 {
				status := strings.TrimSpace(result.Stdout)
				if status == "active" {
					fmt.Printf("  Service:   %s\n", statusSuccessStyle.Render("✓ Active"))
				} else if status == "not-found" {
					fmt.Printf("  Service:   %s\n", statusMutedStyle.Render("- Not configured"))
				} else {
					fmt.Printf("  Service:   %s\n", statusErrorStyle.Render(fmt.Sprintf("✗ %s", status)))
				}
			} else {
				fmt.Printf("  Service:   %s\n", statusMutedStyle.Render("? Unable to check"))
			}

			// Get service uptime
			if statusData.ServiceStatus == "active" && statusData.ServiceUptime != "" {
				fmt.Printf("  Uptime:    %s\n", statusValueStyle.Render(statusData.ServiceUptime))
			}

			result = sshExecutor.Execute(fmt.Sprintf("readlink -f %s/%s/current 2>/dev/null || echo 'none'", config.RemoteAppBaseDir, appName))
			if result.ExitCode == 0 {
				currentRelease := strings.TrimSpace(result.Stdout)
				if currentRelease != "none" && currentRelease != "" {
					releaseTimestamp := strings.TrimPrefix(currentRelease, fmt.Sprintf("%s/%s/releases/", config.RemoteAppBaseDir, appName))
					fmt.Printf("  Current:   %s\n", statusValueStyle.Render(releaseTimestamp))
				} else {
					fmt.Printf("  Current:   %s\n", statusMutedStyle.Render("- No release deployed"))
				}
			}

			result = sshExecutor.Execute("df -h / | tail -1 | awk '{print $5}'")
			if result.ExitCode == 0 {
				diskUsage := strings.TrimSpace(result.Stdout)
				fmt.Printf("  Disk:      %s\n", statusValueStyle.Render(diskUsage+" used"))
			}

			result = sshExecutor.Execute("uptime -p 2>/dev/null || uptime | awk '{print $3, $4}'")
			if result.ExitCode == 0 {
				uptime := strings.TrimSpace(result.Stdout)
				fmt.Printf("  Server:    %s\n", statusValueStyle.Render(uptime))
			}

			if statusData.HealthCheck != nil {
				fmt.Printf("\n%s\n", statusHeaderStyle.Render("Health Check:"))
				if statusData.HealthCheck.Status == "healthy" {
					fmt.Printf("  Status:    %s\n", statusSuccessStyle.Render(fmt.Sprintf("✓ Healthy (HTTP %d)", statusData.HealthCheck.HTTPCode)))
					fmt.Printf("  Response:  %s\n", statusValueStyle.Render(fmt.Sprintf("%dms", statusData.HealthCheck.ResponseTime)))
				} else {
					fmt.Printf("  Status:    %s\n", statusErrorStyle.Render("✗ Unhealthy"))
					if statusData.HealthCheck.Error != "" {
						fmt.Printf("  Error:     %s\n", statusMutedStyle.Render(statusData.HealthCheck.Error))
					}
				}
			}
		}
		fmt.Println()
	}

	if targetState.CreateFailed {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Retry:"), statusValueStyle.Render(fmt.Sprintf("lightfold deploy --target %s", targetName)))
	} else if !targetState.Created {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Next:"), statusValueStyle.Render(fmt.Sprintf("lightfold create --target %s", targetName)))
	} else if targetState.ConfigureFailed {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Retry:"), statusValueStyle.Render(fmt.Sprintf("lightfold configure --target %s --force", targetName)))
	} else if !targetState.Configured {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Next:"), statusValueStyle.Render(fmt.Sprintf("lightfold configure --target %s", targetName)))
	} else if targetState.PushFailed {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Retry:"), statusValueStyle.Render(fmt.Sprintf("lightfold push --target %s", targetName)))
	} else {
		fmt.Printf("%s %s\n", statusLabelStyle.Render("Ready to deploy:"), statusValueStyle.Render(fmt.Sprintf("lightfold push --target %s", targetName)))
	}
}

// collectStatusData gathers all status information for a target
func collectStatusData(cfg *config.Config, targetName string, target config.TargetConfig, targetState *state.TargetState) StatusOutput {
	statusData := StatusOutput{
		Target:          targetName,
		ProjectPath:     target.ProjectPath,
		Framework:       target.Framework,
		Provider:        target.Provider,
		Created:         targetState.Created,
		Configured:      targetState.Configured,
		CreateFailed:    targetState.CreateFailed,
		CreateError:     targetState.CreateError,
		ConfigureFailed: targetState.ConfigureFailed,
		ConfigureError:  targetState.ConfigureError,
		PushFailed:      targetState.PushFailed,
		PushError:       targetState.PushError,
		LastCommit:      targetState.LastCommit,
		LastRelease:     targetState.LastRelease,
		ServerID:        targetState.ProvisionedID,
	}

	if !targetState.LastDeploy.IsZero() {
		statusData.LastDeploy = targetState.LastDeploy.Format(time.RFC3339)
	}

	if !targetState.LastFailure.IsZero() {
		statusData.LastFailure = targetState.LastFailure.Format(time.RFC3339)
	}

	if target.Provider == "s3" {
		return statusData
	}

	providerCfg, err := target.GetSSHProviderConfig()
	if err != nil {
		return statusData
	}

	statusData.ServerIP = providerCfg.GetIP()

	if providerCfg.GetIP() == "" {
		return statusData
	}

	sshExecutor := sshpkg.NewExecutor(providerCfg.GetIP(), "22", providerCfg.GetUsername(), providerCfg.GetSSHKey())
	defer sshExecutor.Disconnect()

	appName := strings.ReplaceAll(targetName, "-", "_")

	result := sshExecutor.Execute(fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo 'not-found'", appName))
	if result.ExitCode == 0 {
		statusData.ServiceStatus = strings.TrimSpace(result.Stdout)
	}

	if statusData.ServiceStatus == "active" {
		result = sshExecutor.Execute(fmt.Sprintf("systemctl show -p ActiveEnterTimestamp %s 2>/dev/null | cut -d= -f2", appName))
		if result.ExitCode == 0 && result.Stdout != "" {
			timestampStr := strings.TrimSpace(result.Stdout)
			if timestampStr != "" && timestampStr != "n/a" {
				activeTime, err := time.Parse("Mon 2006-01-02 15:04:05 MST", timestampStr)
				if err == nil {
					uptime := time.Since(activeTime)
					statusData.ServiceUptime = formatUptime(uptime)
				}
			}
		}
	}

	result = sshExecutor.Execute(fmt.Sprintf("readlink -f %s/%s/current 2>/dev/null || echo 'none'", config.RemoteAppBaseDir, appName))
	if result.ExitCode == 0 {
		currentRelease := strings.TrimSpace(result.Stdout)
		if currentRelease != "none" && currentRelease != "" {
			statusData.CurrentRelease = strings.TrimPrefix(currentRelease, fmt.Sprintf("%s/%s/releases/", config.RemoteAppBaseDir, appName))
		}
	}

	result = sshExecutor.Execute("df -h / | tail -1 | awk '{print $5}'")
	if result.ExitCode == 0 {
		statusData.DiskUsage = strings.TrimSpace(result.Stdout)
	}

	result = sshExecutor.Execute("uptime -p 2>/dev/null || uptime | awk '{print $3, $4}'")
	if result.ExitCode == 0 {
		statusData.ServerUptime = strings.TrimSpace(result.Stdout)
	}

	if statusData.ServiceStatus == "active" {
		healthCheck := performHealthCheck(sshExecutor)
		statusData.HealthCheck = &healthCheck
	}

	return statusData
}

// performHealthCheck performs an HTTP health check and returns the status
func performHealthCheck(sshExecutor *sshpkg.Executor) HealthCheckStatus {
	healthCheck := HealthCheckStatus{
		Status: "unhealthy",
	}

	startTime := time.Now()
	curlCmd := "curl -s -o /dev/null -w '%{http_code}' --max-time 5 http://127.0.0.1:8000/"
	result := sshExecutor.Execute(curlCmd)
	responseTime := time.Since(startTime).Milliseconds()

	if result.Error != nil {
		healthCheck.Error = result.Error.Error()
		return healthCheck
	}

	if result.ExitCode != 0 {
		healthCheck.Error = "health check request failed"
		return healthCheck
	}

	httpCodeStr := strings.TrimSpace(result.Stdout)
	httpCode, err := strconv.Atoi(httpCodeStr)
	if err != nil {
		healthCheck.Error = "invalid HTTP response code"
		return healthCheck
	}

	healthCheck.HTTPCode = httpCode
	healthCheck.ResponseTime = responseTime

	if httpCode >= 200 && httpCode < 400 {
		healthCheck.Status = "healthy"
	}

	return healthCheck
}

// formatUptime formats a duration into a human-readable uptime string
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVar(&statusTargetFlag, "target", "", "Target name (optional - shows all targets if omitted)")
	statusCmd.Flags().BoolVar(&statusJSONFlag, "json", false, "Output status in JSON format")
}
