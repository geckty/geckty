package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// fixtureWasm holds the compiled testdata/fixture guest, built once by
// TestMain and reused by every test that needs a loaded plugin — building
// a wasip1/wasm binary takes a second or two, not worth repeating per
// test.
var fixtureWasm []byte

func TestMain(m *testing.M) {
	wasm, err := buildFixture()
	if err != nil {
		panic("building internal/plugin's test fixture: " + err.Error())
	}
	fixtureWasm = wasm
	os.Exit(m.Run())
}

// buildFixture compiles testdata/fixture into a WASI-reactor wasm module
// (see host.go's package doc comment for why -buildmode=c-shared matters)
// and returns the resulting bytes.
func buildFixture() ([]byte, error) {
	dir, err := filepath.Abs("testdata/fixture")
	if err != nil {
		return nil, err
	}
	out := filepath.Join(os.TempDir(), "geckty-plugin-fixture.wasm")
	defer func() { _ = os.Remove(out) }()

	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", out, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, errWithOutput(err, output)
	}
	return os.ReadFile(out)
}

func errWithOutput(err error, output []byte) error {
	return &buildError{err: err, output: string(output)}
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.output }
func (e *buildError) Unwrap() error { return e.err }

// newFixtureDir creates a temp plugin directory containing testdata/
// fixture's compiled entry.wasm and a plugin.toml declaring the given
// permissions, ready to pass to Host.Load.
func newFixtureDir(t *testing.T, name string, permissions []string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "entry.wasm"), fixtureWasm, 0o600); err != nil {
		t.Fatalf("writing entry.wasm: %v", err)
	}

	var perms string
	for i, p := range permissions {
		if i > 0 {
			perms += ", "
		}
		perms += `"` + p + `"`
	}
	toml := "name = \"" + name + "\"\nversion = \"0.0.1\"\nentry = \"entry.wasm\"\npermissions = [" + perms + "]\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(toml), 0o600); err != nil {
		t.Fatalf("writing plugin.toml: %v", err)
	}
	return dir
}

func newHost(t *testing.T) *Host {
	t.Helper()
	ctx := context.Background()
	h, err := NewHost(ctx)
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	t.Cleanup(func() { _ = h.Close(ctx) })
	return h
}

func TestHostLoadAndActivate(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := newFixtureDir(t, "full", []string{"log", "statusbar"})

	p, err := h.Load(ctx, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Name != "full" {
		t.Fatalf("Name = %q, want %q", p.Name, "full")
	}
	if err := p.Activate(ctx); err != nil {
		t.Fatalf("Activate: %v", err)
	}
}

func TestHostDrawStatusbarUpdatesStatusText(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := newFixtureDir(t, "full", []string{"log", "statusbar"})

	p, err := h.Load(ctx, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := p.StatusText(); got != "" {
		t.Fatalf("StatusText before any draw = %q, want empty", got)
	}
	if err := p.DrawStatusbar(ctx); err != nil {
		t.Fatalf("DrawStatusbar: %v", err)
	}
	if got, want := p.StatusText(), "fixture-status"; got != want {
		t.Fatalf("StatusText = %q, want %q", got, want)
	}
}

func TestHostPermissionDeniedStatusbarIsNoOp(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	// No "statusbar" permission declared — the guest's draw_statusbar
	// hook still runs (it doesn't know it lacks the permission), but its
	// call into the host's statusbar_draw function must be silently
	// denied rather than updating StatusText.
	dir := newFixtureDir(t, "noperm", []string{"log"})

	p, err := h.Load(ctx, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := p.DrawStatusbar(ctx); err != nil {
		t.Fatalf("DrawStatusbar: %v", err)
	}
	if got := p.StatusText(); got != "" {
		t.Fatalf("StatusText = %q, want empty (statusbar permission was not granted)", got)
	}
}

func TestHostDuplicatePluginNameRejected(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir1 := newFixtureDir(t, "dup", []string{"log", "statusbar"})
	dir2 := newFixtureDir(t, "dup", []string{"log", "statusbar"})

	if _, err := h.Load(ctx, dir1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if _, err := h.Load(ctx, dir2); err == nil {
		t.Fatal("expected an error loading a second plugin with the same name")
	}
}

func TestHostMissingManifestError(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := t.TempDir() // no plugin.toml at all
	if _, err := h.Load(ctx, dir); err == nil {
		t.Fatal("expected an error for a missing plugin.toml")
	}
}

func TestHostMissingEntryFileError(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := t.TempDir()
	toml := "name = \"missing-entry\"\nentry = \"nope.wasm\"\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(toml), 0o600); err != nil {
		t.Fatalf("writing plugin.toml: %v", err)
	}
	if _, err := h.Load(ctx, dir); err == nil {
		t.Fatal("expected an error for a plugin.toml pointing at a nonexistent entry.wasm")
	}
}

func TestManifestRejectsUnknownPermission(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "entry.wasm"), fixtureWasm, 0o600); err != nil {
		t.Fatalf("writing entry.wasm: %v", err)
	}
	toml := "name = \"bad-perm\"\nentry = \"entry.wasm\"\npermissions = [\"read_everything\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(toml), 0o600); err != nil {
		t.Fatalf("writing plugin.toml: %v", err)
	}
	if _, err := h.Load(ctx, dir); err == nil {
		t.Fatal("expected an error for an unknown permission")
	}
}

func TestManifestRequiresName(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := t.TempDir()
	toml := "entry = \"entry.wasm\"\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(toml), 0o600); err != nil {
		t.Fatalf("writing plugin.toml: %v", err)
	}
	if _, err := h.Load(ctx, dir); err == nil {
		t.Fatal("expected an error for a manifest missing \"name\"")
	}
}

func TestManifestRequiresEntry(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := t.TempDir()
	toml := "name = \"no-entry\"\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(toml), 0o600); err != nil {
		t.Fatalf("writing plugin.toml: %v", err)
	}
	if _, err := h.Load(ctx, dir); err == nil {
		t.Fatal("expected an error for a manifest missing \"entry\"")
	}
}

func TestHostPluginsReturnsLoadOrder(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dirA := newFixtureDir(t, "a", []string{"log"})
	dirB := newFixtureDir(t, "b", []string{"log"})

	if _, err := h.Load(ctx, dirA); err != nil {
		t.Fatalf("Load a: %v", err)
	}
	if _, err := h.Load(ctx, dirB); err != nil {
		t.Fatalf("Load b: %v", err)
	}

	plugins := h.Plugins()
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}
	if plugins[0].Name != "a" || plugins[1].Name != "b" {
		t.Fatalf("load order = %q, %q, want a, b", plugins[0].Name, plugins[1].Name)
	}
}

func TestPluginWithoutHookExportIsNotAnError(t *testing.T) {
	// The fixture doesn't export on_output or on_key (M9 doesn't
	// implement those hooks at all — see hooks.go) — calling a hook a
	// plugin doesn't export must be a harmless no-op via callHook, not
	// an error, since that's the general contract every hook (including
	// ones this milestone does call) relies on.
	ctx := context.Background()
	h := newHost(t)
	dir := newFixtureDir(t, "nohooks", []string{"log", "statusbar"})
	p, err := h.Load(ctx, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ok, err := p.callHook(ctx, "on_output")
	if err != nil {
		t.Fatalf("callHook(on_output): %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for a hook the fixture doesn't export")
	}
}
