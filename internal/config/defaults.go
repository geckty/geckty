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
		UI:     defaultUI(),
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
			WheelMultiplier: 3,
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

// defaultColors returns geckty's built-in default color scheme from the
// embedded glass theme. Chrome tab colors are left empty so the palette
// layer derives them with glass blends from Background/Foreground.
func defaultColors() ColorsConfig {
	tf, ok := loadEmbeddedTheme(defaultThemeName)
	if !ok {
		// Should never happen — glass.toml is go:embed'd. Keep a minimal
		// fallback so Default() still returns something paintables.
		return ColorsConfig{
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
		}
	}
	return tf.Colors
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
			{Key: "T", Mods: []string{ModCmd}, Action: ActionNewTab},
			{Key: "W", Mods: []string{ModCmd}, Action: ActionClosePane},
			{Key: "W", Mods: []string{ModCmd, ModShift}, Action: ActionCloseTab},
			{Key: "]", Mods: []string{ModCmd, ModShift}, Action: ActionNextTab},
			{Key: "[", Mods: []string{ModCmd, ModShift}, Action: ActionPrevTab},
			{Key: "C", Mods: []string{ModCmd}, Action: ActionCopy},
			{Key: "V", Mods: []string{ModCmd}, Action: ActionPaste},
			{Key: "F", Mods: []string{ModCmd, ModShift}, Action: ActionSearchScrollback},
			{Key: "E", Mods: []string{ModCmd, ModShift}, Action: ActionOpenURLHints},
			{Key: "H", Mods: []string{ModCtrl, ModShift}, Action: ActionShowScrollback},
			{Key: "=", Mods: []string{ModCmd}, Action: ActionIncreaseFontSize},
			{Key: "-", Mods: []string{ModCmd}, Action: ActionDecreaseFontSize},
			{Key: "0", Mods: []string{ModCmd}, Action: ActionResetFontSize},
			{Key: "D", Mods: []string{ModCmd}, Action: ActionSplitVertical},
			{Key: "D", Mods: []string{ModCmd, ModShift}, Action: ActionSplitHorizontal},
			{Key: "]", Mods: []string{ModCmd, ModAlt}, Action: ActionNextPane},
			{Key: "[", Mods: []string{ModCmd, ModAlt}, Action: ActionPrevPane},
			{Key: "Z", Mods: []string{ModCtrl, ModShift}, Action: ActionScrollToPrevPrompt},
			{Key: "X", Mods: []string{ModCtrl, ModShift}, Action: ActionScrollToNextPrompt},
			{Key: "G", Mods: []string{ModCtrl, ModShift}, Action: ActionSelectLastCmdOutput},
		}
	}
	return []Keybinding{
		{Key: "T", Mods: []string{ModCtrl, ModShift}, Action: ActionNewTab},
		{Key: "W", Mods: []string{ModCtrl, ModShift}, Action: ActionClosePane},
		{Key: "W", Mods: []string{ModCtrl, ModAlt}, Action: ActionCloseTab},
		{Key: "Tab", Mods: []string{ModCtrl}, Action: ActionNextTab},
		{Key: "Tab", Mods: []string{ModCtrl, ModShift}, Action: ActionPrevTab},
		{Key: "C", Mods: []string{ModCtrl, ModShift}, Action: ActionCopy},
		{Key: "V", Mods: []string{ModCtrl, ModShift}, Action: ActionPaste},
		{Key: "F", Mods: []string{ModCtrl, ModShift}, Action: ActionSearchScrollback},
		{Key: "E", Mods: []string{ModCtrl, ModShift}, Action: ActionOpenURLHints},
		{Key: "H", Mods: []string{ModCtrl, ModShift}, Action: ActionShowScrollback},
		{Key: "=", Mods: []string{ModCtrl}, Action: ActionIncreaseFontSize},
		{Key: "-", Mods: []string{ModCtrl}, Action: ActionDecreaseFontSize},
		{Key: "0", Mods: []string{ModCtrl}, Action: ActionResetFontSize},
		{Key: "D", Mods: []string{ModCtrl, ModShift}, Action: ActionSplitVertical},
		{Key: "D", Mods: []string{ModCtrl, ModAlt}, Action: ActionSplitHorizontal},
		{Key: "]", Mods: []string{ModCtrl, ModAlt}, Action: ActionNextPane},
		{Key: "[", Mods: []string{ModCtrl, ModAlt}, Action: ActionPrevPane},
		{Key: "Z", Mods: []string{ModCtrl, ModShift}, Action: ActionScrollToPrevPrompt},
		{Key: "X", Mods: []string{ModCtrl, ModShift}, Action: ActionScrollToNextPrompt},
		{Key: "G", Mods: []string{ModCtrl, ModShift}, Action: ActionSelectLastCmdOutput},
	}
}
