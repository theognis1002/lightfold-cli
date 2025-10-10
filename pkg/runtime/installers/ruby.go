package installers

import (
	"strings"

	"lightfold/pkg/runtime"
)

type rubyInstaller struct{}

func init() {
	Register(&rubyInstaller{})
}

func (r *rubyInstaller) Runtime() runtime.Runtime {
	return runtime.RuntimeRuby
}

func (r *rubyInstaller) IsInstalled(ctx *Context) (bool, error) {
	result := ctx.SSH.Execute("ruby --version 2>/dev/null || echo 'not-found'")
	if result.Error != nil {
		return false, result.Error
	}
	return strings.TrimSpace(result.Stdout) != "not-found" && strings.TrimSpace(result.Stdout) != "", nil
}

func (r *rubyInstaller) Install(ctx *Context) error {
	result := ctx.SSH.ExecuteSudo("DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::=\"--force-confdef\" -o Dpkg::Options::=\"--force-confold\" ruby-full")
	if ctx.Tail != nil {
		ctx.Tail(result, 3)
	}
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to install Ruby", result)
	}
	return nil
}
