package installers

import (
	"fmt"
	"io"
	"lightfold/pkg/detector"
	"lightfold/pkg/runtime"
	"os"
	"time"

	sshpkg "lightfold/pkg/ssh"
)

// SSHExecutor is an interface for SSH operations used by installers
// This allows for easy mocking in tests
type SSHExecutor interface {
	Connect(retries int, retryDelay time.Duration) error
	Disconnect() error
	Execute(command string) *sshpkg.CommandResult
	ExecuteWithStreaming(command string, stdoutWriter, stderrWriter io.Writer) *sshpkg.CommandResult
	ExecuteSudo(command string) *sshpkg.CommandResult
	ExecuteSudoWithStreaming(command string, stdoutWriter, stderrWriter io.Writer) *sshpkg.CommandResult
	UploadFile(localPath, remotePath string) error
	UploadBytes(content []byte, remotePath string, mode os.FileMode) error
	WriteRemoteFile(remotePath, content string, mode os.FileMode) error
	RenderAndWriteTemplate(template string, data map[string]string, remotePath string, mode os.FileMode) error
}

// Context carries shared information for runtime installers.
type Context struct {
	SSH       SSHExecutor
	Detection *detector.Detection
	Output    func(string)
	Tail      func(result *sshpkg.CommandResult, lastNLines int)
}

// Installer provides hooks for ensuring a runtime is installed on a server.
type Installer interface {
	Runtime() runtime.Runtime
	IsInstalled(ctx *Context) (bool, error)
	Install(ctx *Context) error
}

var registry = map[runtime.Runtime]Installer{}

// Register adds a runtime installer to the registry.
func Register(installer Installer) {
	registry[installer.Runtime()] = installer
}

func getInstaller(rt runtime.Runtime) (Installer, error) {
	installer, ok := registry[rt]
	if !ok {
		return nil, fmt.Errorf("no installer registered for runtime %s", rt)
	}
	return installer, nil
}

// EnsureRuntimeInstalled verifies the runtime is present and installs it if needed.
func EnsureRuntimeInstalled(ctx *Context) error {
	if ctx == nil || ctx.Detection == nil {
		return nil
	}

	rt := runtime.GetRuntimeFromLanguage(ctx.Detection.Language)
	if rt == runtime.RuntimeUnknown {
		return nil
	}

	installer, err := getInstaller(rt)
	if err != nil {
		return err
	}

	installed, err := installer.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}

	return installer.Install(ctx)
}

// RuntimeNeedsInstall reports whether the runtime for the detection is missing.
func RuntimeNeedsInstall(ctx *Context) (bool, error) {
	if ctx == nil || ctx.Detection == nil {
		return false, nil
	}

	rt := runtime.GetRuntimeFromLanguage(ctx.Detection.Language)
	if rt == runtime.RuntimeUnknown {
		return false, nil
	}

	installer, err := getInstaller(rt)
	if err != nil {
		return false, err
	}

	installed, err := installer.IsInstalled(ctx)
	if err != nil {
		return false, err
	}
	return !installed, nil
}
