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

func TestEmbeddedThemeColorsAreValidHex(t *testing.T) {
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

	for _, name := range ListThemes("") {
		tf, ok := loadEmbeddedTheme(name)
		if !ok {
			t.Fatalf("embedded theme %q missing", name)
		}
		c := tf.Colors
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
	want := defaultColors()
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
	want := defaultColors()
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
	if cfg.Colors.Background != defaultColors().Background {
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
	want := defaultColors()
	if got := Default().Colors; got.Background != want.Background || got.Foreground != want.Foreground {
		t.Fatalf("Default().Colors = %+v, want glass %+v", got, want)
	}
	if Default().UI.VisualBell == "" {
		t.Fatal("Default().UI should include visual_bell")
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

func TestMergeColorsAllChromeFields(t *testing.T) {
	base := defaultColors()
	over := ColorsConfig{
		Selection:             "#010101",
		SelectionBackground:   "#020202",
		SelectionForeground:   "#030303",
		Cursor:                "#040404",
		ActiveTabForeground:   "#050505",
		ActiveTabBackground:   "#060606",
		InactiveTabForeground: "#070707",
		InactiveTabBackground: "#080808",
		TabBarBackground:      "#090909",
		HoverTabBackground:    "#0a0a0a",
		PlusButtonBackground:  "#0b0b0b",
		Preset:                "should-clear",
		ANSI: [16]string{
			"#101010", "#111111", "#121212", "#131313",
			"#141414", "#151515", "#161616", "#171717",
			"#181818", "#191919", "#1a1a1a", "#1b1b1b",
			"#1c1c1c", "#1d1d1d", "#1e1e1e", "#1f1f1f",
		},
	}
	got := mergeColors(base, over)
	if got.Preset != "" {
		t.Fatalf("Preset should be cleared, got %q", got.Preset)
	}
	if got.Selection != "#010101" || got.SelectionBackground != "#020202" || got.SelectionForeground != "#030303" {
		t.Fatalf("selection fields not merged: %+v", got)
	}
	if got.Cursor != "#040404" {
		t.Fatalf("Cursor = %q", got.Cursor)
	}
	if got.ActiveTabForeground != "#050505" || got.ActiveTabBackground != "#060606" {
		t.Fatalf("active tab fields not merged: %+v", got)
	}
	if got.InactiveTabForeground != "#070707" || got.InactiveTabBackground != "#080808" {
		t.Fatalf("inactive tab fields not merged: %+v", got)
	}
	if got.TabBarBackground != "#090909" || got.HoverTabBackground != "#0a0a0a" || got.PlusButtonBackground != "#0b0b0b" {
		t.Fatalf("chrome backgrounds not merged: %+v", got)
	}
	if got.ANSI[0] != "#101010" || got.ANSI[15] != "#1f1f1f" {
		t.Fatalf("ANSI not merged: %v", got.ANSI)
	}
}

func TestLoadAllChromeColorOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fixture := `
[colors]
selection = "#aaaaaa"
selection_background = "#bbbbbb"
selection_foreground = "#cccccc"
cursor = "#dddddd"
active_tab_foreground = "#111111"
active_tab_background = "#222222"
inactive_tab_foreground = "#333333"
inactive_tab_background = "#444444"
tab_bar_background = "#555555"
hover_tab_background = "#666666"
plus_button_background = "#777777"
`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := cfg.Colors
	if c.Selection != "#aaaaaa" || c.SelectionBackground != "#bbbbbb" || c.SelectionForeground != "#cccccc" {
		t.Fatalf("selection overrides: %+v", c)
	}
	if c.Cursor != "#dddddd" {
		t.Fatalf("Cursor = %q", c.Cursor)
	}
	if c.ActiveTabForeground != "#111111" || c.ActiveTabBackground != "#222222" {
		t.Fatalf("active tab: %+v", c)
	}
	if c.InactiveTabForeground != "#333333" || c.InactiveTabBackground != "#444444" {
		t.Fatalf("inactive tab: %+v", c)
	}
	if c.TabBarBackground != "#555555" || c.HoverTabBackground != "#666666" || c.PlusButtonBackground != "#777777" {
		t.Fatalf("chrome bg: %+v", c)
	}
}

func TestLoadThemeEmptyNameErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"   \"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for whitespace-only theme name")
	}
}

func TestLoadThemeFileInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themesDir, "bad.toml"), []byte("[[[not valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"bad\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid theme file")
	}
}

func TestLoadThemeColorsDirect(t *testing.T) {
	if _, err := loadThemeColors("", ""); err == nil {
		t.Fatal("expected empty name error")
	}
	got, err := loadThemeColors("glass", "")
	if err != nil {
		t.Fatalf("builtin glass: %v", err)
	}
	if got.Background != defaultColors().Background {
		t.Fatalf("got %+v", got)
	}
}

func TestKnownThemeNames(t *testing.T) {
	if got := knownThemeNames(); got != "glass" {
		t.Fatalf("knownThemeNames = %q, want glass", got)
	}
}

func TestListThemesIncludesEmbedded(t *testing.T) {
	names := ListThemes("")
	found := false
	for _, n := range names {
		if n == "glass" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListThemes missing glass: %v", names)
	}
}

func TestDefaultUIMatchesEmbeddedGlass(t *testing.T) {
	tf, ok := loadEmbeddedTheme("glass")
	if !ok {
		t.Fatal("embedded glass theme missing")
	}
	got := defaultUI()
	want := tf.UI

	if got.VisualBell != want.VisualBell {
		t.Errorf("VisualBell = %q, want %q", got.VisualBell, want.VisualBell)
	}
	if got.ScrollbarTrack != want.ScrollbarTrack {
		t.Errorf("ScrollbarTrack = %q, want %q", got.ScrollbarTrack, want.ScrollbarTrack)
	}
	if got.ScrollbarThumb != want.ScrollbarThumb {
		t.Errorf("ScrollbarThumb = %q, want %q", got.ScrollbarThumb, want.ScrollbarThumb)
	}
	if got.ContentBrackets != want.ContentBrackets {
		t.Errorf("ContentBrackets = %q, want %q", got.ContentBrackets, want.ContentBrackets)
	}
	if got.CommandBorderEnabled == nil || want.CommandBorderEnabled == nil {
		t.Fatalf("CommandBorderEnabled nil: got %v want %v", got.CommandBorderEnabled, want.CommandBorderEnabled)
	}
	if *got.CommandBorderEnabled != *want.CommandBorderEnabled {
		t.Errorf("CommandBorderEnabled = %v, want %v", *got.CommandBorderEnabled, *want.CommandBorderEnabled)
	}
	if got.CommandDotEnabled == nil || want.CommandDotEnabled == nil {
		t.Fatalf("CommandDotEnabled nil: got %v want %v", got.CommandDotEnabled, want.CommandDotEnabled)
	}
	if *got.CommandDotEnabled != *want.CommandDotEnabled {
		t.Errorf("CommandDotEnabled = %v, want %v", *got.CommandDotEnabled, *want.CommandDotEnabled)
	}

	assertGlassFloatEqual := func(name string, got, want *float64) {
		t.Helper()
		if got == nil || want == nil {
			t.Fatalf("Glass.%s nil: got %v want %v", name, got, want)
		}
		if *got != *want {
			t.Errorf("Glass.%s = %v, want %v", name, *got, *want)
		}
	}
	assertGlassFloatEqual("BarLift", got.Glass.BarLift, want.Glass.BarLift)
	assertGlassFloatEqual("Inactive", got.Glass.Inactive, want.Glass.Inactive)
	assertGlassFloatEqual("Hover", got.Glass.Hover, want.Glass.Hover)
	assertGlassFloatEqual("Active", got.Glass.Active, want.Glass.Active)
	assertGlassFloatEqual("Drag", got.Glass.Drag, want.Glass.Drag)
	assertGlassFloatEqual("PlusHover", got.Glass.PlusHover, want.Glass.PlusHover)
	assertGlassFloatEqual("Rim", got.Glass.Rim, want.Glass.Rim)
	assertGlassFloatEqual("RimAlpha", got.Glass.RimAlpha, want.Glass.RimAlpha)
	assertGlassFloatEqual("FillAlpha", got.Glass.FillAlpha, want.Glass.FillAlpha)
}

func TestGlassVarsMatchEmbedded(t *testing.T) {
	tf, ok := loadEmbeddedTheme(ThemeGlass)
	if !ok {
		t.Fatal("embedded glass theme missing")
	}
	g := tf.UI.Glass
	check := func(name string, got float64, ptr *float64) {
		t.Helper()
		if ptr == nil {
			t.Fatalf("%s: embedded pointer nil", name)
		}
		if got != *ptr {
			t.Errorf("%s = %v, want embedded %v", name, got, *ptr)
		}
	}
	check("BarLift", GlassBarLift, g.BarLift)
	check("Inactive", GlassInactive, g.Inactive)
	check("Hover", GlassHover, g.Hover)
	check("Active", GlassActive, g.Active)
	check("Drag", GlassDrag, g.Drag)
	check("PlusHover", GlassPlusHover, g.PlusHover)
	check("Rim", GlassRim, g.Rim)
	check("RimAlpha", GlassRimAlpha, g.RimAlpha)
	check("FillAlpha", GlassFillAlpha, g.FillAlpha)
}

func TestLoadThemeUISection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"glass\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UI.VisualBell != "#ffffff55" {
		t.Fatalf("UI.VisualBell = %q", cfg.UI.VisualBell)
	}
	if cfg.UI.CommandBorderEnabled == nil || *cfg.UI.CommandBorderEnabled {
		t.Fatal("command_border_enabled should default false")
	}
	if cfg.UI.CommandDotEnabled == nil || *cfg.UI.CommandDotEnabled {
		t.Fatal("command_dot_enabled should default false")
	}
	if cfg.UI.Glass.PlusHover == nil || *cfg.UI.Glass.PlusHover != 0.08 {
		t.Fatalf("Glass.PlusHover = %v", cfg.UI.Glass.PlusHover)
	}
	if cfg.UI.Glass.Rim == nil || *cfg.UI.Glass.Rim != 0.70 {
		t.Fatalf("Glass.Rim = %v", cfg.UI.Glass.Rim)
	}
	if cfg.UI.Glass.FillAlpha == nil || *cfg.UI.Glass.FillAlpha != 0.78 {
		t.Fatalf("Glass.FillAlpha = %v", cfg.UI.Glass.FillAlpha)
	}
}
