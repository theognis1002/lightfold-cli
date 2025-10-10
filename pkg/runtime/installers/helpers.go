package installers

import (
	"fmt"
	"strings"

	sshpkg "lightfold/pkg/ssh"
)

func logOutput(ctx *Context, message string) {
	if ctx != nil && ctx.Output != nil && message != "" {
		ctx.Output(message)
	}
}

func formatCommandError(operation string, result *sshpkg.CommandResult) error {
	var details []string
	if result.ExitCode != 0 {
		details = append(details, fmt.Sprintf("exit_code=%d", result.ExitCode))
	}
	if strings.TrimSpace(result.Stdout) != "" {
		details = append(details, fmt.Sprintf("stdout=%q", strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		details = append(details, fmt.Sprintf("stderr=%q", strings.TrimSpace(result.Stderr)))
	}
	if result.Error != nil {
		details = append(details, fmt.Sprintf("error=%v", result.Error))
	}

	if len(details) > 0 {
		return fmt.Errorf("%s: %s", operation, strings.Join(details, ", "))
	}
	return fmt.Errorf("%s failed", operation)
}

func commandAvailable(ctx *Context, command string) (bool, error) {
	result := ctx.SSH.Execute(fmt.Sprintf("command -v %s >/dev/null 2>&1 && echo 'found' || echo 'not-found'", command))
	if result.Error != nil {
		return false, result.Error
	}
	return strings.TrimSpace(result.Stdout) == "found", nil
}
