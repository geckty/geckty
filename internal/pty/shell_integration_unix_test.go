//go:build !windows

package pty

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readUntilContains reads r until its accumulated output contains want or
// timeout elapses, returning whatever was read either way.
func readUntilContains(t *testing.T, r io.Reader, want string, timeout time.Duration) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		var buf []byte
		tmp := make([]byte, 256)
		for {
			n, err := r.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				if strings.Contains(string(buf), want) {
					done <- string(buf)
					return
				}
			}
			if err != nil {
				done <- string(buf)
				return
			}
		}
	}()
	select {
	case out := <-done:
		return out
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for output containing %q", want)
		return ""
	}
}

func TestApplyShellIntegrationLeavesUnknownShellsAlone(t *testing.T) {
	command, env := applyShellIntegration([]string{"/usr/local/bin/fish"}, []string{"FOO=bar"})
	if len(command) != 1 || command[0] != "/usr/local/bin/fish" {
		t.Fatalf("command = %v, want unchanged", command)
	}
	if len(env) != 1 || env[0] != "FOO=bar" {
		t.Fatalf("env = %v, want unchanged", env)
	}
}

func TestApplyShellIntegrationZshSetsZDOTDIR(t *testing.T) {
	t.Setenv("ZDOTDIR", "/original/zdotdir")
	command, env := applyShellIntegration([]string{"/bin/zsh"}, nil)
	if len(command) != 1 || command[0] != "/bin/zsh" {
		t.Fatalf("command = %v, want unchanged argv (integration is via ZDOTDIR, not extra args)", command)
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "GECKTY_ORIG_ZDOTDIR=/original/zdotdir") {
		t.Fatalf("env missing GECKTY_ORIG_ZDOTDIR, got %v", env)
	}
	if !strings.Contains(joined, "ZDOTDIR=") {
		t.Fatalf("env missing overridden ZDOTDIR, got %v", env)
	}
}

func TestApplyShellIntegrationBashAddsRcfile(t *testing.T) {
	command, _ := applyShellIntegration([]string{"/bin/bash"}, nil)
	if len(command) != 3 || command[0] != "/bin/bash" || command[1] != "--rcfile" {
		t.Fatalf("command = %v, want [/bin/bash --rcfile <path>]", command)
	}
}

func TestShellIntegrationZshSourcesRealRCAndEmitsOSC133(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed")
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("export GECKTY_TEST_MARKER=from-real-zshrc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("ZDOTDIR", "")

	p, err := Open(Config{
		Env:         []string{"HOME=" + home, "PS1=$ ", "TERM=xterm-256color"},
		Cols:        80,
		Rows:        24,
		Integration: true,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Wait for the *expanded value*, not any substring of the typed
	// command itself — the PTY's line-discipline echoes keystrokes back
	// immediately, so waiting on something present in the typed text
	// (e.g. "$GECKTY_TEST_MARKER") would match that echo, not the shell
	// actually having run the command and sourced the real .zshrc.
	if _, err := p.Write([]byte("echo -n $GECKTY_TEST_MARKER\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := readUntilContains(t, p, "from-real-zshrc", 5*time.Second)

	if !strings.Contains(out, "\x1b]133;A") {
		t.Fatalf("expected an OSC 133;A (prompt start) sequence; output: %q", out)
	}
}

func TestShellIntegrationBashSourcesRealRCAndEmitsOSC133(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("export GECKTY_TEST_MARKER=from-real-bashrc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "/bin/bash")

	p, err := Open(Config{
		Env:         []string{"HOME=" + home, "PS1=$ ", "TERM=xterm-256color"},
		Cols:        80,
		Rows:        24,
		Integration: true,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = p.Close() }()

	// See the zsh test's comment on why this waits for the expanded
	// value, not any substring of the typed command itself.
	if _, err := p.Write([]byte("echo -n $GECKTY_TEST_MARKER\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := readUntilContains(t, p, "from-real-bashrc", 5*time.Second)

	if !strings.Contains(out, "\x1b]133;A") {
		t.Fatalf("expected an OSC 133;A (prompt start) sequence; output: %q", out)
	}
}

func TestShellIntegrationDisabledEmitsNoOSC133(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed")
	}
	home := t.TempDir()
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("ZDOTDIR", "")

	p, err := Open(Config{
		Env:         []string{"HOME=" + home, "PS1=$ ", "TERM=xterm-256color"},
		Cols:        80,
		Rows:        24,
		Integration: false,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = p.Close() }()

	if _, err := p.Write([]byte("echo done-marker\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := readUntilContains(t, p, "done-marker", 5*time.Second)

	if strings.Contains(out, "\x1b]133;") {
		t.Fatalf("Integration:false should not inject OSC 133 hooks; output: %q", out)
	}
}
