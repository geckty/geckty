//go:build !windows

package pty

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// ptyOpenRetries and ptyOpenRetryDelay bound the retry below: on macOS,
// opening /dev/ptmx can transiently fail with ENODEV ("device not
// configured") when many pseudo-terminals are allocated concurrently —
// observed as flaky CI failures under `go test -parallel` across packages
// that each spawn real shells (internal/pty, internal/session,
// internal/plugin, internal/ui/gogpu). The device frees up within
// milliseconds, so a few short retries clear it without masking a genuine,
// persistent failure.
const (
	ptyOpenRetries    = 5
	ptyOpenRetryDelay = 20 * time.Millisecond
)

type posixPTY struct {
	master *os.File
	cmd    *exec.Cmd
}

// Open spawns Config.Command (or the platform default shell) attached to a
// new POSIX pseudo-terminal via creack/pty.
//
// creack/pty's StartWithSize sets SysProcAttr.Setsid, making the child its
// own session/process-group leader — this is what lets Close kill the whole
// process group (shell + any backgrounded descendants) instead of leaking
// orphans, which is a gap in some PTY wrappers that only kill the direct
// child.
func Open(cfg Config) (PTY, error) {
	command := cfg.Command
	env := cfg.Env
	if len(command) == 0 {
		command = []string{resolveShell()}
		if cfg.Integration {
			command, env = applyShellIntegration(command, env)
		}
	}

	// command is caller-supplied config (the shell to launch), the
	// intended purpose of Open — not untrusted external input.
	cmd := exec.Command(command[0], command[1:]...) //nolint:gosec
	cmd.Dir = cfg.Dir
	cmd.Env = append(os.Environ(), env...)

	cols, rows := cfg.Cols, cfg.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	var master *os.File
	var err error
	for attempt := 0; ; attempt++ {
		master, err = pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
		if err == nil || !errors.Is(err, syscall.ENODEV) || attempt >= ptyOpenRetries {
			break
		}
		time.Sleep(ptyOpenRetryDelay)
	}
	if err != nil {
		return nil, err
	}

	return &posixPTY{master: master, cmd: cmd}, nil
}

func (p *posixPTY) Read(b []byte) (int, error)  { return p.master.Read(b) }
func (p *posixPTY) Write(b []byte) (int, error) { return p.master.Write(b) }

func (p *posixPTY) Resize(cols, rows uint16) error {
	return pty.Setsize(p.master, &pty.Winsize{Rows: rows, Cols: cols})
}

func (p *posixPTY) Pid() int { return p.cmd.Process.Pid }

// Wait blocks until the child exits.
//
// On macOS, cmd.Wait() can block indefinitely if the child is stuck writing
// to a full PTY buffer that nothing is reading. This is not fixed inside
// Wait itself — Wait must not add a second reader on p.master, since
// session.Session already runs a continuous Read loop that feeds the VT
// parser, and two concurrent readers would race for the same output bytes.
// The caller is responsible for keeping that read loop running until Wait
// returns (session.Session.Run does this).
func (p *posixPTY) Wait() error {
	return p.cmd.Wait()
}

func (p *posixPTY) Close() error {
	if p.cmd.Process != nil {
		// Negative pid targets the whole process group (StartWithSize
		// set Setsid, so the shell is the group leader) — this reaps
		// backgrounded/descendant processes the shell spawned, not
		// just the shell itself.
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGHUP)
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	}
	return p.master.Close()
}
