// Package input translates gio keyboard/text events into the byte
// sequences a terminal shell expects on its stdin.
//
// This is the legacy/xterm-style baseline encoder. gio's key.Event.Name is
// always the upper-case, shift-normalized key identity (see gio's own
// doc comment on key.Name) — it is meant for shortcut matching, not literal
// text. Actual typed text (respecting shift, layout, and case) arrives
// separately as key.EditEvent, so control keys and printable text are
// handled through two different gio event types here, not one.
package input

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
)

// Filters returns the event filters that must be passed to
// layout.Context.Event for tag to receive the key/text events Encode
// understands.
func Filters(tag event.Tag) []event.Filter {
	return []event.Filter{
		key.FocusFilter{Target: tag},

		key.Filter{Focus: tag, Name: key.NameReturn},
		key.Filter{Focus: tag, Name: key.NameEnter},
		key.Filter{Focus: tag, Name: key.NameDeleteBackward},
		key.Filter{Focus: tag, Name: key.NameDeleteForward},
		key.Filter{Focus: tag, Name: key.NameTab},
		key.Filter{Focus: tag, Name: key.NameEscape},
		key.Filter{Focus: tag, Name: key.NameLeftArrow},
		key.Filter{Focus: tag, Name: key.NameRightArrow},
		key.Filter{Focus: tag, Name: key.NameUpArrow},
		key.Filter{Focus: tag, Name: key.NameDownArrow},
		key.Filter{Focus: tag, Name: key.NameHome},
		key.Filter{Focus: tag, Name: key.NameEnd},
		key.Filter{Focus: tag, Name: key.NamePageUp},
		key.Filter{Focus: tag, Name: key.NamePageDown},
		key.Filter{Focus: tag, Name: key.NameF1},
		key.Filter{Focus: tag, Name: key.NameF2},
		key.Filter{Focus: tag, Name: key.NameF3},
		key.Filter{Focus: tag, Name: key.NameF4},
		key.Filter{Focus: tag, Name: key.NameF5},
		key.Filter{Focus: tag, Name: key.NameF6},
		key.Filter{Focus: tag, Name: key.NameF7},
		key.Filter{Focus: tag, Name: key.NameF8},
		key.Filter{Focus: tag, Name: key.NameF9},
		key.Filter{Focus: tag, Name: key.NameF10},
		key.Filter{Focus: tag, Name: key.NameF11},
		key.Filter{Focus: tag, Name: key.NameF12},

		// Ctrl+letter -> control bytes (Ctrl+C, Ctrl+D, Ctrl+A, ...).
		// key.Name for letters is already upper-case regardless of
		// shift, so a single filter per A-Z covers both cases.
		key.Filter{Focus: tag, Name: "A", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "B", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "C", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "D", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "E", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "F", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "G", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "H", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "I", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "J", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "K", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "L", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "M", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "N", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "O", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "P", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "Q", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "R", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "S", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "T", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "U", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "V", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "W", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "X", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "Y", Required: key.ModCtrl},
		key.Filter{Focus: tag, Name: "Z", Required: key.ModCtrl},

		// Alt+letter -> otherwise unrequested; only needed so
		// EncodeKitty's Kitty-keyboard-protocol Disambiguate path (see
		// kitty.go) can see and encode these when active. In legacy
		// mode these still produce nothing (matching pre-existing
		// behavior — Alt+letter legacy encoding isn't implemented
		// either way, so this is a no-op for non-Kitty sessions).
		key.Filter{Focus: tag, Name: "A", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "B", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "C", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "D", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "E", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "F", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "G", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "H", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "I", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "J", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "K", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "L", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "M", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "N", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "O", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "P", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "Q", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "R", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "S", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "T", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "U", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "V", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "W", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "X", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "Y", Required: key.ModAlt},
		key.Filter{Focus: tag, Name: "Z", Required: key.ModAlt},
	}
}

var namedKeys = map[key.Name][]byte{
	key.NameReturn:         {'\r'},
	key.NameEnter:          {'\r'},
	key.NameDeleteBackward: {0x7f},
	key.NameDeleteForward:  {0x1b, '[', '3', '~'},
	key.NameTab:            {'\t'},
	key.NameEscape:         {0x1b},
	key.NameLeftArrow:      {0x1b, '[', 'D'},
	key.NameRightArrow:     {0x1b, '[', 'C'},
	key.NameUpArrow:        {0x1b, '[', 'A'},
	key.NameDownArrow:      {0x1b, '[', 'B'},
	key.NameHome:           {0x1b, '[', 'H'},
	key.NameEnd:            {0x1b, '[', 'F'},
	key.NamePageUp:         {0x1b, '[', '5', '~'},
	key.NamePageDown:       {0x1b, '[', '6', '~'},
	key.NameF1:             {0x1b, 'O', 'P'},
	key.NameF2:             {0x1b, 'O', 'Q'},
	key.NameF3:             {0x1b, 'O', 'R'},
	key.NameF4:             {0x1b, 'O', 'S'},
	key.NameF5:             {0x1b, '[', '1', '5', '~'},
	key.NameF6:             {0x1b, '[', '1', '7', '~'},
	key.NameF7:             {0x1b, '[', '1', '8', '~'},
	key.NameF8:             {0x1b, '[', '1', '9', '~'},
	key.NameF9:             {0x1b, '[', '2', '0', '~'},
	key.NameF10:            {0x1b, '[', '2', '1', '~'},
	key.NameF11:            {0x1b, '[', '2', '3', '~'},
	key.NameF12:            {0x1b, '[', '2', '4', '~'},
}

// Encode converts a key.Event into the bytes to write to the shell. ok is
// false for events Encode has nothing to say about (e.g. Release, or a
// plain letter/digit press whose text arrives via key.EditEvent instead).
func Encode(e key.Event) (out []byte, ok bool) {
	if e.State != key.Press {
		return nil, false
	}

	if e.Modifiers.Contain(key.ModCtrl) && len(e.Name) == 1 {
		c := e.Name[0]
		if c >= 'A' && c <= 'Z' {
			return []byte{c - 'A' + 1}, true
		}
	}

	if b, ok := namedKeys[e.Name]; ok {
		return b, true
	}
	return nil, false
}

// EncodeText returns the bytes for a key.EditEvent's literal text.
func EncodeText(text string) []byte {
	return []byte(text)
}
