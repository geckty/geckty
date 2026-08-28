package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestDefaultFontBoldItalicAreOn(t *testing.T) {
	d := Default()
	if !d.Font.Bold || !d.Font.Italic {
		t.Fatalf("Font.Bold/Italic should default to true, got %+v", d.Font)
	}
	if d.Font.Family != "" {
		t.Fatalf("Font.Family should default to empty (embedded font), got %q", d.Font.Family)
	}
	if d.UIFont.Family != "" {
		t.Fatalf("UIFont.Family should default to empty (embedded font), got %q", d.UIFont.Family)
	}
}

func TestLoadOverridesFontAndUIFont(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fixture := `
[font]
family = "My Mono"
size = 16
bold = false
italic = false

[ui_font]
family = "My Sans"
size = 14
`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Font.Family != "My Mono" || cfg.Font.Size != 16 {
		t.Fatalf("Font not overridden: %+v", cfg.Font)
	}
	if cfg.Font.Bold || cfg.Font.Italic {
		t.Fatalf("Font.Bold/Italic should have been overridden to false: %+v", cfg.Font)
	}
	if cfg.UIFont.Family != "My Sans" || cfg.UIFont.Size != 14 {
		t.Fatalf("UIFont not overridden: %+v", cfg.UIFont)
	}
}

func TestDefaultTabBarShowsFromSecondTab(t *testing.T) {
	d := Default()
	if d.TabBar.Hidden || d.TabBar.PlusButton.Hidden {
		t.Fatal("tab bar and plus button should not be hidden by default")
	}
	if d.TabBar.ShowThreshold != 2 {
		t.Fatalf("TabBar.ShowThreshold = %d, want 2 (single tab shows no strip)", d.TabBar.ShowThreshold)
	}
	if d.TabBar.PlusButton.ShowThreshold != 2 {
		t.Fatalf("TabBar.PlusButton.ShowThreshold = %d, want 2 (single tab shows no plus button)", d.TabBar.PlusButton.ShowThreshold)
	}
}

func TestLoadOverridesTabBar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fixture := `
[tabbar]
hidden = false
show_threshold = 3

[tabbar.plus_button]
hidden = true
show_threshold = 1
`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TabBar.Hidden {
		t.Fatal("tabbar.hidden should be false")
	}
	if cfg.TabBar.ShowThreshold != 3 {
		t.Fatalf("TabBar.ShowThreshold = %d, want 3", cfg.TabBar.ShowThreshold)
	}
	if !cfg.TabBar.PlusButton.Hidden {
		t.Fatal("tabbar.plus_button.hidden should be true")
	}
	if cfg.TabBar.PlusButton.ShowThreshold != 1 {
		t.Fatalf("TabBar.PlusButton.ShowThreshold = %d, want 1", cfg.TabBar.PlusButton.ShowThreshold)
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
		"split_vertical": true, "split_horizontal": true,
		"next_pane": true, "prev_pane": true, "close_pane": true,
		"scroll_to_prev_prompt": true, "scroll_to_next_prompt": true,
		"select_last_command_output": true,
		"open_url_hints":             true, "show_scrollback": true,
	}
	for _, kb := range kbs {
		delete(wantActions, kb.Action)
	}
	if len(wantActions) != 0 {
		t.Fatalf("default keybindings missing actions: %v", wantActions)
	}
}

func TestDefaultLogLevelIsError(t *testing.T) {
	if Default().LogLevel != "error" {
		t.Fatalf("Default().LogLevel = %q, want %q", Default().LogLevel, "error")
	}
}

func TestLoadOverridesLogLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`log_level = "debug"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"Error":   slog.LevelError,
	}
	for in, want := range cases {
		got, err := ParseLogLevel(in)
		if err != nil {
			t.Fatalf("ParseLogLevel(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseLogLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDefaultHotReloadIsTrue(t *testing.T) {
	if !Default().HotReload {
		t.Fatal("Default().HotReload should be true")
	}
}

func TestWatchNoSourcePathIsNoop(t *testing.T) {
	cfg := Default()
	called := make(chan struct{}, 1)
	stop := cfg.Watch(func(*Config, error) { called <- struct{}{} })
	defer stop()

	select {
	case <-called:
		t.Fatal("Watch should be a no-op for a Config without a source path (not obtained from Load)")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWatchDetectsReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[font]\nsize = 10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	changes := make(chan *Config, 1)
	stop := cfg.Watch(func(c *Config, err error) {
		if err != nil {
			t.Errorf("Watch onChange: %v", err)
			return
		}
		changes <- c
	})
	defer stop()

	if err := os.WriteFile(path, []byte("[font]\nsize = 20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Force the mtime forward regardless of the filesystem's timestamp
	// resolution or how fast the write above landed relative to Load's
	// initial os.Stat.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-changes:
		if got.Font.Size != 20 {
			t.Fatalf("reloaded Font.Size = %v, want 20", got.Font.Size)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Watch did not report the config change in time")
	}
}

func TestParseLogLevelRejectsUnknown(t *testing.T) {
	for _, in := range []string{"", "verbose", "trace"} {
		if _, err := ParseLogLevel(in); err == nil {
			t.Fatalf("ParseLogLevel(%q): expected error, got nil", in)
		}
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
