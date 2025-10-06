package nixpacks

import (
	"context"
	"encoding/json"
	"fmt"
	"lightfold/pkg/builders"
	"lightfold/pkg/config"
	"strings"
)

// NixpacksBuilder uses Nixpacks for framework detection and building
// https://nixpacks.com
//
// Instead of building containers, we use Nixpacks to:
// 1. Generate an optimized build plan
// 2. Execute install/build commands on the server
// 3. Determine the start command
type NixpacksBuilder struct {
	planData *NixpacksPlan
}

// NixpacksPlan represents the nixpacks plan.json structure
type NixpacksPlan struct {
	Phases struct {
		Setup struct {
			NixPkgs []string `json:"nixPkgs"`
		} `json:"setup"`
		Install *struct {
			Commands []string `json:"cmds"`
		} `json:"install,omitempty"`
		Build *struct {
			Commands []string `json:"cmds"`
		} `json:"build,omitempty"`
	} `json:"phases"`
	Start *struct {
		Command string `json:"cmd"`
	} `json:"start"`
}

func init() {
	builders.Register("nixpacks", func() builders.Builder {
		return &NixpacksBuilder{}
	})
}

func (n *NixpacksBuilder) Name() string {
	return "nixpacks"
}

func (n *NixpacksBuilder) IsAvailable() bool {
	// Nixpacks is installed on-demand on the VPS, not locally
	// Always return true - installation happens during Build()
	return true
}

func (n *NixpacksBuilder) NeedsNginx() bool {
	return true
}

func (n *NixpacksBuilder) Build(ctx context.Context, opts *builders.BuildOptions) (*builders.BuildResult, error) {
	ssh := opts.SSHExecutor
	var buildLog strings.Builder

	buildLog.WriteString("==> Checking nixpacks installation...\n")
	checkResult := ssh.Execute("which nixpacks")
	if checkResult.ExitCode != 0 {
		buildLog.WriteString("==> Installing nixpacks via curl...\n")
		installCmd := "curl -sSL https://nixpacks.com/install.sh | bash"
		installResult := ssh.Execute(installCmd)
		buildLog.WriteString(installResult.Stdout)
		buildLog.WriteString(installResult.Stderr)

		if installResult.ExitCode != 0 {
			return &builders.BuildResult{
				Success:  false,
				BuildLog: buildLog.String(),
			}, fmt.Errorf("failed to install nixpacks: %s", installResult.Stderr)
		}

		ssh.Execute("export PATH=$HOME/.nixpacks/bin:$PATH")
	}

	buildLog.WriteString("==> Generating nixpacks build plan...\n")
	planCmd := fmt.Sprintf("cd %s && $HOME/.nixpacks/bin/nixpacks plan . --format json 2>/dev/null || nixpacks plan . --format json", opts.ReleasePath)
	planResult := ssh.Execute(planCmd)
	buildLog.WriteString(planResult.Stdout)

	if planResult.ExitCode != 0 {
		return &builders.BuildResult{
			Success:  false,
			BuildLog: buildLog.String(),
		}, fmt.Errorf("nixpacks plan failed: %s", planResult.Stderr)
	}

	// Parse the plan
	var plan NixpacksPlan
	if err := json.Unmarshal([]byte(planResult.Stdout), &plan); err != nil {
		return &builders.BuildResult{
			Success:  false,
			BuildLog: buildLog.String(),
		}, fmt.Errorf("failed to parse nixpacks plan: %w", err)
	}
	n.planData = &plan

	if len(plan.Phases.Setup.NixPkgs) > 0 {
		buildLog.WriteString("==> Installing Nix packages...\n")
		buildLog.WriteString("    (skipping - using system packages)\n")
	}

	// Pre-create /opt/venv if nixpacks needs it (Python projects)
	if plan.Phases.Install != nil && len(plan.Phases.Install.Commands) > 0 {
		for _, cmd := range plan.Phases.Install.Commands {
			if strings.Contains(cmd, "/opt/venv") {
				buildLog.WriteString("==> Preparing /opt/venv directory...\n")
				ssh.ExecuteSudo("mkdir -p /opt/venv")
				ssh.ExecuteSudo("chown deploy:deploy /opt/venv")
				break
			}
		}
	}

	if plan.Phases.Install != nil && len(plan.Phases.Install.Commands) > 0 {
		buildLog.WriteString("==> Running install commands...\n")
		for _, cmd := range plan.Phases.Install.Commands {
			fullCmd := fmt.Sprintf("cd %s && %s", opts.ReleasePath, cmd)
			result := ssh.Execute(fullCmd)
			buildLog.WriteString(fmt.Sprintf("    $ %s\n", cmd))
			buildLog.WriteString(result.Stdout)

			if result.ExitCode != 0 {
				buildLog.WriteString(result.Stderr)
				return &builders.BuildResult{
					Success:  false,
					BuildLog: buildLog.String(),
				}, fmt.Errorf("install command failed '%s': %s", cmd, result.Stderr)
			}
		}
	}

	if plan.Phases.Build != nil && len(plan.Phases.Build.Commands) > 0 {
		buildLog.WriteString("==> Running build commands...\n")
		for _, cmd := range plan.Phases.Build.Commands {
			fullCmd := fmt.Sprintf("cd %s && %s", opts.ReleasePath, cmd)
			result := ssh.Execute(fullCmd)
			buildLog.WriteString(fmt.Sprintf("    $ %s\n", cmd))
			buildLog.WriteString(result.Stdout)

			if result.ExitCode != 0 {
				buildLog.WriteString(result.Stderr)

				if result.ExitCode == 137 || result.ExitCode == 143 || strings.Contains(result.Stderr, "Killed") {
					return &builders.BuildResult{
						Success:  false,
						BuildLog: buildLog.String(),
					}, fmt.Errorf("build command killed (OOM): increase server memory")
				}

				return &builders.BuildResult{
					Success:  false,
					BuildLog: buildLog.String(),
				}, fmt.Errorf("build command failed '%s': %s", cmd, result.Stderr)
			}
		}
	}

	if len(opts.EnvVars) > 0 {
		var envContent strings.Builder
		for key, value := range opts.EnvVars {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}

		envPath := fmt.Sprintf("%s/.env", opts.ReleasePath)
		if err := ssh.WriteRemoteFile(envPath, envContent.String(), config.PermEnvFile); err != nil {
			return &builders.BuildResult{
				Success:  false,
				BuildLog: buildLog.String(),
			}, fmt.Errorf("failed to write .env: %w", err)
		}
		ssh.ExecuteSudo(fmt.Sprintf("chown deploy:deploy %s", envPath))
	}

	buildLog.WriteString("==> Build completed successfully!\n")

	startCommand := ""
	if n.planData != nil && n.planData.Start != nil {
		startCommand = n.planData.Start.Command

		// Fix Python commands to use /opt/venv binaries if they reference python/pip/uvicorn/gunicorn
		if strings.Contains(startCommand, "uvicorn") || strings.Contains(startCommand, "gunicorn") {
			// Replace with full path to venv binary
			startCommand = strings.Replace(startCommand, "uvicorn ", "/opt/venv/bin/uvicorn ", 1)
			startCommand = strings.Replace(startCommand, "gunicorn ", "/opt/venv/bin/gunicorn ", 1)
			buildLog.WriteString(fmt.Sprintf("==> Using nixpacks start command with venv: %s\n", startCommand))
		} else if strings.HasPrefix(startCommand, "python ") {
			// Nixpacks sometimes returns "python main.py" for FastAPI apps, which is incorrect
			// Check if this is a web framework that needs uvicorn/gunicorn
			checkCmd := fmt.Sprintf("cd %s && /opt/venv/bin/pip list 2>/dev/null | grep -iE 'fastapi|flask|django|uvicorn|gunicorn'", opts.ReleasePath)
			result := ssh.Execute(checkCmd)
			pipList := strings.ToLower(result.Stdout)

			if strings.Contains(pipList, "fastapi") || strings.Contains(pipList, "uvicorn") {
				// Check if main.py has FastAPI app
				checkApp := fmt.Sprintf("cd %s && grep -E 'app\\s*=.*FastAPI|from fastapi import' main.py 2>/dev/null", opts.ReleasePath)
				appCheck := ssh.Execute(checkApp)
				if appCheck.ExitCode == 0 {
					startCommand = "/opt/venv/bin/uvicorn main:app --host 0.0.0.0 --port 8000"
					buildLog.WriteString("==> Detected FastAPI, overriding to use uvicorn\n")
				} else {
					startCommand = strings.Replace(startCommand, "python ", "/opt/venv/bin/python ", 1)
					buildLog.WriteString(fmt.Sprintf("==> Using nixpacks start command with venv: %s\n", startCommand))
				}
			} else if strings.Contains(pipList, "flask") {
				startCommand = "/opt/venv/bin/gunicorn main:app --bind 0.0.0.0:8000 --workers 2"
				buildLog.WriteString("==> Detected Flask, overriding to use gunicorn\n")
			} else if strings.Contains(pipList, "django") {
				startCommand = "/opt/venv/bin/gunicorn wsgi:application --bind 0.0.0.0:8000 --workers 2"
				buildLog.WriteString("==> Detected Django, overriding to use gunicorn\n")
			} else {
				// No web framework detected, use python with venv
				startCommand = strings.Replace(startCommand, "python ", "/opt/venv/bin/python ", 1)
				buildLog.WriteString(fmt.Sprintf("==> Using nixpacks start command with venv: %s\n", startCommand))
			}
		}
	}

	return &builders.BuildResult{
		Success:       true,
		BuildLog:      buildLog.String(),
		IncludesNginx: false,
		StartCommand:  startCommand,
	}, nil
}
