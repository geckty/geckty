package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/geckty/geckty/assets"
)

const (
	glassThemeName   = ThemeGlass
	defaultThemeName = glassThemeName
)

// ThemeFile is the on-disk shape of themes/<name>.toml.
type ThemeFile struct {
	Colors ColorsConfig `toml:"colors"`
	UI     UIConfig     `toml:"ui"`
}

// UIConfig is the [ui] section of a theme or config.toml — UX chrome tokens
// that are not part of the VT ANSI palette (bell, scrollbar, indicators).
type UIConfig struct {
	VisualBell           string      `toml:"visual_bell"`
	ScrollbarTrack       string      `toml:"scrollbar_track"`
	ScrollbarThumb       string      `toml:"scrollbar_thumb"`
	URLUnderline         string      `toml:"url_underline"`
	SearchMatch          string      `toml:"search_match"`
	HintLabelBG          string      `toml:"hint_label_bg"`
	HintLabelFG          string      `toml:"hint_label_fg"`
	PaneFocusBorder      string      `toml:"pane_focus_border"`
	CommandRunning       string      `toml:"command_running"`
	CommandSuccess       string      `toml:"command_success"`
	CommandFailed        string      `toml:"command_failed"`
	CommandBorderEnabled *bool       `toml:"command_border_enabled"`
	CommandDotEnabled    *bool       `toml:"command_dot_enabled"`
	ContentBrackets      string      `toml:"content_brackets"` // color; empty = off
	Glass                GlassConfig `toml:"glass"`
}

// GlassConfig is [ui.glass] — blend factors for derived tab chrome fills.
type GlassConfig struct {
	BarLift   *float64 `toml:"bar_lift"`
	Inactive  *float64 `toml:"inactive"`
	Hover     *float64 `toml:"hover"`
	Active    *float64 `toml:"active"`
	Drag      *float64 `toml:"drag"`
	PlusHover *float64 `toml:"plus_hover"`
	Rim       *float64 `toml:"rim"`        // edge highlight strength (0–1 toward white)
	RimAlpha  *float64 `toml:"rim_alpha"`  // outline opacity 0–1
	FillAlpha *float64 `toml:"fill_alpha"` // frosted pill opacity 0–1
}

// defaultUI returns the glass theme's [ui] defaults from the embedded
// assets/themes/glass.toml. Pointer bools/floats in the file are set so
// merge can distinguish "unset" from "explicitly false/zero".
func defaultUI() UIConfig {
	tf, ok := loadEmbeddedTheme(defaultThemeName)
	if !ok {
		return defaultUIFallback()
	}
	return tf.UI
}

// defaultUIFallback is used only when the embedded glass theme cannot be
// read. Values come from Glass* (also seeded from embed in init).
func defaultUIFallback() UIConfig {
	cmdBorder, cmdDot := false, false
	barLift, inactive, hover, active, drag := GlassBarLift, GlassInactive, GlassHover, GlassActive, GlassDrag
	plusHover, rim, rimAlpha, fillAlpha := GlassPlusHover, GlassRim, GlassRimAlpha, GlassFillAlpha
	return UIConfig{
		VisualBell:           "#ffffff55",
		ScrollbarTrack:       "#ffffff28",
		ScrollbarThumb:       "#ffffff70",
		ContentBrackets:      "#ffffff55",
		CommandBorderEnabled: &cmdBorder,
		CommandDotEnabled:    &cmdDot,
		Glass: GlassConfig{
			BarLift:   &barLift,
			Inactive:  &inactive,
			Hover:     &hover,
			Active:    &active,
			Drag:      &drag,
			PlusHover: &plusHover,
			Rim:       &rim,
			RimAlpha:  &rimAlpha,
			FillAlpha: &fillAlpha,
		},
	}
}

// loadThemeFile resolves name to a ThemeFile: user themes/<name>.toml first,
// then an embedded assets/themes/<name>.toml.
func loadThemeFile(name, configPath string) (ThemeFile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ThemeFile{}, fmt.Errorf("theme: empty name")
	}
	if path, ok := findThemeFile(name, configPath); ok {
		var tf ThemeFile
		if _, err := toml.DecodeFile(path, &tf); err != nil {
			return ThemeFile{}, fmt.Errorf("theme %q (%s): %w", name, path, err)
		}
		return tf, nil
	}
	if tf, ok := loadEmbeddedTheme(name); ok {
		return tf, nil
	}
	return ThemeFile{}, fmt.Errorf("theme: unknown theme %q (known: %s)", name, strings.Join(ListThemes(configPath), ", "))
}

// loadThemeColors is kept for callers that only need [colors]; prefer loadThemeFile.
func loadThemeColors(name, configPath string) (ColorsConfig, error) {
	tf, err := loadThemeFile(name, configPath)
	if err != nil {
		return ColorsConfig{}, err
	}
	return tf.Colors, nil
}

func loadEmbeddedTheme(name string) (ThemeFile, bool) {
	data, err := fs.ReadFile(assets.Themes, "themes/"+name+".toml")
	if err != nil {
		return ThemeFile{}, false
	}
	var tf ThemeFile
	if _, err := toml.Decode(string(data), &tf); err != nil {
		return ThemeFile{}, false
	}
	return tf, true
}

func findThemeFile(name, configPath string) (string, bool) {
	candidates := make([]string, 0, 2)
	if configPath != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(configPath), "themes", name+".toml"))
	}
	if dir, err := configDir(); err == nil {
		p := filepath.Join(dir, "themes", name+".toml")
		if len(candidates) == 0 || p != candidates[0] {
			candidates = append(candidates, p)
		}
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

func configDir() (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// ListThemes returns sorted theme names from the user themes directory
// (next to configPath when set) plus embedded themes. User files shadow
// embedded names of the same stem.
func ListThemes(configPath string) []string {
	seen := map[string]struct{}{}
	var names []string
	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	dirs := make([]string, 0, 2)
	if configPath != "" {
		dirs = append(dirs, filepath.Join(filepath.Dir(configPath), "themes"))
	}
	if dir, err := configDir(); err == nil {
		p := filepath.Join(dir, "themes")
		if len(dirs) == 0 || p != dirs[0] {
			dirs = append(dirs, p)
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".toml") {
				add(strings.TrimSuffix(name, ".toml"))
			}
		}
	}
	_ = fs.WalkDir(assets.Themes, "themes", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".toml") {
			add(strings.TrimSuffix(base, ".toml"))
		}
		return nil
	})
	sort.Strings(names)
	return names
}

func knownThemeNames() string {
	return strings.Join(ListThemes(""), ", ")
}

func ansiNonEmpty(ansi [16]string) bool {
	for _, s := range ansi {
		if s != "" {
			return true
		}
	}
	return false
}
