package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Executor struct {
	Host       string
	Port       string
	Username   string
	SSHKeyPath string
	client     *ssh.Client
}

func NewExecutor(host, port, username, sshKeyPath string) *Executor {
	if port == "" {
		port = "22"
	}
	return &Executor{
		Host:       host,
		Port:       port,
		Username:   username,
		SSHKeyPath: sshKeyPath,
	}
}

func (e *Executor) Connect(retries int, retryDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		keyPath := e.SSHKeyPath
		if strings.HasPrefix(keyPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				lastErr = fmt.Errorf("failed to get home directory: %w", err)
				continue
			}
			keyPath = filepath.Join(home, keyPath[2:])
		}

		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			lastErr = fmt.Errorf("failed to read SSH key: %w", err)
			continue
		}

		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse SSH key: %w", err)
			continue
		}

		config := &ssh.ClientConfig{
			User: e.Username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         30 * time.Second,
		}

		addr := fmt.Sprintf("%s:%s", e.Host, e.Port)
		client, err := ssh.Dial("tcp", addr, config)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to SSH server (attempt %d/%d): %w", attempt+1, retries+1, err)
			continue
		}

		e.client = client
		return nil
	}

	return fmt.Errorf("failed to connect after %d attempts: %w", retries+1, lastErr)
}

func (e *Executor) Disconnect() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

func (e *Executor) Execute(command string) *CommandResult {
	return e.ExecuteWithStreaming(command, nil, nil)
}

func (e *Executor) ExecuteWithStreaming(command string, stdoutWriter, stderrWriter io.Writer) *CommandResult {
	if e.client == nil {
		return &CommandResult{
			Error: fmt.Errorf("not connected to SSH server"),
		}
	}

	session, err := e.client.NewSession()
	if err != nil {
		return &CommandResult{
			Error: fmt.Errorf("failed to create session: %w", err),
		}
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer

	if stdoutWriter != nil {
		session.Stdout = io.MultiWriter(&stdoutBuf, stdoutWriter)
	} else {
		session.Stdout = &stdoutBuf
	}

	if stderrWriter != nil {
		session.Stderr = io.MultiWriter(&stderrBuf, stderrWriter)
	} else {
		session.Stderr = &stderrBuf
	}

	err = session.Run(command)

	result := &CommandResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			result.Error = err
		}
	}

	return result
}

func (e *Executor) ExecuteSudo(command string) *CommandResult {
	sudoCommand := fmt.Sprintf("sudo -n %s", command)
	return e.Execute(sudoCommand)
}

func (e *Executor) ExecuteSudoWithStreaming(command string, stdoutWriter, stderrWriter io.Writer) *CommandResult {
	sudoCommand := fmt.Sprintf("sudo -n %s", command)
	return e.ExecuteWithStreaming(sudoCommand, stdoutWriter, stderrWriter)
}

func (e *Executor) UploadFile(localPath, remotePath string) error {
	if e.client == nil {
		return fmt.Errorf("not connected to SSH server")
	}

	localFile, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	localInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	return e.UploadBytes(localFile, remotePath, localInfo.Mode())
}

func (e *Executor) UploadBytes(content []byte, remotePath string, mode os.FileMode) error {
	if e.client == nil {
		return fmt.Errorf("not connected to SSH server")
	}

	session, err := e.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	filename := filepath.Base(remotePath)
	remoteDir := filepath.Dir(remotePath)

	mkdirResult := e.Execute(fmt.Sprintf("mkdir -p %s", remoteDir))
	if mkdirResult.Error != nil || mkdirResult.ExitCode != 0 {
		return fmt.Errorf("failed to create remote directory: %v", mkdirResult.Error)
	}

	go func() {
		stdin, err := session.StdinPipe()
		if err != nil {
			return
		}
		defer stdin.Close()

		fmt.Fprintf(stdin, "C%04o %d %s\n", mode.Perm(), len(content), filename)
		stdin.Write(content)
		fmt.Fprint(stdin, "\x00")
	}()

	if err := session.Run(fmt.Sprintf("scp -t %s", remotePath)); err != nil {
		return fmt.Errorf("scp failed: %w", err)
	}

	return nil
}

func (e *Executor) WriteRemoteFile(remotePath, content string, mode os.FileMode) error {
	return e.UploadBytes([]byte(content), remotePath, mode)
}

func (e *Executor) RenderAndWriteTemplate(template string, data map[string]string, remotePath string, mode os.FileMode) error {
	rendered := template
	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		rendered = strings.ReplaceAll(rendered, placeholder, value)
	}

	return e.WriteRemoteFile(remotePath, rendered, mode)
}
