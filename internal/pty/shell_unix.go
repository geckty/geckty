//go:build !windows

package pty

import (
	"os"
	"runtime"
)

// resolveShell picks the shell to launch when Config.Command is empty:
// $SHELL if set, otherwise a platform default. No /etc/passwd lookup —
// $SHELL is what every POSIX login sets, and parsing passwd adds a syscall
// dependency for no practical gain in a desktop terminal.
func resolveShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/sh"
}
