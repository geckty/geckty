//go:build !windows

package pty

import (
	"bufio"
	"strings"
	"testing"
	"time"
)

func TestOpenEcho(t *testing.T) {
	p, err := Open(Config{Command: []string{"/bin/echo", "hello-geckty"}, Cols: 80, Rows: 24})
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
