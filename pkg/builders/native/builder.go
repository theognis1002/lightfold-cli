package native

import (
	"context"
	"fmt"
	"lightfold/pkg/builders"
	"lightfold/pkg/config"
	"lightfold/pkg/detector"
	"strings"
)

// NativeBuilder implements the traditional build approach
// Runs build commands directly on the server via SSH
type NativeBuilder struct{}

func init() {
	builders.Register("native", func() builders.Builder {
		return &NativeBuilder{}
	})
}

func (n *NativeBuilder) Name() string {
	return "native"
}

func (n *NativeBuilder) IsAvailable() bool {
	return true
}

func (n *NativeBuilder) NeedsNginx() bool {
	return true
}

func (n *NativeBuilder) Build(ctx context.Context, opts *builders.BuildOptions) (*builders.BuildResult, error) {
	if opts.Detection == nil {
		return &builders.BuildResult{Success: true}, nil
	}

	buildPlan := opts.Detection.BuildPlan
	if len(buildPlan) == 0 {
		return &builders.BuildResult{Success: true}, nil
	}

	ssh := opts.SSHExecutor
	releasePath := opts.ReleasePath
	envVars := opts.EnvVars

	if len(envVars) > 0 {
		var envContent strings.Builder
		for key, value := range envVars {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}

		envPath := fmt.Sprintf("%s/.env", releasePath)
		if err := ssh.WriteRemoteFile(envPath, envContent.String(), config.PermEnvFile); err != nil {
			return nil, fmt.Errorf("failed to write .env for build: %w", err)
		}

		ssh.ExecuteSudo(fmt.Sprintf("chown deploy:deploy %s", envPath))
	}

	if opts.Detection.Language == "Python" {
		appName := getAppName(releasePath)
		venvPath := fmt.Sprintf("%s/%s/shared/venv", config.RemoteAppBaseDir, appName)
		result := ssh.ExecuteSudo(fmt.Sprintf("python3 -m venv %s", venvPath))
		if result.Error != nil || result.ExitCode != 0 {
			return nil, fmt.Errorf("failed to create venv: %s", result.Stderr)
		}
		ssh.ExecuteSudo(fmt.Sprintf("chown -R deploy:deploy %s", venvPath))
	}

	var buildLog strings.Builder
	for _, cmd := range buildPlan {
		trimmed := strings.TrimSpace(cmd)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		buildCmd := adjustBuildCommand(cmd, releasePath, opts.Detection)
		pathPrefix := getPackageManagerPath(opts.Detection)
		fullCmd := fmt.Sprintf("cd %s && %s%s", releasePath, pathPrefix, buildCmd)

		result := ssh.Execute(fullCmd)
		buildLog.WriteString(result.Stdout)
		buildLog.WriteString(result.Stderr)

		if result.Error != nil || result.ExitCode != 0 {
			errorOutput := result.Stderr
			if result.Stdout != "" {
				errorOutput = result.Stdout + "\n" + result.Stderr
			}

			// Check for OOM errors
			if result.ExitCode == 137 || result.ExitCode == 143 || strings.Contains(errorOutput, "Killed") {
				suggestions := "Suggestions:\n  - Increase server memory (upgrade droplet size)\n  - Add swap space to the server"

				if strings.Contains(cmd, "bun") {
					suggestions += "\n  - Use npm instead of bun (bun uses more memory during install)"
				} else if strings.Contains(cmd, "poetry") || strings.Contains(cmd, "pip") {
					suggestions += "\n  - Use --no-cache-dir flag with pip to reduce memory usage"
				}

				return &builders.BuildResult{
					Success:  false,
					BuildLog: buildLog.String(),
				}, fmt.Errorf("build command failed '%s' (exit code %d):\n\n%s\n\nProcess was killed, likely due to insufficient memory (OOM).\n%s", cmd, result.ExitCode, errorOutput, suggestions)
			}

			return &builders.BuildResult{
				Success:  false,
				BuildLog: buildLog.String(),
			}, fmt.Errorf("build command failed '%s' (exit code %d): %s", cmd, result.ExitCode, errorOutput)
		}
	}

	return &builders.BuildResult{
		Success:       true,
		BuildLog:      buildLog.String(),
		IncludesNginx: false,
	}, nil
}

// Helper functions

func getAppName(releasePath string) string {
	// Extract app name from /srv/<app>/releases/<timestamp>
	parts := strings.Split(releasePath, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return "app"
}

func adjustBuildCommand(cmd, releasePath string, detection *detector.Detection) string {
	// Handle Python venv activation
	if detection != nil && detection.Language == "Python" {
		appName := getAppName(releasePath)
		venvPath := fmt.Sprintf("%s/%s/shared/venv", config.RemoteAppBaseDir, appName)
		if strings.Contains(cmd, "pip install") || strings.Contains(cmd, "poetry install") || strings.Contains(cmd, "uv") {
			return fmt.Sprintf("source %s/bin/activate && %s", venvPath, cmd)
		}
	}
	return cmd
}

func getPackageManagerPath(detection *detector.Detection) string {
	if detection == nil {
		return ""
	}

	pm, ok := detection.Meta["package_manager"]
	if !ok {
		return ""
	}

	switch pm {
	case "bun":
		return "export PATH=$HOME/.bun/bin:$PATH && "
	case "poetry":
		return "export PATH=$HOME/.local/bin:$PATH && "
	case "uv":
		return "export PATH=$HOME/.cargo/bin:$PATH && "
	default:
		return ""
	}
}
