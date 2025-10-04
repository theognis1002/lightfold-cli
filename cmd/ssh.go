package cmd

import (
	"fmt"
	"io"
	"lightfold/pkg/config"
	"lightfold/pkg/state"
	"lightfold/pkg/util"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var (
	sshTargetFlag    string
	sshCommandFlag   string
)

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH into a deployment target",
	Long: `Open an interactive SSH session or run a command on a deployment target server.

Without --target flag: Attempts to find target by matching current directory
With --target flag: Connects to the specified target
With --command flag: Runs a single command instead of an interactive session

Examples:
  lightfold ssh                                    # SSH to target in current directory
  lightfold ssh --target myapp                     # SSH to specific target
  lightfold ssh --target myapp --command "uptime"  # Run a command`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfigOrExit()

		var targetName string
		var target config.TargetConfig
		var exists bool

		if sshTargetFlag == "" {
			// Auto-detect target from current directory
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Cannot determine current directory: %v\n", err)
				os.Exit(1)
			}

			// Try to infer target name from directory
			inferredTargetName := util.GetTargetName(cwd)
			target, exists = cfg.GetTarget(inferredTargetName)

			if !exists {
				// Try to find by path
				targetName, target, exists = cfg.FindTargetByPath(cwd)
			} else {
				targetName = inferredTargetName
			}

			if !exists {
				fmt.Fprintf(os.Stderr, "Error: No target found for current directory\n")
				fmt.Fprintf(os.Stderr, "\nRun 'lightfold status' to list all configured targets, or specify a target:\n")
				fmt.Fprintf(os.Stderr, "  lightfold ssh --target <name>\n")
				os.Exit(1)
			}
		} else {
			targetName = sshTargetFlag
			target = loadTargetOrExit(cfg, targetName)
		}

		if target.Provider == "s3" {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' uses S3 provider, which does not support SSH\n", targetName)
			fmt.Fprintf(os.Stderr, "\nS3 targets are for static site deployments only.\n")
			os.Exit(1)
		}

		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Cannot get SSH configuration: %v\n", err)
			os.Exit(1)
		}

		ip := providerCfg.GetIP()
		username := providerCfg.GetUsername()
		sshKey := providerCfg.GetSSHKey()

		if ip == "" {
			targetState, err := state.LoadState(targetName)
			if err == nil && !targetState.Created {
				fmt.Fprintf(os.Stderr, "Error: Target '%s' has not been created yet\n", targetName)
				fmt.Fprintf(os.Stderr, "\nCreate the target first:\n")
				fmt.Fprintf(os.Stderr, "  lightfold create --target %s\n", targetName)
			} else {
				fmt.Fprintf(os.Stderr, "Error: Target '%s' does not have an IP address configured\n", targetName)
				fmt.Fprintf(os.Stderr, "\nThe target may not be fully provisioned. Check status:\n")
				fmt.Fprintf(os.Stderr, "  lightfold status --target %s\n", targetName)
			}
			os.Exit(1)
		}

		if username == "" {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' does not have a username configured\n", targetName)
			fmt.Fprintf(os.Stderr, "\nCheck your target configuration:\n")
			fmt.Fprintf(os.Stderr, "  lightfold status --target %s\n", targetName)
			os.Exit(1)
		}

		if sshKey == "" {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' does not have an SSH key configured\n", targetName)
			fmt.Fprintf(os.Stderr, "\nCheck your target configuration:\n")
			fmt.Fprintf(os.Stderr, "  lightfold status --target %s\n", targetName)
			os.Exit(1)
		}

		if sshCommandFlag != "" {
			if err := executeSSHCommand(ip, username, sshKey, sshCommandFlag); err != nil {
				fmt.Fprintf(os.Stderr, "Error: SSH command failed: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := connectInteractiveSSH(ip, username, sshKey); err != nil {
				fmt.Fprintf(os.Stderr, "Error: SSH connection failed: %v\n", err)
				fmt.Fprintf(os.Stderr, "\nTroubleshooting:\n")
				fmt.Fprintf(os.Stderr, "  1. Verify the server is running and reachable\n")
				fmt.Fprintf(os.Stderr, "  2. Check your SSH key has correct permissions (chmod 600 %s)\n", sshKey)
				fmt.Fprintf(os.Stderr, "  3. Verify network connectivity to %s\n", ip)
				os.Exit(1)
			}
		}
	},
}

func executeSSHCommand(host, username, keyPath, command string) error {
	if keyPath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(home, keyPath[2:])
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse SSH key: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%s", host, config.DefaultSSHPort)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	if err := session.Run(command); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			os.Exit(exitErr.ExitStatus())
		}
		return err
	}

	return nil
}

func connectInteractiveSSH(host, username, keyPath string) error {
	if keyPath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(home, keyPath[2:])
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse SSH key: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%s", host, config.DefaultSSHPort)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %w", err)
	}
	defer term.Restore(fd, state)

	width, height, err := term.GetSize(fd)
	if err != nil {
		width = 80
		height = 24
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		for range sigwinch {
			w, h, err := term.GetSize(fd)
			if err == nil {
				session.WindowChange(h, w)
			}
		}
	}()

	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	if err := session.Wait(); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			if exitErr.ExitStatus() != 0 {
				return fmt.Errorf("remote shell exited with status %d", exitErr.ExitStatus())
			}
		} else if err != io.EOF {
			return fmt.Errorf("session error: %w", err)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(sshCmd)

	sshCmd.Flags().StringVar(&sshTargetFlag, "target", "", "Target name (optional - auto-detects from current directory)")
	sshCmd.Flags().StringVar(&sshCommandFlag, "command", "", "Command to execute (if not specified, starts interactive session)")
}
