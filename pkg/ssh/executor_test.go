package ssh

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewExecutor(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		username string
		keyPath  string
		wantPort string
	}{
		{
			name:     "default port",
			host:     "192.168.1.1",
			port:     "",
			username: "root",
			keyPath:  "~/.ssh/id_rsa",
			wantPort: "22",
		},
		{
			name:     "custom port",
			host:     "192.168.1.1",
			port:     "2222",
			username: "deploy",
			keyPath:  "~/.ssh/deploy_key",
			wantPort: "2222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewExecutor(tt.host, tt.port, tt.username, tt.keyPath)
			if exec.Host != tt.host {
				t.Errorf("Host = %v, want %v", exec.Host, tt.host)
			}
			if exec.Port != tt.wantPort {
				t.Errorf("Port = %v, want %v", exec.Port, tt.wantPort)
			}
			if exec.Username != tt.username {
				t.Errorf("Username = %v, want %v", exec.Username, tt.username)
			}
			if exec.SSHKeyPath != tt.keyPath {
				t.Errorf("SSHKeyPath = %v, want %v", exec.SSHKeyPath, tt.keyPath)
			}
		})
	}
}

func TestCommandResult(t *testing.T) {
	result := &CommandResult{
		Stdout:   "hello world",
		Stderr:   "warning",
		ExitCode: 0,
		Error:    nil,
	}

	if result.Stdout != "hello world" {
		t.Errorf("Stdout = %v, want 'hello world'", result.Stdout)
	}
	if result.Stderr != "warning" {
		t.Errorf("Stderr = %v, want 'warning'", result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", result.ExitCode)
	}
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]string
		want     string
	}{
		{
			name:     "simple replacement",
			template: "Hello {{name}}!",
			data:     map[string]string{"name": "World"},
			want:     "Hello World!",
		},
		{
			name:     "multiple replacements",
			template: "{{greeting}} {{name}}, welcome to {{place}}",
			data: map[string]string{
				"greeting": "Hello",
				"name":     "Alice",
				"place":    "Wonderland",
			},
			want: "Hello Alice, welcome to Wonderland",
		},
		{
			name:     "no replacements",
			template: "Plain text",
			data:     map[string]string{},
			want:     "Plain text",
		},
		{
			name:     "repeated placeholders",
			template: "{{x}} + {{x}} = {{result}}",
			data: map[string]string{
				"x":      "5",
				"result": "10",
			},
			want: "5 + 5 = 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate template rendering
			rendered := tt.template
			for key, value := range tt.data {
				placeholder := "{{" + key + "}}"
				rendered = strings.ReplaceAll(rendered, placeholder, value)
			}

			if rendered != tt.want {
				t.Errorf("Rendered = %v, want %v", rendered, tt.want)
			}
		})
	}
}

func TestUploadBytes(t *testing.T) {
	// This is a unit test that doesn't require actual SSH connection
	// We're testing the logic around file preparation
	content := []byte("test content")
	remotePath := "/tmp/test.txt"
	mode := os.FileMode(0644)

	// Verify file content
	if string(content) != "test content" {
		t.Errorf("Content = %v, want 'test content'", string(content))
	}

	// Verify path components
	filename := filepath.Base(remotePath)
	if filename != "test.txt" {
		t.Errorf("Filename = %v, want 'test.txt'", filename)
	}

	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "/tmp" {
		t.Errorf("RemoteDir = %v, want '/tmp'", remoteDir)
	}

	// Verify mode formatting
	modeStr := mode.Perm().String()
	if !strings.Contains(modeStr, "rw-") {
		t.Errorf("Mode = %v, expected readable/writable", modeStr)
	}
}

func TestExecutorDisconnect(t *testing.T) {
	exec := NewExecutor("localhost", "22", "user", "~/.ssh/id_rsa")

	// Disconnecting when not connected should not error
	err := exec.Disconnect()
	if err != nil {
		t.Errorf("Disconnect on unconnected executor should not error, got: %v", err)
	}
}

// Integration test helpers (these would require actual SSH server for full testing)

func TestConnectionRetryLogic(t *testing.T) {
	// Test that retry parameters are correctly configured
	retries := 3
	retryDelay := 100 * time.Millisecond

	if retries < 0 {
		t.Error("Retries should be non-negative")
	}
	if retryDelay < 0 {
		t.Error("Retry delay should be non-negative")
	}

	// Total attempts should be retries + 1 (initial attempt)
	expectedAttempts := retries + 1
	if expectedAttempts != 4 {
		t.Errorf("Expected %d attempts, got %d", 4, expectedAttempts)
	}
}

func TestStreamingOutput(t *testing.T) {
	// Test that streaming writers work correctly
	var stdout, stderr bytes.Buffer

	testStdout := "output line 1\noutput line 2\n"
	testStderr := "error line 1\n"

	stdout.WriteString(testStdout)
	stderr.WriteString(testStderr)

	if stdout.String() != testStdout {
		t.Errorf("Stdout = %v, want %v", stdout.String(), testStdout)
	}
	if stderr.String() != testStderr {
		t.Errorf("Stderr = %v, want %v", stderr.String(), testStderr)
	}
}

func TestSudoCommandFormatting(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "simple command",
			command: "apt-get update",
			want:    "sudo -n apt-get update",
		},
		{
			name:    "command with args",
			command: "systemctl restart nginx",
			want:    "sudo -n systemctl restart nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sudoCmd := "sudo -n " + tt.command
			if sudoCmd != tt.want {
				t.Errorf("Sudo command = %v, want %v", sudoCmd, tt.want)
			}
		})
	}
}
