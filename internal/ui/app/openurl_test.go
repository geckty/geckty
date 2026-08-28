package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenURLInvokesPlatformHandler(t *testing.T) {
	dir := t.TempDir()
	var name string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		t.Skip("rundll32 path is not overridden via PATH")
	default:
		name = "xdg-open"
	}
	script := filepath.Join(dir, name)
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	openURL("https://example.com")
}
