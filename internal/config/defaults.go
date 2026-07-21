package config

import "runtime"

// Default returns a Config populated with geckty's built-in defaults, used
// whenever no config file exists or a section/field is omitted from one
// that does.
func Default() *Config {
	return &Config{
		Window: WindowConfig{Width: 1000, Height: 650},
		Font:   FontConfig{Family: "monospace", Size: 13},
		Colors: defaultColors(),
		Selection: SelectionConfig{
			// Letters/digits/"_" always count; extra chars match macOS
			// Terminal.app's double-click on filenames like
			// config.example.toml ("." / "-").
			WordChars: "._-",
		},
		TabBar: TabBarConfig{
			// 2 (not 1) so a single open tab shows neither the strip nor
			// "+" — matching Terminal.app/iTerm2's default of staying out
			// of the way until there's a second tab to switch between.
			ShowThreshold: 2,
			PlusButton:    PlusButtonConfig{ShowThreshold: 2},
		},
		Keybindings: defaultKeybindings(),
		// "error": geckty is desktop software, not a service someone
		// tails logs of by default — stay quiet unless something's
		// actually gone wrong. Raise via log_level or -log-level to
		// debug a specific issue.
		LogLevel: "error",
	}
}

// defaultColors returns geckty's built-in default color scheme: the
// "glass" preset (see presets.go), chosen as the default rather than left
// opt-in.
func defaultColors() ColorsConfig {
	c := presets[glassPresetName]
	c.Preset = glassPresetName
	return c
}

// defaultKeybindings avoids plain Ctrl+T/Ctrl+W/Ctrl+C/Ctrl+V: those are
// shell readline/job-control bindings (transpose-char, delete-word-
// backward, SIGINT, literal-next) that must reach the shell untouched, not
// get intercepted. macOS gets Cmd-based bindings matching Terminal.app/
// iTerm2 convention (Cmd isn't consumed by the shell, so plain Cmd+C/Cmd+V
// are safe there); other platforms get Ctrl+Shift, matching VS Code's
// integrated terminal, Windows Terminal, and GNOME Terminal's Ctrl+Shift+C/V.
func defaultKeybindings() []Keybinding {
	if runtime.GOOS == "darwin" {
		return []Keybinding{
			{Key: "T", Mods: []string{"cmd"}, Action: "new_tab"},
			{Key: "W", Mods: []string{"cmd"}, Action: "close_tab"},
			{Key: "]", Mods: []string{"cmd", "shift"}, Action: "next_tab"},
			{Key: "[", Mods: []string{"cmd", "shift"}, Action: "prev_tab"},
			{Key: "C", Mods: []string{"cmd"}, Action: "copy"},
			{Key: "V", Mods: []string{"cmd"}, Action: "paste"},
		}
	}
	return []Keybinding{
		{Key: "T", Mods: []string{"ctrl", "shift"}, Action: "new_tab"},
		{Key: "W", Mods: []string{"ctrl", "shift"}, Action: "close_tab"},
		{Key: "Tab", Mods: []string{"ctrl"}, Action: "next_tab"},
		{Key: "Tab", Mods: []string{"ctrl", "shift"}, Action: "prev_tab"},
		{Key: "C", Mods: []string{"ctrl", "shift"}, Action: "copy"},
		{Key: "V", Mods: []string{"ctrl", "shift"}, Action: "paste"},
	}
}
