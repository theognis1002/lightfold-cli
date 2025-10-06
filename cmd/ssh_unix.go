//go:build !windows

package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// setupWindowChangeHandler sets up terminal window resize handling for Unix systems
func setupWindowChangeHandler(session *ssh.Session, fd int) {
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
}
