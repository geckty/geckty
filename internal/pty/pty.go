// Package pty spawns and manages the shell process behind a terminal
// session, abstracting over POSIX PTYs and Windows ConPTY.
package pty

import "io"

// PTY is a running shell process attached to a pseudo-terminal.
type PTY interface {
	io.Reader
	io.Writer

	// Resize changes the pseudo-terminal's reported window size.
	Resize(cols, rows uint16) error

	// Pid returns the child process's process ID.
	Pid() int

	// Wait blocks until the child process exits.
	Wait() error

	// Close terminates the child process (and, where the platform
	// supports it, its descendants) and releases the pseudo-terminal.
	Close() error
}

// Config describes how to spawn a shell.
type Config struct {
	// Command is the argv to execute. If empty, the platform default
	// shell is resolved (see shell_unix.go / shell_windows.go).
	Command []string

	// Env is appended to the process environment (os.Environ()).
	Env []string

	// Dir is the child process's working directory. Empty means the
	// current process's working directory.
	Dir string

	Cols, Rows uint16
}

// Open spawns Config.Command (or the platform default shell) attached to a
// new pseudo-terminal. Implemented per-platform in pty_posix.go (creack/pty,
// darwin+linux) and conpty_windows.go (windows.CreatePseudoConsole).
