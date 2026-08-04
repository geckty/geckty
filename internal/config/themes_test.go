package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeColorsOverlaysNonEmpty(t *testing.T) {
	base := ColorsConfig{Foreground: "#111111", Background: "#222222", Cursor: "#333333"}
	over := ColorsConfig{Foreground: "#abcdef", ActiveTabBackground: "#ff0000"}
	got := mergeColors(base, over)
	if got.Foreground != "#abcdef" {
		t.Fatalf("Foreground = %q, want #abcdef", got.Foreground)
	}
	if got.Background != "#222222" {
		t.Fatalf("Background should stay from base, got %q", got.Background)
	}
	if got.Cursor != "#333333" {
		t.Fatalf("Cursor should stay from base, got %q", got.Cursor)
	}
	if got.ActiveTabBackground != "#ff0000" {
		t.Fatalf("ActiveTabBackground = %q, want #ff0000", got.ActiveTabBackground)
	}
}

func TestBuiltinThemeColorsAreValidHex(t *testing.T) {
	const hexDigits = "0123456789abcdefABCDEF"
	isHex := func(s string) bool {
		if len(s) != 7 || s[0] != '#' {
			return false
		}
		for _, c := range s[1:] {
			if !strings.ContainsRune(hexDigits, c) {
				return false
			}
		}
		return true
	}

	for name, c := range builtinThemes {
		if !isHex(c.Foreground) {
			t.Errorf("theme %q has an invalid Foreground %q", name, c.Foreground)
		}
		if !isHex(c.Background) {
			t.Errorf("theme %q has an invalid Background %q", name, c.Background)
		}
		for i, ansi := range c.ANSI {
			if !isHex(ansi) {
				t.Errorf("theme %q has an invalid ANSI[%d] %q", name, i, ansi)
			}
		}
	}
}

func TestLoadThemeBuiltin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"glass\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := builtinThemes["glass"]
	if cfg.Theme != "glass" {
		t.Fatalf("Theme = %q, want glass", cfg.Theme)
	}
	if cfg.Colors.Background != want.Background || cfg.Colors.Foreground != want.Foreground {
		t.Fatalf("Load did not apply glass theme: %+v", cfg.Colors)
	}
}

func TestLoadDeprecatedPresetAlias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[colors]\npreset = \"glass\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "glass" {
		t.Fatalf("Theme = %q, want glass (from deprecated preset)", cfg.Theme)
	}
	if cfg.Colors.Preset != "" {
		t.Fatalf("Preset should be cleared after resolve, got %q", cfg.Colors.Preset)
	}
	want := builtinThemes["glass"]
	if cfg.Colors.Background != want.Background {
		t.Fatalf("Background = %q, want %q", cfg.Colors.Background, want.Background)
	}
}

func TestLoadInlineColorsOverrideTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	fixture := `
theme = "glass"

[colors]
foreground = "#112233"
active_tab_background = "#ff00aa"
`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Colors.Foreground != "#112233" {
		t.Fatalf("Foreground = %q, want #112233", cfg.Colors.Foreground)
	}
	if cfg.Colors.Background != builtinThemes["glass"].Background {
		t.Fatalf("Background should remain glass, got %q", cfg.Colors.Background)
	}
	if cfg.Colors.ActiveTabBackground != "#ff00aa" {
		t.Fatalf("ActiveTabBackground = %q, want #ff00aa", cfg.Colors.ActiveTabBackground)
	}
}

func TestLoadThemeFileMerges(t *testing.T) {
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o750); err != nil {
		t.Fatal(err)
	}
	themeTOML := `
[colors]
foreground = "#aaaaaa"
background = "#111111"
active_tab_background = "#00ff00"
ansi = [
  "#000000", "#010101", "#020202", "#030303",
  "#040404", "#050505", "#060606", "#070707",
  "#080808", "#090909", "#0a0a0a", "#0b0b0b",
  "#0c0c0c", "#0d0d0d", "#0e0e0e", "#0f0f0f",
]
`
	if err := os.WriteFile(filepath.Join(themesDir, "mine.toml"), []byte(themeTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"mine\"\n\n[colors]\ncursor = \"#ffffff\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Colors.Foreground != "#aaaaaa" || cfg.Colors.Background != "#111111" {
		t.Fatalf("theme file not applied: %+v", cfg.Colors)
	}
	if cfg.Colors.ActiveTabBackground != "#00ff00" {
		t.Fatalf("ActiveTabBackground = %q", cfg.Colors.ActiveTabBackground)
	}
	if cfg.Colors.Cursor != "#ffffff" {
		t.Fatalf("inline cursor override = %q, want #ffffff", cfg.Colors.Cursor)
	}
}

func TestLoadExplicitColorsWithoutThemeKeepsThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fixture := `
[colors]
foreground = "#d8dee9"
background = "#2e3440"
ansi = [
  "#3b4252", "#bf616a", "#a3be8c", "#ebcb8b",
  "#81a1c1", "#b48ead", "#88c0d0", "#e5e9f0",
  "#4c566a", "#bf616a", "#a3be8c", "#ebcb8b",
  "#81a1c1", "#b48ead", "#8fbcbb", "#eceff4",
]
`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "" {
		t.Fatalf("Theme = %q, want empty", cfg.Theme)
	}
	if cfg.Colors.Background != "#2e3440" || cfg.Colors.Foreground != "#d8dee9" {
		t.Fatalf("Load wiped explicit colors: %+v", cfg.Colors)
	}
}

func TestDefaultUsesGlassColors(t *testing.T) {
	want := builtinThemes["glass"]
	if got := Default().Colors; got != want {
		t.Fatalf("Default().Colors = %+v, want glass %+v", got, want)
	}
}

func TestLoadUnknownThemeErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"nope\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected Load to error on an unknown theme")
	}
}

func TestLoadUnknownDeprecatedPresetErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[colors]\npreset = \"nope\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected Load to error on an unknown colors.preset")
	}
}
