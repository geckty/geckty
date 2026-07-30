package config

import "runtime"

// Default returns a Config populated with geckty's built-in defaults, used
// whenever no config file exists or a section/field is omitted from one
// that does.
func Default() *Config {
	return &Config{
		Window: WindowConfig{Width: 1000, Height: 650, Padding: 8},
		// Family "": geckty's bundled IBM Plex Mono/PT Sans (see
		// assets.Fonts and FontConfig/UIFontConfig's doc comments) rather
		// than "monospace"'s platform-default search — a consistent,
		// good-looking default everywhere instead of depending on what a
		// given machine happens to have installed.
		Font:   FontConfig{Family: "", Size: 13, Bold: true, Italic: true},
		UIFont: UIFontConfig{Family: "", Size: 12},
		Colors: defaultColors(),
		Shell: ShellConfig{
			// See Integration's doc comment: on by default, only touches
			// zsh/bash, and only when Command (empty here) is left at its
			// own default.
			Integration: true,
		},
		Selection: SelectionConfig{
			WordChars: "._-",
		},
		TabBar: TabBarConfig{
			ShowThreshold: 2,
			PlusButton:    PlusButtonConfig{ShowThreshold: 2},
		},
		Scrollback: ScrollbackConfig{
			Lines:           10000,
			WheelMultiplier: 1,
		},
		Cursor: CursorConfig{
			Shape:      "block",
			Blink:      true,
			IntervalMs: 530,
		},
		Clipboard: ClipboardConfig{
			OSC52Write: "allow",
			OSC52Read:  "deny",
			MaxSize:    5 << 20, // 5 MiB
		},
		Keybindings: defaultKeybindings(),
		LogLevel:    "error",
		HotReload:   true,
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
// get intercepted. macOS gets Cmd-based bindings (Cmd isn't consumed by
// the shell there, so plain Cmd+C/Cmd+V are safe); other platforms get
// Ctrl+Shift, matching how VS Code's integrated terminal, Windows
// Terminal, and GNOME Terminal all bind Ctrl+Shift+C/V.
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
