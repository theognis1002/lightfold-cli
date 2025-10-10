package installers

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"lightfold/pkg/ssh"
)

// mockSSHExecutor implements the SSHExecutor interface for testing
type mockSSHExecutor struct {
	commands []string
	failures map[string]bool
}

func newMockSSHExecutor() *mockSSHExecutor {
	return &mockSSHExecutor{
		commands: []string{},
		failures: make(map[string]bool),
	}
}

func (m *mockSSHExecutor) Connect(retries int, retryDelay time.Duration) error {
	return nil
}

func (m *mockSSHExecutor) Disconnect() error {
	return nil
}

func (m *mockSSHExecutor) Execute(command string) *ssh.CommandResult {
	m.commands = append(m.commands, command)

	// Handle failures
	if m.failures[command] {
		// Special case: if the command includes "|| echo 'not-found'", return not-found
		if strings.Contains(command, "|| echo 'not-found'") {
			return &ssh.CommandResult{
				ExitCode: 0,
				Stdout:   "not-found",
			}
		}
		return &ssh.CommandResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("mock error: %s failed", command),
			Error:    fmt.Errorf("command failed"),
		}
	}

	result := &ssh.CommandResult{
		ExitCode: 0,
	}

	switch {
	case strings.Contains(command, "python3 --version"):
		result.Stdout = "Python 3.10.12"
	case strings.Contains(command, "python --version"):
		result.Stdout = "Python 3.10.12"
	case strings.Contains(command, "grep -oP"):
		// Mock Python version extraction
		result.Stdout = "3.10"
	case strings.Contains(command, "import ensurepip"):
		// Mock python3-venv being available
		result.Stdout = ""
		result.ExitCode = 0
	case strings.Contains(command, "command -v poetry"):
		// Mock poetry command availability check
		result.Stdout = "found"
	case strings.Contains(command, "poetry --version"):
		result.Stdout = "Poetry 1.5.0"
	case strings.Contains(command, "command -v pipenv"):
		// Mock pipenv command availability check
		result.Stdout = "found"
	case strings.Contains(command, "pipenv --version"):
		result.Stdout = "pipenv 2023.6.0"
	case strings.Contains(command, "command -v uv"):
		// Mock uv command availability check
		result.Stdout = "found"
	case strings.Contains(command, "uv --version"):
		result.Stdout = "uv 0.1.0"
	}

	return result
}

func (m *mockSSHExecutor) ExecuteWithStreaming(command string, stdoutWriter, stderrWriter io.Writer) *ssh.CommandResult {
	return m.Execute(command)
}

func (m *mockSSHExecutor) ExecuteSudo(command string) *ssh.CommandResult {
	return m.Execute("sudo -n " + command)
}

func (m *mockSSHExecutor) ExecuteSudoWithStreaming(command string, stdoutWriter, stderrWriter io.Writer) *ssh.CommandResult {
	return m.Execute("sudo -n " + command)
}

func (m *mockSSHExecutor) UploadFile(localPath, remotePath string) error {
	return nil
}

func (m *mockSSHExecutor) UploadBytes(content []byte, remotePath string, mode os.FileMode) error {
	return nil
}

func (m *mockSSHExecutor) WriteRemoteFile(remotePath, content string, mode os.FileMode) error {
	return nil
}

func (m *mockSSHExecutor) RenderAndWriteTemplate(template string, data map[string]string, remotePath string, mode os.FileMode) error {
	return nil
}

func (m *mockSSHExecutor) hasCommand(substr string) bool {
	for _, cmd := range m.commands {
		if strings.Contains(cmd, substr) {
			return true
		}
	}
	return false
}

func (m *mockSSHExecutor) commandCount(substr string) int {
	count := 0
	for _, cmd := range m.commands {
		if strings.Contains(cmd, substr) {
			count++
		}
	}
	return count
}
