package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePresetEmptyIsNoOp(t *testing.T) {
	c := ColorsConfig{Foreground: "#111111", Background: "#222222"}
	got, err := resolvePreset(c)
	if err != nil {
		t.Fatalf("resolvePreset: %v", err)
	}
	if got != c {
		t.Fatalf("resolvePreset with no Preset set changed the config: got %+v, want %+v", got, c)
	}
}

func TestResolvePresetApplies(t *testing.T) {
	c := ColorsConfig{Preset: "glass", Foreground: "#111111"} // Foreground here is ignored, not merged
	got, err := resolvePreset(c)
	if err != nil {
		t.Fatalf("resolvePreset: %v", err)
	}
	want := presets["glass"]
	want.Preset = "glass"
	if got != want {
		t.Fatalf("resolvePreset(%+v) = %+v, want the full \"glass\" preset %+v", c, got, want)
	}
	if got.Foreground == "#111111" {
		t.Fatal("expected the preset's own Foreground to replace the explicit one, not merge with it")
	}
}

func TestResolvePresetUnknownNameErrors(t *testing.T) {
	_, err := resolvePreset(ColorsConfig{Preset: "not-a-real-preset"})
	if err == nil {
		t.Fatal("expected an error for an unknown preset name")
	}
}

// TestPresetColorsAreValidHex checks every preset's colors are well-formed
// "#rrggbb" strings without depending on internal/ui/theme (which itself
// imports this package for config.ColorsConfig — importing it back from a
// same-package test file here would be a real import cycle, not just a
// theoretical one). theme.NewPalette does the actual parsing at runtime;
// this just guards against a typo'd hex literal in presets.go going
// unnoticed until someone opts into that preset.
func TestPresetColorsAreValidHex(t *testing.T) {
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

	for name, c := range presets {
		if !isHex(c.Foreground) {
			t.Errorf("preset %q has an invalid Foreground %q", name, c.Foreground)
		}
		if !isHex(c.Background) {
			t.Errorf("preset %q has an invalid Background %q", name, c.Background)
		}
		for i, ansi := range c.ANSI {
			if !isHex(ansi) {
				t.Errorf("preset %q has an invalid ANSI[%d] %q", name, i, ansi)
			}
		}
	}
}

func TestLoadResolvesPreset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fixture := "[colors]\npreset = \"glass\"\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := presets["glass"]
	if cfg.Colors.Background != want.Background || cfg.Colors.Foreground != want.Foreground {
		t.Fatalf("Load did not apply the \"glass\" preset: %+v", cfg.Colors)
	}
}

// TestLoadExplicitColorsWithoutPresetKeepsThem proves a config that sets
// foreground/background/ansi but omits colors.preset is not overwritten
// by Default()'s tagged "glass" preset — that was a real Load bug.
func TestLoadExplicitColorsWithoutPresetKeepsThem(t *testing.T) {
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
	if cfg.Colors.Preset != "" {
		t.Fatalf("Preset = %q, want empty (file omitted colors.preset)", cfg.Colors.Preset)
	}
	if cfg.Colors.Background != "#2e3440" || cfg.Colors.Foreground != "#d8dee9" {
		t.Fatalf("Load wiped explicit colors: %+v", cfg.Colors)
	}
}

// TestDefaultUsesGlassPreset proves the "glass" preset is geckty's actual
// default color scheme (see defaultColors in defaults.go), not just an
// opt-in choice a config.toml has to ask for.
func TestDefaultUsesGlassPreset(t *testing.T) {
	want := presets["glass"]
	want.Preset = "glass"
	if got := Default().Colors; got != want {
		t.Fatalf("Default().Colors = %+v, want the \"glass\" preset %+v", got, want)
	}
}

func TestLoadUnknownPresetErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fixture := "[colors]\npreset = \"nope\"\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected Load to error on an unknown colors.preset")
	}
}
