package utils

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	"lightfold/pkg/state"
	"strconv"
	"strings"
	"time"
)

// GetOrAllocatePort gets the port for a target, allocating one if necessary
func GetOrAllocatePort(target *config.TargetConfig, targetName string) (int, error) {
	// If port is already set in target config, use it
	if target.Port > 0 {
		return target.Port, nil
	}

	// If ServerIP is set, use server state for port allocation
	if target.ServerIP != "" {
		// Check if app is already registered with a port
		app, err := state.GetAppFromServer(target.ServerIP, targetName)
		if err == nil && app.Port > 0 {
			return app.Port, nil
		}

		// Allocate a new port
		port, err := state.AllocatePort(target.ServerIP)
		if err != nil {
			return 0, fmt.Errorf("failed to allocate port: %w", err)
		}
		return port, nil
	}

	// Fallback to default port detection
	return ExtractPortFromTarget(target, target.ProjectPath), nil
}

// RegisterAppWithServer registers an app in the server state
func RegisterAppWithServer(target *config.TargetConfig, targetName string, port int, framework string) error {
	if target.ServerIP == "" {
		return nil // Not using server state
	}

	app := state.DeployedApp{
		TargetName: targetName,
		AppName:    targetName,
		Port:       port,
		Framework:  framework,
		LastDeploy: time.Now(),
	}

	// Add domain if configured
	if target.Domain != nil && target.Domain.Domain != "" {
		app.Domain = target.Domain.Domain
	}

	return state.RegisterApp(target.ServerIP, app)
}

// ExtractPortFromTarget extracts the port from target config or detection
func ExtractPortFromTarget(target *config.TargetConfig, projectPath string) int {
	detection := detector.DetectFramework(projectPath)

	for _, runCmd := range detection.RunPlan {
		if strings.Contains(runCmd, "-port") || strings.Contains(runCmd, "--port") {
			parts := strings.Fields(runCmd)
			for i, part := range parts {
				if (part == "-port" || part == "--port") && i+1 < len(parts) {
					if port, err := strconv.Atoi(parts[i+1]); err == nil {
						return port
					}
				}
			}
		}

		if strings.Contains(runCmd, "--bind") || strings.Contains(runCmd, "--listen") {
			parts := strings.Fields(runCmd)
			for i, part := range parts {
				if (part == "--bind" || part == "--listen") && i+1 < len(parts) {
					bindAddr := parts[i+1]
					if colonIdx := strings.LastIndex(bindAddr, ":"); colonIdx != -1 {
						portStr := bindAddr[colonIdx+1:]
						if port, err := strconv.Atoi(portStr); err == nil {
							return port
						}
					}
				}
			}
		}
	}

	switch detection.Framework {
	case "Next.js", "Nuxt.js", "Remix", "SvelteKit", "Astro":
		return 3000
	case "Django", "Flask", "FastAPI":
		return 5000
	case "Go", "Fiber":
		return 8080
	case "Express.js":
		return 3000
	case "Ruby on Rails":
		return 3000
	default:
		return 3000
	}
}
