package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// TestExamplePluginStatusbarClock builds the real, shipped
// plugins/examples/statusbar-clock (not a synthetic test fixture) exactly
// as its own doc comment instructs, loads it through the real Host, and
// confirms it produces an HH:MM:SS status string — proving the one
// example plugin this milestone ships actually works end-to-end, not just
// that the host ABI works against a purpose-built fixture.
func TestExamplePluginStatusbarClock(t *testing.T) {
	srcDir, err := filepath.Abs("../../plugins/examples/statusbar-clock")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "main.go")); err != nil {
		t.Skipf("example plugin source not found at %s: %v", srcDir, err)
	}

	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "entry.wasm")
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", wasmPath, ".")
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the example plugin: %v\n%s", err, output)
	}

	manifest, err := os.ReadFile(filepath.Join(srcDir, "plugin.toml"))
	if err != nil {
		t.Fatalf("reading plugin.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), manifest, 0o600); err != nil {
		t.Fatalf("copying plugin.toml: %v", err)
	}

	ctx := context.Background()
	h := newHost(t)
	p, err := h.Load(ctx, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Name != "statusbar-clock" {
		t.Fatalf("Name = %q, want %q", p.Name, "statusbar-clock")
	}

	if err := p.Activate(ctx); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := p.DrawStatusbar(ctx); err != nil {
		t.Fatalf("DrawStatusbar: %v", err)
	}

	text := p.StatusText()
	if !regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`).MatchString(text) {
		t.Fatalf("StatusText = %q, want an HH:MM:SS time string", text)
	}
}
