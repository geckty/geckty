package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const glassThemeName = "glass"

// builtinThemes maps a theme name to a full color set shipped with geckty.
// User theme files under themes/<name>.toml override these when present.
var builtinThemes = map[string]ColorsConfig{
	glassThemeName: {
		Foreground:          "#f4f4f4",
		Background:          "#1d1f22",
		Selection:           "#525252",
		SelectionBackground: "#525252",
		ANSI: [16]string{
			"#000000", "#c23621", "#25a250", "#caca33",
			"#492ee1", "#d338d3", "#33bbc8", "#cbcccc",
			"#818383", "#fc391f", "#31e722", "#eaec23",
			"#5833ff", "#f935f8", "#14f0f0", "#e9ebeb",
		},
	},
}

// themeFile is the on-disk shape of themes/<name>.toml — a [colors] table
// with the same keys as Config.Colors.
type themeFile struct {
	Colors ColorsConfig `toml:"colors"`
}

// loadThemeColors resolves name to a ColorsConfig: first a themes/<name>.toml
// next to the config file (or under the geckty config dir), then a built-in.
func loadThemeColors(name, configPath string) (ColorsConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ColorsConfig{}, fmt.Errorf("theme: empty name")
	}
	if path, ok := findThemeFile(name, configPath); ok {
		var tf themeFile
		if _, err := toml.DecodeFile(path, &tf); err != nil {
			return ColorsConfig{}, fmt.Errorf("theme %q (%s): %w", name, path, err)
		}
		return tf.Colors, nil
	}
	if c, ok := builtinThemes[name]; ok {
		return c, nil
	}
	return ColorsConfig{}, fmt.Errorf("theme: unknown theme %q (known builtins: %s)", name, knownThemeNames())
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

func knownThemeNames() string {
	names := make([]string, 0, len(builtinThemes))
	for name := range builtinThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// mergeColors overlays non-empty fields from over onto base. Empty strings
// and a zero ANSI array in over leave base's values. Chrome keys follow the
// same rule so a theme can omit them and keep glass-derived defaults at the
// palette layer.
func mergeColors(base, over ColorsConfig) ColorsConfig {
	out := base
	if over.Foreground != "" {
		out.Foreground = over.Foreground
	}
	if over.Background != "" {
		out.Background = over.Background
	}
	if over.Selection != "" {
		out.Selection = over.Selection
	}
	if over.SelectionBackground != "" {
		out.SelectionBackground = over.SelectionBackground
	}
	if over.SelectionForeground != "" {
		out.SelectionForeground = over.SelectionForeground
	}
	if over.Cursor != "" {
		out.Cursor = over.Cursor
	}
	if over.ActiveTabForeground != "" {
		out.ActiveTabForeground = over.ActiveTabForeground
	}
	if over.ActiveTabBackground != "" {
		out.ActiveTabBackground = over.ActiveTabBackground
	}
	if over.InactiveTabForeground != "" {
		out.InactiveTabForeground = over.InactiveTabForeground
	}
	if over.InactiveTabBackground != "" {
		out.InactiveTabBackground = over.InactiveTabBackground
	}
	if over.TabBarBackground != "" {
		out.TabBarBackground = over.TabBarBackground
	}
	if over.HoverTabBackground != "" {
		out.HoverTabBackground = over.HoverTabBackground
	}
	if over.PlusButtonBackground != "" {
		out.PlusButtonBackground = over.PlusButtonBackground
	}
	if ansiNonEmpty(over.ANSI) {
		out.ANSI = over.ANSI
	}
	// Deprecated Preset is never carried forward.
	out.Preset = ""
	return out
}

func ansiNonEmpty(ansi [16]string) bool {
	for _, s := range ansi {
		if s != "" {
			return true
		}
	}
	return false
}

// colorsOverridesFrom returns only the color fields that md marks as
// defined under [colors], taken from decoded (which already holds the
// file's values for those keys).
func colorsOverridesFrom(md toml.MetaData, decoded ColorsConfig) ColorsConfig {
	var over ColorsConfig
	defined := func(key string) bool { return md.IsDefined("colors", key) }
	if defined("foreground") {
		over.Foreground = decoded.Foreground
	}
	if defined("background") {
		over.Background = decoded.Background
	}
	if defined("selection") {
		over.Selection = decoded.Selection
	}
	if defined("selection_background") {
		over.SelectionBackground = decoded.SelectionBackground
	}
	if defined("selection_foreground") {
		over.SelectionForeground = decoded.SelectionForeground
	}
	if defined("cursor") {
		over.Cursor = decoded.Cursor
	}
	if defined("active_tab_foreground") {
		over.ActiveTabForeground = decoded.ActiveTabForeground
	}
	if defined("active_tab_background") {
		over.ActiveTabBackground = decoded.ActiveTabBackground
	}
	if defined("inactive_tab_foreground") {
		over.InactiveTabForeground = decoded.InactiveTabForeground
	}
	if defined("inactive_tab_background") {
		over.InactiveTabBackground = decoded.InactiveTabBackground
	}
	if defined("tab_bar_background") {
		over.TabBarBackground = decoded.TabBarBackground
	}
	if defined("hover_tab_background") {
		over.HoverTabBackground = decoded.HoverTabBackground
	}
	if defined("plus_button_background") {
		over.PlusButtonBackground = decoded.PlusButtonBackground
	}
	if defined("ansi") {
		over.ANSI = decoded.ANSI
	}
	return over
}
