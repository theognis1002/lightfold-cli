package installers

import (
	"fmt"
	"strings"

	"lightfold/pkg/runtime"
)

type pythonInstaller struct{}

func init() {
	Register(&pythonInstaller{})
}

func (p *pythonInstaller) Runtime() runtime.Runtime {
	return runtime.RuntimePython
}

func (p *pythonInstaller) IsInstalled(ctx *Context) (bool, error) {
	result := ctx.SSH.Execute("python3 --version 2>/dev/null || echo 'not-found'")
	if result.Error != nil {
		return false, result.Error
	}

	if strings.TrimSpace(result.Stdout) == "not-found" || strings.TrimSpace(result.Stdout) == "" {
		return false, nil
	}

	// CRITICAL: Check if python3-venv package is actually installed (required for nixpacks)
	// We check for the ensurepip module specifically, as that's what venv needs
	venvCheck := ctx.SSH.Execute("python3 -c 'import ensurepip' 2>/dev/null")
	if venvCheck.ExitCode != 0 {
		return false, nil // ensurepip not available - need to install python3-venv
	}

	if ctx.Detection != nil {
		if pm, ok := ctx.Detection.Meta["package_manager"]; ok && pm != "" && pm != "pip" {
			switch pm {
			case "poetry":
				return commandAvailable(ctx, "poetry")
			case "pipenv":
				return commandAvailable(ctx, "pipenv")
			case "uv":
				return commandAvailable(ctx, "uv")
			}
		}
	}

	return true, nil
}

func (p *pythonInstaller) Install(ctx *Context) error {
	// Get the exact Python version to install the correct venv package
	versionResult := ctx.SSH.Execute("python3 --version 2>&1 | grep -oP '\\d+\\.\\d+' | head -1")
	pythonVersion := strings.TrimSpace(versionResult.Stdout)

	// Build the package list - install version-specific venv package (e.g., python3.10-venv)
	packages := "python3 python3-pip"
	if pythonVersion != "" {
		packages += " python" + pythonVersion + "-venv"
	}
	packages += " python3-venv" // Also install the metapackage as fallback

	// DEBIAN_FRONTEND=noninteractive prevents interactive prompts
	// -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" prevents config file prompts
	result := ctx.SSH.ExecuteSudo(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::=\"--force-confdef\" -o Dpkg::Options::=\"--force-confold\" %s", packages))
	if ctx.Tail != nil {
		ctx.Tail(result, 3)
	}
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to install Python packages", result)
	}

	// Create symlinks for python and pip (required for nixpacks and some tools)
	result = ctx.SSH.ExecuteSudo("ln -sf /usr/bin/python3 /usr/bin/python")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to create python symlink", result)
	}

	result = ctx.SSH.ExecuteSudo("ln -sf /usr/bin/pip3 /usr/bin/pip")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to create pip symlink", result)
	}

	// Verify symlinks work
	result = ctx.SSH.Execute("python --version")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("python symlink verification failed", result)
	}

	if ctx.Detection == nil {
		return nil
	}

	pm, ok := ctx.Detection.Meta["package_manager"]
	if !ok || pm == "" || pm == "pip" {
		return nil
	}

	switch pm {
	case "poetry":
		result = ctx.SSH.Execute("curl -sSL https://install.python-poetry.org | python3 -")
	case "pipenv":
		result = ctx.SSH.Execute("pip3 install --user pipenv")
	case "uv":
		result = ctx.SSH.Execute("curl -LsSf https://astral.sh/uv/install.sh | sh")
	default:
		return nil
	}

	if ctx.Tail != nil {
		ctx.Tail(result, 3)
	}
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to install "+pm, result)
	}

	return nil
}
