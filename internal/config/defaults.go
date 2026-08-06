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

// defaultColors returns geckty's built-in default color scheme (the
// "glass" theme values). Chrome tab colors are left empty so the palette
// layer derives them with glass blends from Background/Foreground.
func defaultColors() ColorsConfig {
	return builtinThemes[glassThemeName]
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
			{Key: "W", Mods: []string{"cmd"}, Action: "close_pane"},
			{Key: "W", Mods: []string{"cmd", "shift"}, Action: "close_tab"},
			{Key: "]", Mods: []string{"cmd", "shift"}, Action: "next_tab"},
			{Key: "[", Mods: []string{"cmd", "shift"}, Action: "prev_tab"},
			{Key: "C", Mods: []string{"cmd"}, Action: "copy"},
			{Key: "V", Mods: []string{"cmd"}, Action: "paste"},
			{Key: "F", Mods: []string{"cmd", "shift"}, Action: "search_scrollback"},
			{Key: "E", Mods: []string{"cmd", "shift"}, Action: "open_url_hints"},
			{Key: "H", Mods: []string{"ctrl", "shift"}, Action: "show_scrollback"},
			{Key: "=", Mods: []string{"cmd"}, Action: "increase_font_size"},
			{Key: "-", Mods: []string{"cmd"}, Action: "decrease_font_size"},
			{Key: "0", Mods: []string{"cmd"}, Action: "reset_font_size"},
			{Key: "D", Mods: []string{"cmd"}, Action: "split_vertical"},
			{Key: "D", Mods: []string{"cmd", "shift"}, Action: "split_horizontal"},
			{Key: "]", Mods: []string{"cmd", "alt"}, Action: "next_pane"},
			{Key: "[", Mods: []string{"cmd", "alt"}, Action: "prev_pane"},
			{Key: "Z", Mods: []string{"ctrl", "shift"}, Action: "scroll_to_prev_prompt"},
			{Key: "X", Mods: []string{"ctrl", "shift"}, Action: "scroll_to_next_prompt"},
			{Key: "G", Mods: []string{"ctrl", "shift"}, Action: "select_last_command_output"},
		}
	}
	return []Keybinding{
		{Key: "T", Mods: []string{"ctrl", "shift"}, Action: "new_tab"},
		{Key: "W", Mods: []string{"ctrl", "shift"}, Action: "close_pane"},
		{Key: "W", Mods: []string{"ctrl", "alt"}, Action: "close_tab"},
		{Key: "Tab", Mods: []string{"ctrl"}, Action: "next_tab"},
		{Key: "Tab", Mods: []string{"ctrl", "shift"}, Action: "prev_tab"},
		{Key: "C", Mods: []string{"ctrl", "shift"}, Action: "copy"},
		{Key: "V", Mods: []string{"ctrl", "shift"}, Action: "paste"},
		{Key: "F", Mods: []string{"ctrl", "shift"}, Action: "search_scrollback"},
		{Key: "E", Mods: []string{"ctrl", "shift"}, Action: "open_url_hints"},
		{Key: "H", Mods: []string{"ctrl", "shift"}, Action: "show_scrollback"},
		{Key: "=", Mods: []string{"ctrl"}, Action: "increase_font_size"},
		{Key: "-", Mods: []string{"ctrl"}, Action: "decrease_font_size"},
		{Key: "0", Mods: []string{"ctrl"}, Action: "reset_font_size"},
		{Key: "D", Mods: []string{"ctrl", "shift"}, Action: "split_vertical"},
		{Key: "D", Mods: []string{"ctrl", "alt"}, Action: "split_horizontal"},
		{Key: "]", Mods: []string{"ctrl", "alt"}, Action: "next_pane"},
		{Key: "[", Mods: []string{"ctrl", "alt"}, Action: "prev_pane"},
		{Key: "Z", Mods: []string{"ctrl", "shift"}, Action: "scroll_to_prev_prompt"},
		{Key: "X", Mods: []string{"ctrl", "shift"}, Action: "scroll_to_next_prompt"},
		{Key: "G", Mods: []string{"ctrl", "shift"}, Action: "select_last_command_output"},
	}
}
