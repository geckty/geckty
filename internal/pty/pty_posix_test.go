//go:build !windows

package pty

import (
	"bufio"
	"io"
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

func TestOpenWriteResizePid(t *testing.T) {
	// cat echoes stdin back to stdout, so a single blocking read confirms
	// Write's bytes actually reached the child — without depending on
	// shell prompts or PTY line-discipline echo semantics.
	p, err := Open(Config{Command: []string{"/bin/sh", "-c", "cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = p.Close() }()

	if pid := p.Pid(); pid <= 0 {
		t.Fatalf("Pid() = %d, want > 0", pid)
	}

	if err := p.Resize(100, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	if _, err := p.Write([]byte("x")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	readDone := make(chan byte, 1)
	go func() {
		buf := make([]byte, 1)
		if _, err := io.ReadFull(p, buf); err == nil {
			readDone <- buf[0]
		}
	}()
	select {
	case b := <-readDone:
		if b != 'x' {
			t.Fatalf("read back %q, want 'x' (cat echoing the write)", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cat to echo the written byte back")
	}
}

func TestWaitReturnsAfterChildExits(t *testing.T) {
	p, err := Open(Config{Command: []string{"/bin/sh", "-c", "exit 0"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Wait must not race a second reader against session.Session's own
	// read loop (see Wait's doc comment) — this goroutine plays that
	// role here for the life of the test.
	go func() { _, _ = io.Copy(io.Discard, p) }()

	waitDone := make(chan error, 1)
	go func() { waitDone <- p.Wait() }()
	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Wait to return after the shell exited")
	}
}

func TestResolveShellFallsBackWithoutEnv(t *testing.T) {
	t.Setenv("SHELL", "")
	if got := resolveShell(); got == "" {
		t.Fatal("resolveShell returned empty string")
	}
}
