//go:build !windows

package pty

import (
	"bufio"
	"strings"
	"testing"
	"time"
)

func TestOpenEcho(t *testing.T) {
	// The child briefly outlives its own echo (rather than exiting
	// instantly) so the reader goroutine below has time to attach before
	// the slave side closes. Without this, a process that exits the
	// moment it's done writing can race the PTY teardown: on some BSD-
	// derived kernels (observed on macOS CI runners under load), the
	// master's read can return EOF/EIO once every slave fd has closed,
	// even if the written bytes were never actually drained by a reader —
	// unlike a plain pipe, a closed PTY isn't guaranteed to still yield
	// its buffered data to a reader that attaches late.
	p, err := Open(Config{Command: []string{"/bin/sh", "-c", "echo hello-geckty; sleep 0.2"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = p.Close() }()

	done := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(p)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				done <- line
				return
			}
		}
		done <- ""
	}()

	select {
	case line := <-done:
		if line != "hello-geckty" {
			t.Fatalf("got %q, want %q", line, "hello-geckty")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for echo output")
	}
}

func TestResolveShellFallsBackWithoutEnv(t *testing.T) {
	t.Setenv("SHELL", "")
	if got := resolveShell(); got == "" {
		t.Fatal("resolveShell returned empty string")
	}
}
