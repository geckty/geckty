// Package config loads geckty's TOML configuration file.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is geckty's user-facing configuration. Sections are added as the
// features they configure are implemented — see defaults.go for values
// used when a field (or the whole file) is absent.
type Config struct {
	Window WindowConfig `toml:"window"`
	Font   FontConfig   `toml:"font"`
	UIFont UIFontConfig `toml:"ui_font"`
	// Theme, if set, loads themes/<name>.toml (next to this config file or
	// under the geckty config dir) or a built-in theme of the same name,
	// then merges any inline [colors] keys on top — Kitty-style include +
	// override, not an all-or-nothing preset.
	Theme  string       `toml:"theme"`
	Colors ColorsConfig `toml:"colors"`
	// UI holds [ui] chrome tokens (bell, scrollbar, indicators, glass
	// blends). Resolved like Colors: defaults ← theme file ← inline.
	UI          UIConfig         `toml:"ui"`
	Shell       ShellConfig      `toml:"shell"`
	Selection   SelectionConfig  `toml:"selection"`
	TabBar      TabBarConfig     `toml:"tabbar"`
	Scrollback  ScrollbackConfig `toml:"scrollback"`
	Cursor      CursorConfig     `toml:"cursor"`
	Clipboard   ClipboardConfig  `toml:"clipboard"`
	Keybindings []Keybinding     `toml:"keybindings"`
	// Plugins lists directories, each containing a plugin.toml + its
	// entry.wasm, to load at startup (see internal/plugin.Host). Empty by
	// default — plugins are strictly opt-in, matching how every other
	// off-by-default feature in geckty (e.g. OSC 52 clipboard read) is
	// handled: no behavior change unless a user asks for it.
	Plugins []string `toml:"plugins"`
	// LogLevel sets the minimum severity geckty logs, one of "debug",
	// "info", "warn", or "error" (case-insensitive). See ParseLogLevel.
	// The -log-level flag (cmd/geckty/main.go) overrides this when set.
	LogLevel string `toml:"log_level"`
	// HotReload enables watching the config file for changes and applying
	// them without restarting (see Watch). Covers colors, font,
	// keybindings, selection, tabbar, cursor, clipboard, and log_level;
	// window size, shell command (existing tabs keep their already-launched
	// shell — only new tabs pick up a changed command), scrollback.lines
	// for already-open tabs, and plugins are only read at startup. On by
	// default: editing the config file and having nothing happen is
	// surprising, not a safety concern like plugins/OSC 52 clipboard read.
	HotReload bool `toml:"hot_reload"`

	// sourcePath is the file Load read this Config from, set by Load and
	// used by Watch. Unexported so it's invisible to the TOML encoder/
	// decoder — it's plumbing, not a user-facing setting.
	sourcePath string
}

// WindowConfig is the [window] section: initial window size and chrome pad.
type WindowConfig struct {
	Width  int `toml:"width"`
	Height int `toml:"height"`
	// Padding is content inset around the terminal grid, in logical dp.
	// Hot-reloaded. Default 8.
	Padding int `toml:"padding"`
	// ConfirmClose asks before quitting when more than one tab is open.
	// Not yet wired to a modal; reserved for the UI close path.
	ConfirmClose bool `toml:"confirm_close"`
}

// ScrollbackConfig is the [scrollback] section.
type ScrollbackConfig struct {
	// Lines caps physical history lines kept per tab. 0 means unlimited
	// (legacy). Default 10000. Applied to newly opened tabs only.
	Lines int `toml:"lines"`
	// WheelMultiplier scales pointer-wheel scroll line counts. Default 3.
	WheelMultiplier float64 `toml:"wheel_multiplier"`
}

// CursorConfig is the [cursor] section.
type CursorConfig struct {
	// Shape is "block", "beam", or "underline". Default "block".
	Shape string `toml:"shape"`
	// Blink enables the soft blink loop when the VT cursor style also blinks.
	Blink bool `toml:"blink"`
	// IntervalMs is the half-period of the blink ticker. Default 530.
	IntervalMs int `toml:"interval_ms"`
	// Color is an optional hex override; empty uses the theme foreground.
	Color string `toml:"color"`
}

// ClipboardConfig is the [clipboard] section (OSC 52 + selection).
type ClipboardConfig struct {
	// OSC52Write is "allow" or "deny". Default "allow".
	OSC52Write string `toml:"osc52_write"`
	// OSC52Read is "allow" or "deny". Default "deny" (exfiltration risk).
	OSC52Read string `toml:"osc52_read"`
	// MaxSize caps OSC 52 write payloads in bytes. 0 means 5 MiB default.
	MaxSize int `toml:"max_size"`
	// CopyOnSelect copies the selection to the clipboard on mouse release.
	CopyOnSelect bool `toml:"copy_on_select"`
}

// FontConfig is the [font] section: the terminal grid's typeface.
type FontConfig struct {
	// Family is a font name to look up on disk (see font.go's
	// configuredFamilyPaths for the filename guesses this tries), the
	// literal string "monospace" to skip straight to the platform's own
	// default monospace search, or empty to use geckty's bundled default
	// (IBM Plex Mono, see assets.Fonts) directly without touching disk.
	Family string  `toml:"family"`
	Size   float64 `toml:"size"`
	// Bold, when false, never renders SGR-bold text in the family's bold
	// weight — it stays regular-weight (the bold *attribute* still exists
	// in the terminal state, e.g. for bright-color ANSI codes that key off
	// it; only the font weight switch is disabled). On by default.
	Bold bool `toml:"bold"`
	// Italic is Bold's equivalent for the SGR-italic attribute.
	Italic bool `toml:"italic"`
}

// UIFontConfig is the [ui_font] section: the tab bar's typeface —
// independent of Font since chrome text has no reason to share the
// terminal grid's monospacing. Family/Size follow FontConfig's own rules
// (with geckty's bundled PT Sans as the empty-Family default); there's no
// Bold/Italic here since chrome text doesn't carry SGR attributes to
// switch weight on.
type UIFontConfig struct {
	Family string  `toml:"family"`
	Size   float64 `toml:"size"`
}

// ColorsConfig is the [colors] section: terminal palette plus optional
// chrome (tab bar) colors. Empty chrome keys are derived at the palette
// layer from Background/Foreground (glass blends). Themes are free-form
// color maps — see Theme on Config and themes/*.toml — not closed presets.
type ColorsConfig struct {
	// Preset is deprecated: treated as an alias for Config.Theme when
	// theme is unset. Prefer top-level theme = "name".
	Preset     string `toml:"preset"`
	Foreground string `toml:"foreground"`
	Background string `toml:"background"`
	// Selection is the legacy selection-highlight key (opaque hex). Prefer
	// selection_background; both are accepted, selection_background wins.
	Selection           string `toml:"selection"`
	SelectionBackground string `toml:"selection_background"`
	SelectionForeground string `toml:"selection_foreground"`
	// Cursor is the caret color; empty uses foreground (overridden by
	// [cursor].color when that is set).
	Cursor string `toml:"cursor"`
	// Tab chrome (Kitty-style keys). Empty → glass-derived from Background.
	ActiveTabForeground   string     `toml:"active_tab_foreground"`
	ActiveTabBackground   string     `toml:"active_tab_background"`
	InactiveTabForeground string     `toml:"inactive_tab_foreground"`
	InactiveTabBackground string     `toml:"inactive_tab_background"`
	TabBarBackground      string     `toml:"tab_bar_background"`
	HoverTabBackground    string     `toml:"hover_tab_background"`
	PlusButtonBackground  string     `toml:"plus_button_background"`
	ANSI                  [16]string `toml:"ansi"`
}

// ShellConfig is the [shell] section.
type ShellConfig struct {
	// Command is argv for the shell to launch. Empty means resolve the
	// platform default (see internal/pty's resolveShell).
	Command []string `toml:"command"`
	// Env is appended to the child environment (KEY=value entries).
	Env []string `toml:"env"`
	// WorkingDir is the initial cwd for new tabs. Empty means the user
	// home directory (see ui wireFirstTab).
	WorkingDir string `toml:"working_dir"`
	// Integration injects OSC 133 semantic-prompt hooks (precmd/preexec)
	// into the resolved default shell's startup — zsh and bash only, and
	// only when Command is empty; an explicit Command is the user's exact
	// choice and is never modified. This is what makes the "command
	// running" indicator (window border + tab-bar dot) actually light up:
	// without a shell emitting OSC 133, geckty has no way to know a
	// command started or finished. See internal/pty's shell integration
	// scripts for exactly what gets injected — the user's real
	// .zshenv/.zshrc/.bashrc is still sourced in full, this only adds to
	// it. On by default so the indicator works out of the box; set to
	// false to spawn the shell completely unmodified.
	Integration bool `toml:"integration"`
}

// SelectionConfig is the [selection] section, controlling mouse text
// selection behavior (see internal/session.Session.SelectWord).
type SelectionConfig struct {
	// WordChars lists extra characters (beyond letters, digits, and "_",
	// which are always included) treated as part of a word for double-
	// click word selection — e.g. adding "-." lets a double-click select
	// a whole kebab-case-name.ext or path segment in one go, rather than
	// hardcoding one definition of "word".
	WordChars string `toml:"word_chars"`
}

// TabBarConfig is the [tabbar] section, controlling the tab strip's own
// visibility — independent of what it's showing (tab-title formatting
// etc. isn't configurable here, only whether the strip and its "+" button
// appear at all).
type TabBarConfig struct {
	// Hidden disables the tab bar entirely — no tab strip and no "+"
	// button, regardless of ShowThreshold or PlusButton below, and
	// regardless of how many tabs are open. Off by default.
	Hidden bool `toml:"hidden"`
	// ShowThreshold is the minimum number of open tabs before the tab
	// strip (the row of tab pills) is shown, when not Hidden. Values below
	// 1 behave as 1. Default 2: a single tab has nothing to switch
	// between, so the strip only appears once a second tab exists.
	ShowThreshold int `toml:"show_threshold"`
	// PlusButton controls the "+" new-tab button's own visibility,
	// independent of ShowThreshold above (e.g. keep "+" visible with a
	// single tab open even though the tab strip itself stays hidden until
	// a second one exists, or hide "+" entirely and rely on a keybinding
	// for new_tab instead).
	PlusButton PlusButtonConfig `toml:"plus_button"`
}

// PlusButtonConfig is the [tabbar.plus_button] section.
type PlusButtonConfig struct {
	// Hidden disables the "+" button entirely, regardless of ShowThreshold
	// or how many tabs are open. Off by default.
	Hidden bool `toml:"hidden"`
	// ShowThreshold is the minimum number of open tabs before "+" is
	// shown, when not Hidden (and TabBarConfig.Hidden is also false).
	// Values below 1 behave as 1. Default 2, matching TabBarConfig's own
	// default so both appear together at the same tab count unless
	// configured otherwise.
	ShowThreshold int `toml:"show_threshold"`
}

// Keybinding is one [[keybindings]] entry: Key is a gpucontext.Key name
// ("T", "Tab", "["...), Mods is a set of "ctrl"/"shift"/"alt"/"cmd", and
// Action is one of the action names internal/ui/input.Keymap understands
// (e.g. "new_tab"). Unrecognized Key/Mods/Action values are rejected at
// keymap construction, not silently ignored.
type Keybinding struct {
	Key    string   `toml:"key"`
	Mods   []string `toml:"mods"`
	Action string   `toml:"action"`
}

// DefaultPath returns the platform-conventional config file location:
// $XDG_CONFIG_HOME/geckty/config.toml, falling back to ~/.config/geckty on
// systems without XDG_CONFIG_HOME set (including macOS and Windows, which
// don't set it by default — geckty uses the same XDG-style layout
// everywhere for consistency rather than following each OS's native
// convention).
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "geckty", "config.toml"), nil
}

// Load reads and parses the TOML file at path, filling any field absent
// from the file with its default value.
//
// Color/UI resolution is defaults ← theme file (embedded or on disk) ←
// inline [colors]/[ui] (Kitty-style merge). colors.preset is accepted as
// a deprecated alias for top-level theme when theme is unset.
func Load(path string) (*Config, error) {
	cfg := Default()
	cfg.sourcePath = path
	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// DecodeFile returns before touching cfg when it can't even
			// open the file, so cfg is still pure Default() values here.
			return cfg, nil
		}
		return nil, err
	}
	if err := resolveTheme(cfg, md, path); err != nil {
		return nil, err
	}
	return cfg, nil
}

// resolveTheme rebuilds cfg.Colors and cfg.UI as defaults ← theme ← overrides.
func resolveTheme(cfg *Config, md toml.MetaData, configPath string) error {
	themeName := cfg.Theme
	if themeName == "" && md.IsDefined("colors", "preset") && cfg.Colors.Preset != "" {
		themeName = cfg.Colors.Preset
	}
	cfg.Theme = themeName
	cfg.Colors.Preset = ""

	colorOverrides := colorsOverridesFrom(md, cfg.Colors)
	uiOverrides := uiOverridesFrom(md, cfg.UI)
	mergedColors := defaultColors()
	mergedUI := defaultUI()
	if themeName != "" {
		tf, err := loadThemeFile(themeName, configPath)
		if err != nil {
			return err
		}
		mergedColors = mergeColors(mergedColors, tf.Colors)
		mergedUI = mergeUI(mergedUI, tf.UI)
	}
	cfg.Colors = mergeColors(mergedColors, colorOverrides)
	cfg.UI = mergeUI(mergedUI, uiOverrides)
	return nil
}

// EnsureDefaultFile writes the default config to path if nothing exists
// there yet. It does not overwrite an existing file.
func EnsureDefaultFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	// path is the resolved config path (see DefaultPath), not
	// user-controlled input from a different trust boundary.
	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return toml.NewEncoder(f).Encode(Default())
}

// ShellCommand returns the configured shell argv, or nil to signal that the
// platform default should be resolved.
func (c *Config) ShellCommand() []string {
	return c.Shell.Command
}
