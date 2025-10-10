package installers

import (
	"fmt"
	"strings"

	"lightfold/pkg/runtime"
)

const nodeVersionTarget = "v20.11.1"
const nodeArchiveURL = "https://nodejs.org/dist/v20.11.1/node-v20.11.1-linux-x64.tar.xz"

type nodeInstaller struct{}

func init() {
	Register(&nodeInstaller{})
}

func (n *nodeInstaller) Runtime() runtime.Runtime {
	return runtime.RuntimeNodeJS
}

func (n *nodeInstaller) IsInstalled(ctx *Context) (bool, error) {
	version := n.currentNodeVersion(ctx)
	if version == "" {
		return false, nil
	}

	if ctx.Detection != nil {
		if pm, ok := ctx.Detection.Meta["package_manager"]; ok && pm != "" && pm != "npm" {
			installed, err := commandAvailable(ctx, pm)
			if err != nil {
				return false, err
			}
			if !installed {
				return false, nil
			}
		}
	}

	return true, nil
}

func (n *nodeInstaller) Install(ctx *Context) error {
	existingVersion := n.currentNodeVersion(ctx)
	if strings.HasPrefix(existingVersion, "v20.") {
		logOutput(ctx, fmt.Sprintf("  Node.js already installed: %s", existingVersion))
		n.linkNodeBinaries(ctx)
		return n.ensurePackageManagers(ctx)
	}

	logOutput(ctx, fmt.Sprintf("  Installing Node.js %s...", nodeVersionTarget))

	n.removeLegacyNode(ctx)

	if err := n.downloadAndInstallNode(ctx); err != nil {
		return err
	}

	if err := n.ensurePackageManagers(ctx); err != nil {
		return err
	}

	return nil
}

func (n *nodeInstaller) currentNodeVersion(ctx *Context) string {
	result := ctx.SSH.Execute("/usr/local/bin/node --version 2>/dev/null || /usr/bin/node --version 2>/dev/null || echo 'not-found'")
	version := strings.TrimSpace(result.Stdout)
	if version == "not-found" || version == "" {
		return ""
	}
	return version
}

func (n *nodeInstaller) linkNodeBinaries(ctx *Context) {
	ctx.SSH.ExecuteSudo("ln -sf /usr/local/bin/node /usr/bin/node")
	ctx.SSH.ExecuteSudo("ln -sf /usr/local/bin/npm /usr/bin/npm")
	ctx.SSH.ExecuteSudo("ln -sf /usr/local/bin/npx /usr/bin/npx")
}

func (n *nodeInstaller) removeLegacyNode(ctx *Context) {
	ctx.SSH.ExecuteSudo("apt-get remove -y nodejs npm libnode-dev libnode72 2>/dev/null || true")
	ctx.SSH.ExecuteSudo("apt-get purge -y nodejs npm libnode-dev libnode72 2>/dev/null || true")
	ctx.SSH.ExecuteSudo("apt-get autoremove -y 2>/dev/null || true")
	ctx.SSH.ExecuteSudo("rm -f /usr/bin/node /usr/bin/npm /usr/bin/npx 2>/dev/null || true")
	ctx.SSH.ExecuteSudo("rm -rf /usr/lib/node_modules 2>/dev/null || true")
	ctx.SSH.ExecuteSudo("rm -f /etc/apt/sources.list.d/nodesource.list 2>/dev/null || true")
}

func (n *nodeInstaller) downloadAndInstallNode(ctx *Context) error {
	result := ctx.SSH.ExecuteSudo(fmt.Sprintf("curl -fsSL %s -o /tmp/node.tar.xz", nodeArchiveURL))
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to download Node.js", result)
	}

	result = ctx.SSH.ExecuteSudo("tar -xf /tmp/node.tar.xz -C /tmp")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to extract Node.js", result)
	}

	result = ctx.SSH.ExecuteSudo("cp -r /tmp/node-v20.11.1-linux-x64/* /usr/local/")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to install Node.js to /usr/local", result)
	}

	n.linkNodeBinaries(ctx)

	ctx.SSH.ExecuteSudo("rm -rf /tmp/node-v20.11.1-linux-x64 /tmp/node.tar.xz")

	versionResult := ctx.SSH.Execute("/usr/bin/node --version")
	nodeVersion := strings.TrimSpace(versionResult.Stdout)
	if ctx.Output != nil && nodeVersion != "" {
		ctx.Output(fmt.Sprintf("  Node.js installed: %s at /usr/bin/node", nodeVersion))
	}

	if !strings.HasPrefix(nodeVersion, "v1") && !strings.HasPrefix(nodeVersion, "v2") {
		return fmt.Errorf("failed to install modern Node.js, got version: %s", nodeVersion)
	}

	return nil
}

func (n *nodeInstaller) ensurePackageManagers(ctx *Context) error {
	if ctx.Detection == nil {
		return nil
	}

	pm, ok := ctx.Detection.Meta["package_manager"]
	if !ok || pm == "" || pm == "npm" {
		return nil
	}

	switch pm {
	case "bun":
		result := ctx.SSH.Execute("curl -fsSL https://bun.sh/install | bash")
		if ctx.Tail != nil {
			ctx.Tail(result, 3)
		}
		if result.Error != nil || result.ExitCode != 0 {
			return formatCommandError("failed to install bun", result)
		}
	case "pnpm":
		result := ctx.SSH.ExecuteSudo("npm install -g pnpm")
		if ctx.Tail != nil {
			ctx.Tail(result, 3)
		}
		if result.Error != nil || result.ExitCode != 0 {
			return formatCommandError("failed to install pnpm", result)
		}
	case "yarn":
		result := ctx.SSH.ExecuteSudo("npm install -g yarn")
		if ctx.Tail != nil {
			ctx.Tail(result, 3)
		}
		if result.Error != nil || result.ExitCode != 0 {
			return formatCommandError("failed to install yarn", result)
		}
	}

	return nil
}
