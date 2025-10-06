//go:build windows

package cmd

import (
	"golang.org/x/crypto/ssh"
)

// setupWindowChangeHandler is a no-op on Windows
// Windows doesn't have SIGWINCH, so we don't support dynamic terminal resizing
func setupWindowChangeHandler(session *ssh.Session, fd int) {
	// No-op on Windows - terminal resize signals not supported
}
