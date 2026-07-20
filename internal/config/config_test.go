package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg-config")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join("/xdg-config", "geckty", "config.toml")
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestDefaultPathFallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory available: %v", err)
	}
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(home, ".config", "geckty", "config.toml")
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Window.Width != Default().Window.Width {
		t.Fatalf("expected default width, got %d", cfg.Window.Width)
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fixture := `
[window]
width = 1200
height = 800

[shell]
command = ["/bin/zsh", "-l"]
`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Window.Width != 1200 || cfg.Window.Height != 800 {
		t.Fatalf("window not overridden: %+v", cfg.Window)
	}
	// Untouched section should still carry defaults.
	if cfg.Font.Family != Default().Font.Family {
		t.Fatalf("expected default font family, got %q", cfg.Font.Family)
	}
	got := cfg.ShellCommand()
	if len(got) != 2 || got[0] != "/bin/zsh" || got[1] != "-l" {
		t.Fatalf("shell command = %v", got)
	}
}

func TestEnsureDefaultFileDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geckty", "config.toml")
	if err := EnsureDefaultFile(path); err != nil {
		t.Fatalf("EnsureDefaultFile: %v", err)
	}

	custom := "[window]\nwidth = 42\nheight = 42\n"
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureDefaultFile(path); err != nil {
		t.Fatalf("EnsureDefaultFile (second call): %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Window.Width != 42 {
		t.Fatalf("EnsureDefaultFile overwrote an existing config file: width=%d", cfg.Window.Width)
	}
}

func TestDefaultKeybindingsAvoidShellReadlineBindings(t *testing.T) {
	// Ctrl+T (transpose-char), Ctrl+W (delete-word-backward), Ctrl+C
	// (SIGINT), and Ctrl+V (literal-next) are shell readline/job-control
	// bindings that must reach the shell untouched — the defaults must
	// never bind any of them alone to a geckty action.
	for _, kb := range Default().Keybindings {
		mods := map[string]bool{}
		for _, m := range kb.Mods {
			mods[m] = true
		}
		onlyCtrl := len(kb.Mods) == 1 && mods["ctrl"]
		if onlyCtrl && (kb.Key == "T" || kb.Key == "W" || kb.Key == "C" || kb.Key == "V") {
			t.Fatalf("default keybinding %+v collides with a shell readline/job-control binding", kb)
		}
	}
}

func TestDefaultKeybindingsNonEmpty(t *testing.T) {
	kbs := Default().Keybindings
	if len(kbs) == 0 {
		t.Fatal("expected non-empty default keybindings")
	}
	wantActions := map[string]bool{
		"new_tab": true, "close_tab": true, "next_tab": true, "prev_tab": true,
		"copy": true, "paste": true,
	}
	for _, kb := range kbs {
		delete(wantActions, kb.Action)
	}
	if len(wantActions) != 0 {
		t.Fatalf("default keybindings missing actions: %v", wantActions)
	}
}

func TestLoadOverridesKeybindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fixture := `
[[keybindings]]
key = "N"
mods = ["ctrl", "shift"]
action = "new_tab"
`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Keybindings) != 1 || cfg.Keybindings[0].Key != "N" {
		t.Fatalf("keybindings not overridden: %+v", cfg.Keybindings)
	}
}
