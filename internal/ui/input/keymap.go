package input

import (
	"fmt"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"

	"github.com/geckty/geckty/internal/config"
)

// Action is a named command a keybinding can trigger.
type Action string

// Actions Keymap recognizes.
const (
	ActionNewTab   Action = "new_tab"
	ActionCloseTab Action = "close_tab"
	ActionNextTab  Action = "next_tab"
	ActionPrevTab  Action = "prev_tab"
	ActionCopy     Action = "copy"
	ActionPaste    Action = "paste"
)

var validActions = map[string]Action{
	string(ActionNewTab):   ActionNewTab,
	string(ActionCloseTab): ActionCloseTab,
	string(ActionNextTab):  ActionNextTab,
	string(ActionPrevTab):  ActionPrevTab,
	string(ActionCopy):     ActionCopy,
	string(ActionPaste):    ActionPaste,
}

// namedKeyBindings maps config-file key names to gio key.Name values for
// the keys that aren't already their own name (letters/digits/punctuation
// are, per gio's key.Name doc — "T" is key.Name("T")). Kept in sync with
// keyboard.go's namedKeys where the two overlap.
var namedKeyBindings = map[string]key.Name{
	"Tab":       key.NameTab,
	"Return":    key.NameReturn,
	"Enter":     key.NameEnter,
	"Escape":    key.NameEscape,
	"Space":     key.NameSpace,
	"Backspace": key.NameDeleteBackward,
	"Delete":    key.NameDeleteForward,
	"Left":      key.NameLeftArrow,
	"Right":     key.NameRightArrow,
	"Up":        key.NameUpArrow,
	"Down":      key.NameDownArrow,
	"Home":      key.NameHome,
	"End":       key.NameEnd,
	"PageUp":    key.NamePageUp,
	"PageDown":  key.NamePageDown,
}

var modBindings = map[string]key.Modifiers{
	"ctrl":  key.ModCtrl,
	"shift": key.ModShift,
	"alt":   key.ModAlt,
	"cmd":   key.ModCommand,
}

type chord struct {
	name key.Name
	mods key.Modifiers
}

// Keymap resolves gio key.Event values into configured Actions.
type Keymap struct {
	bindings map[chord]Action
}

// NewKeymap validates and compiles cfg into a Keymap. It returns an error
// naming the first unrecognized key, modifier, or action rather than
// silently ignoring a misconfigured binding.
func NewKeymap(cfg []config.Keybinding) (*Keymap, error) {
	k := &Keymap{bindings: make(map[chord]Action, len(cfg))}
	for _, kb := range cfg {
		action, ok := validActions[kb.Action]
		if !ok {
			return nil, fmt.Errorf("keybinding %+v: unknown action %q", kb, kb.Action)
		}

		name, ok := namedKeyBindings[kb.Key]
		if !ok {
			if kb.Key == "" {
				return nil, fmt.Errorf("keybinding %+v: empty key", kb)
			}
			name = key.Name(kb.Key)
		}

		var mods key.Modifiers
		for _, m := range kb.Mods {
			bit, ok := modBindings[m]
			if !ok {
				return nil, fmt.Errorf("keybinding %+v: unknown modifier %q", kb, m)
			}
			mods |= bit
		}

		k.bindings[chord{name: name, mods: mods}] = action
	}
	return k, nil
}

// Match returns the Action bound to e, if any. Only key.Press events can
// match — matching on Release would fire every bound action twice.
//
// Modifiers must match exactly (a binding for Cmd+C must not also fire
// for Cmd+Shift+C). Caps/Num lock bits are ignored — platforms sometimes
// leave them set and that silently broke copy/paste.
func (k *Keymap) Match(e key.Event) (Action, bool) {
	if e.State != key.Press {
		return "", false
	}
	mods := e.Modifiers & (key.ModCommand | key.ModCtrl | key.ModAlt | key.ModShift)
	a, ok := k.bindings[chord{name: e.Name, mods: mods}]
	if ok {
		return a, true
	}
	// macOS: on non-Latin layouts (e.g. Russian), key.Name for letter
	// shortcuts can come in layout-local letters. Map the common shortcut
	// keys by physical position so Cmd+C/V/T/W still work regardless of
	// active layout.
	norm := normalizeShortcutName(e.Name)
	if norm == e.Name {
		return "", false
	}
	a, ok = k.bindings[chord{name: norm, mods: mods}]
	return a, ok
}

func normalizeShortcutName(name key.Name) key.Name {
	switch strings.ToUpper(string(name)) {
	case "С": // russian key at latin C
		return "C"
	case "М": // russian key at latin V
		return "V"
	case "Е": // russian key at latin T
		return "T"
	case "Ц": // russian key at latin W
		return "W"
	case "Х": // russian key at latin [
		return "["
	case "Ъ": // russian key at latin ]
		return "]"
	default:
		return name
	}
}

// Filters returns the event.Filter values needed for tag to receive the
// key.Event values Match can resolve.
func (k *Keymap) Filters(tag event.Tag) []event.Filter {
	filters := make([]event.Filter, 0, len(k.bindings))
	for c := range k.bindings {
		filters = append(filters, key.Filter{Focus: tag, Name: c.name, Required: c.mods})
		for _, alias := range shortcutAliasNames(c.name) {
			filters = append(filters, key.Filter{Focus: tag, Name: alias, Required: c.mods})
		}
	}
	return filters
}

func shortcutAliasNames(name key.Name) []key.Name {
	switch strings.ToUpper(string(name)) {
	case "C":
		return []key.Name{"С"}
	case "V":
		return []key.Name{"М"}
	case "T":
		return []key.Name{"Е"}
	case "W":
		return []key.Name{"Ц"}
	case "[":
		return []key.Name{"Х"}
	case "]":
		return []key.Name{"Ъ"}
	default:
		return nil
	}
}
