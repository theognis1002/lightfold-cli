package installers

import (
	"strings"

	"lightfold/pkg/runtime"
)

type goInstaller struct{}

func init() {
	Register(&goInstaller{})
}

func (g *goInstaller) Runtime() runtime.Runtime {
	return runtime.RuntimeGo
}

func (g *goInstaller) IsInstalled(ctx *Context) (bool, error) {
	result := ctx.SSH.Execute("go version 2>/dev/null || echo 'not-found'")
	if result.Error != nil {
		return false, result.Error
	}
	return strings.TrimSpace(result.Stdout) != "not-found" && strings.TrimSpace(result.Stdout) != "", nil
}

func (g *goInstaller) Install(ctx *Context) error {
	result := ctx.SSH.ExecuteSudo("DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::=\"--force-confdef\" -o Dpkg::Options::=\"--force-confold\" golang-go")
	if ctx.Tail != nil {
		ctx.Tail(result, 3)
	}
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to install Go", result)
	}
	return nil
}
