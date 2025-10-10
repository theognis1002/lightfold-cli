package installers

import (
	"strings"

	"lightfold/pkg/runtime"
)

type phpInstaller struct{}

func init() {
	Register(&phpInstaller{})
}

func (p *phpInstaller) Runtime() runtime.Runtime {
	return runtime.RuntimePHP
}

func (p *phpInstaller) IsInstalled(ctx *Context) (bool, error) {
	result := ctx.SSH.Execute("php --version 2>/dev/null || echo 'not-found'")
	if result.Error != nil {
		return false, result.Error
	}
	return strings.TrimSpace(result.Stdout) != "not-found" && strings.TrimSpace(result.Stdout) != "", nil
}

func (p *phpInstaller) Install(ctx *Context) error {
	result := ctx.SSH.ExecuteSudo("DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::=\"--force-confdef\" -o Dpkg::Options::=\"--force-confold\" php php-fpm php-mysql php-xml php-mbstring")
	if ctx.Tail != nil {
		ctx.Tail(result, 3)
	}
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to install PHP", result)
	}
	return nil
}
