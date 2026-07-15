package config

import (
	"fmt"
	"sort"
	"strings"
)

const glassPresetName = "glass"

// presets maps a ColorsConfig.Preset name to a full color set. Selecting
// a preset replaces Foreground/Background/ANSI (no field-level merge).
var presets = map[string]ColorsConfig{
	// Default dark "Pro"-like palette for the glass chrome theme.
	glassPresetName: {
		Foreground: "#f4f4f4",
		Background: "#1d1f22",
		Selection:  "#525252",
		ANSI: [16]string{
			"#000000", "#c23621", "#25a250", "#caca33",
			"#492ee1", "#d338d3", "#33bbc8", "#cbcccc",
			"#818383", "#fc391f", "#31e722", "#eaec23",
			"#5833ff", "#f935f8", "#14f0f0", "#e9ebeb",
		},
	},
}

// resolvePreset applies c.Preset if set. Unknown names are an error.
func resolvePreset(c ColorsConfig) (ColorsConfig, error) {
	if c.Preset == "" {
		return c, nil
	}
	preset, ok := presets[c.Preset]
	if !ok {
		return ColorsConfig{}, fmt.Errorf("colors.preset: unknown preset %q (known: %s)", c.Preset, knownPresetNames())
	}
	preset.Preset = c.Preset
	return preset, nil
}

func knownPresetNames() string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
