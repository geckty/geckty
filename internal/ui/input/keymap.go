package input

import (
	"fmt"
	"strings"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
)

// Action is a named command a keybinding can trigger.
type Action string

// Actions Keymap recognizes — values are config.Action* string constants.
const (
	ActionNewTab              Action = config.ActionNewTab
	ActionCloseTab            Action = config.ActionCloseTab
	ActionNextTab             Action = config.ActionNextTab
	ActionPrevTab             Action = config.ActionPrevTab
	ActionCopy                Action = config.ActionCopy
	ActionPaste               Action = config.ActionPaste
	ActionSearchScrollback    Action = config.ActionSearchScrollback
	ActionOpenURLHints        Action = config.ActionOpenURLHints
	ActionShowScrollback      Action = config.ActionShowScrollback
	ActionIncreaseFontSize    Action = config.ActionIncreaseFontSize
	ActionDecreaseFontSize    Action = config.ActionDecreaseFontSize
	ActionResetFontSize       Action = config.ActionResetFontSize
	ActionSplitVertical       Action = config.ActionSplitVertical
	ActionSplitHorizontal     Action = config.ActionSplitHorizontal
	ActionNextPane            Action = config.ActionNextPane
	ActionPrevPane            Action = config.ActionPrevPane
	ActionClosePane           Action = config.ActionClosePane
	ActionScrollToPrevPrompt  Action = config.ActionScrollToPrevPrompt
	ActionScrollToNextPrompt  Action = config.ActionScrollToNextPrompt
	ActionSelectLastCmdOutput Action = config.ActionSelectLastCmdOutput
)

// ActionByName maps config action strings to typed Actions.
type ActionByName = map[string]Action

// KeyNameTable maps config key names to gpucontext.Key values.
type KeyNameTable = map[string]gpucontext.Key

// ModNameTable maps config modifier names to gpucontext.Modifiers bits.
type ModNameTable = map[string]gpucontext.Modifiers

// ChordBindings maps (key, mods) chords to Actions.
type ChordBindings = map[chord]Action

// knownActions is the canonical list used to build validActions — adding a
// const above without listing it here will leave it unrecognized in TOML.
var knownActions = []Action{
	ActionNewTab, ActionCloseTab, ActionNextTab, ActionPrevTab,
	ActionCopy, ActionPaste, ActionSearchScrollback, ActionOpenURLHints,
	ActionShowScrollback, ActionIncreaseFontSize, ActionDecreaseFontSize,
	ActionResetFontSize, ActionSplitVertical, ActionSplitHorizontal,
	ActionNextPane, ActionPrevPane, ActionClosePane,
	ActionScrollToPrevPrompt, ActionScrollToNextPrompt, ActionSelectLastCmdOutput,
}

var validActions ActionByName

func init() {
	validActions = make(ActionByName, len(knownActions))
	for _, a := range knownActions {
		validActions[string(a)] = a
	}
}

// namedKeyBindings maps config-file key names to gpucontext.Key values for
// the keys that aren't a single letter/digit/punctuation character.
var namedKeyBindings KeyNameTable = KeyNameTable{
	"Tab":       gpucontext.KeyTab,
	"Return":    gpucontext.KeyEnter,
	"Enter":     gpucontext.KeyEnter,
	"Escape":    gpucontext.KeyEscape,
	"Space":     gpucontext.KeySpace,
	"Backspace": gpucontext.KeyBackspace,
	"Delete":    gpucontext.KeyDelete,
	"Left":      gpucontext.KeyLeft,
	"Right":     gpucontext.KeyRight,
	"Up":        gpucontext.KeyUp,
	"Down":      gpucontext.KeyDown,
	"Home":      gpucontext.KeyHome,
	"End":       gpucontext.KeyEnd,
	"PageUp":    gpucontext.KeyPageUp,
	"PageDown":  gpucontext.KeyPageDown,
}

// SingleCharBindings covers config keys that are exactly one printable
// character (letters, digits, and the punctuation geckty's default
// keybindings use). gpucontext.Key is positional (matches USB-HID usage
// codes) rather than the localized character a layout produces — unlike
// a layout-normalized character name, this means a Cmd+C shortcut
// resolves to the same gpucontext.KeyC on any keyboard layout.
var SingleCharBindings KeyNameTable = KeyNameTable{
	"A": gpucontext.KeyA, "B": gpucontext.KeyB, "C": gpucontext.KeyC, "D": gpucontext.KeyD,
	"E": gpucontext.KeyE, "F": gpucontext.KeyF, "G": gpucontext.KeyG, "H": gpucontext.KeyH,
	"I": gpucontext.KeyI, "J": gpucontext.KeyJ, "K": gpucontext.KeyK, "L": gpucontext.KeyL,
	"M": gpucontext.KeyM, "N": gpucontext.KeyN, "O": gpucontext.KeyO, "P": gpucontext.KeyP,
	"Q": gpucontext.KeyQ, "R": gpucontext.KeyR, "S": gpucontext.KeyS, "T": gpucontext.KeyT,
	"U": gpucontext.KeyU, "V": gpucontext.KeyV, "W": gpucontext.KeyW, "X": gpucontext.KeyX,
	"Y": gpucontext.KeyY, "Z": gpucontext.KeyZ,
	"0": gpucontext.Key0, "1": gpucontext.Key1, "2": gpucontext.Key2, "3": gpucontext.Key3,
	"4": gpucontext.Key4, "5": gpucontext.Key5, "6": gpucontext.Key6, "7": gpucontext.Key7,
	"8": gpucontext.Key8, "9": gpucontext.Key9,
	"[": gpucontext.KeyLeftBracket, "]": gpucontext.KeyRightBracket,
	"-": gpucontext.KeyMinus, "=": gpucontext.KeyEqual,
	";": gpucontext.KeySemicolon, "'": gpucontext.KeyApostrophe,
	",": gpucontext.KeyComma, ".": gpucontext.KeyPeriod, "/": gpucontext.KeySlash,
	"\\": gpucontext.KeyBackslash, "`": gpucontext.KeyGrave,
}

var modBindings ModNameTable = ModNameTable{
	config.ModCtrl:  gpucontext.ModControl,
	config.ModShift: gpucontext.ModShift,
	config.ModAlt:   gpucontext.ModAlt,
	config.ModCmd:   gpucontext.ModSuper,
}

// relevantMods is every modifier bit a keybinding can require — CapsLock/
// NumLock are ignored so a platform leaving them set doesn't silently
// break a shortcut.
const relevantMods = gpucontext.ModShift | gpucontext.ModControl | gpucontext.ModAlt | gpucontext.ModSuper

type chord struct {
	key  gpucontext.Key
	mods gpucontext.Modifiers
}

// Keymap resolves gpucontext key presses into configured Actions.
type Keymap struct {
	bindings ChordBindings
}

// NewKeymap validates and compiles cfg into a Keymap. It returns an error
// naming the first unrecognized key, modifier, or action rather than
// silently ignoring a misconfigured binding.
func NewKeymap(cfg []config.Keybinding) (*Keymap, error) {
	k := &Keymap{bindings: make(ChordBindings, len(cfg))}
	for _, kb := range cfg {
		action, ok := validActions[kb.Action]
		if !ok {
			return nil, fmt.Errorf("keybinding %+v: unknown action %q", kb, kb.Action)
		}

		key, ok := namedKeyBindings[kb.Key]
		if !ok {
			key, ok = SingleCharBindings[strings.ToUpper(kb.Key)]
		}
		if !ok {
			if kb.Key == "" {
				return nil, fmt.Errorf("keybinding %+v: empty key", kb)
			}
			return nil, fmt.Errorf("keybinding %+v: unknown key %q", kb, kb.Key)
		}

		var mods gpucontext.Modifiers
		for _, m := range kb.Mods {
			bit, ok := modBindings[m]
			if !ok {
				return nil, fmt.Errorf("keybinding %+v: unknown modifier %q", kb, m)
			}
			mods |= bit
		}

		k.bindings[chord{key: key, mods: mods}] = action
	}
	return k, nil
}

// Match returns the Action bound to (key, mods), if any.
func (k *Keymap) Match(key gpucontext.Key, mods gpucontext.Modifiers) (Action, bool) {
	a, ok := k.bindings[chord{key: key, mods: mods & relevantMods}]
	return a, ok
}
