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
	Window      WindowConfig    `toml:"window"`
	Font        FontConfig      `toml:"font"`
	Colors      ColorsConfig    `toml:"colors"`
	Shell       ShellConfig     `toml:"shell"`
	Selection   SelectionConfig `toml:"selection"`
	TabBar      TabBarConfig    `toml:"tabbar"`
	Keybindings []Keybinding    `toml:"keybindings"`
	// Plugins lists directories, each containing a plugin.toml + its
	// entry.wasm, to load at startup (see internal/plugin.Host). Empty by
	// default — plugins are strictly opt-in, matching how every other
	// off-by-default feature in geckty (e.g. OSC 52 clipboard read) is
	// handled: no behavior change unless a user asks for it.
	Plugins []string `toml:"plugins"`
}

// WindowConfig is the [window] section: initial window size.
type WindowConfig struct {
	Width  int `toml:"width"`
	Height int `toml:"height"`
}

// FontConfig is the [font] section.
type FontConfig struct {
	Family string  `toml:"family"`
	Size   float64 `toml:"size"`
}

// ColorsConfig is the [colors] section: default foreground/background plus
// the 16-color ANSI palette.
type ColorsConfig struct {
	// Preset, if set, selects a complete built-in color scheme by name
	// (see presets.go) in place of Foreground/Background/ANSI below —
	// an alternative to specifying them, not a per-field merge with
	// them. Empty (the default) means use Foreground/Background/ANSI
	// directly, as before presets existed.
	Preset     string `toml:"preset"`
	Foreground string `toml:"foreground"`
	Background string `toml:"background"`
	// Selection is the text-selection highlight (macOS Terminal Pro uses
	// opaque #525252). Empty falls back to a blend of Foreground in the
	// theme layer.
	Selection string     `toml:"selection"`
	ANSI      [16]string `toml:"ansi"`
}

// ShellConfig is the [shell] section.
type ShellConfig struct {
	// Command is argv for the shell to launch. Empty means resolve the
	// platform default (see internal/pty's resolveShell).
	Command []string `toml:"command"`
}

// SelectionConfig is the [selection] section, controlling mouse text
// selection behavior (see internal/session.Session.SelectWord).
type SelectionConfig struct {
	// WordChars lists extra characters (beyond letters, digits, and "_",
	// which are always included) treated as part of a word for double-
	// click word selection — e.g. adding "-." lets a double-click select
	// a whole kebab-case-name.ext or path segment in one go, matching
	// how terminals like iTerm2 make this configurable rather than
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
	// between, so — matching Terminal.app/iTerm2's default — the strip
	// only appears once a second tab exists.
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
// colors.preset is special: Default tags Colors with Preset="glass", but
// a config file that sets foreground/background/ansi without mentioning
// preset must keep those explicit colors. If we left Default's Preset
// set, resolvePreset would wipe the file's palette. So Preset is only
// resolved when the file actually defines colors.preset.
func Load(path string) (*Config, error) {
	cfg := Default()
	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, err
	}
	if !md.IsDefined("colors", "preset") {
		cfg.Colors.Preset = ""
	}
	resolved, err := resolvePreset(cfg.Colors)
	if err != nil {
		return nil, err
	}
	cfg.Colors = resolved
	return cfg, nil
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
