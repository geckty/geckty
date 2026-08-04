// Package kittykbd encodes key events per the Kitty keyboard protocol
// (https://sw.kovidgoyal.net/kitty/keyboard-protocol/), gated by
// emu.KeyProtocol — the enable-state emu already tracks natively (the
// CSI > flags u / CSI = flags;mode u / CSI ? u push/pop/query handshake).
// emu has no encoder of its own; this package is that missing half.
//
// Scope: implements the two most commonly needed progressive-enhancement
// flags — Disambiguate escape codes (fixes real ambiguity: a bare Escape
// keypress is indistinguishable from the start of any other escape
// sequence in legacy encoding, and Ctrl+letter combos collide with
// control characters like Tab/Enter/Backspace) and Report event types
// (press/release; "repeat" is not implemented — gpucontext's key callbacks
// only distinguish Press/Release, with no OS-level autorepeat signal to key
// off of).
//
// Not implemented, deliberately: Report alternate keys (needs keyboard-
// layout information — shifted/base-layout variants — that gpucontext's
// key events don't expose), Report all keys as escape codes (would
// require suppressing/replacing the text-input-callback-driven literal-text
// path entirely, a bigger architectural change than this pass), and
// Report associated text (depends on Report all keys). Programs that
// specifically require those flags will not get the enhanced behavior
// they asked for; the flags themselves are still tracked correctly by
// emu (KeyState() reflects them), geckty just doesn't act on them yet.
package kittykbd

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/geckty/geckty/internal/vt/emu"
)

// Key identifies a physical key, as a plain string. This package doesn't
// import gpucontext (only internal/ui/gogpu does); callers map
// gpucontext.Key values to these constants themselves (see
// internal/ui/gogpu/kitty.go's kittyFunctionalKeys), so the glyphs below
// are opaque wire-vocabulary literals, not references to gpucontext
// constants.
type Key string

// Functional keys this package recognizes.
const (
	KeyLeftArrow      Key = "←"
	KeyRightArrow     Key = "→"
	KeyUpArrow        Key = "↑"
	KeyDownArrow      Key = "↓"
	KeyReturn         Key = "⏎"
	KeyEnter          Key = "⌤"
	KeyEscape         Key = "⎋"
	KeyHome           Key = "⇱"
	KeyEnd            Key = "⇲"
	KeyDeleteBackward Key = "⌫"
	KeyDeleteForward  Key = "⌦"
	KeyPageUp         Key = "⇞"
	KeyPageDown       Key = "⇟"
	KeyTab            Key = "Tab"
	KeyF1             Key = "F1"
	KeyF2             Key = "F2"
	KeyF3             Key = "F3"
	KeyF4             Key = "F4"
	KeyF5             Key = "F5"
	KeyF6             Key = "F6"
	KeyF7             Key = "F7"
	KeyF8             Key = "F8"
	KeyF9             Key = "F9"
	KeyF10            Key = "F10"
	KeyF11            Key = "F11"
	KeyF12            Key = "F12"
)

// functionalCodepoints is the Kitty keyboard protocol's unified key-code
// table for non-text keys (spec section "Functional key definitions").
var functionalCodepoints = map[Key]int{
	KeyEscape:        27,
	KeyLeftArrow:     57417,
	KeyRightArrow:    57418,
	KeyUpArrow:       57419,
	KeyDownArrow:     57420,
	KeyPageUp:        57421,
	KeyPageDown:      57422,
	KeyHome:          57423,
	KeyEnd:           57424,
	KeyDeleteForward: 57426, // the "Delete" key; backspace is a separate key, see legacyBytes below
	KeyF1:            57376,
	KeyF2:            57377,
	KeyF3:            57378,
	KeyF4:            57379,
	KeyF5:            57380,
	KeyF6:            57381,
	KeyF7:            57382,
	KeyF8:            57383,
	KeyF9:            57384,
	KeyF10:           57385,
	KeyF11:           57386,
	KeyF12:           57387,
}

// legacyBytes holds the keys that keep their traditional single-byte
// encoding even under Disambiguate, per the spec: "Enter, Tab and
// Backspace keys will not have release events unless Report all keys...
// is also set" — implying (and confirmed by the spec's compatibility
// section) they keep legacy bytes unless emu.KeyReportAllKeys is set,
// which this package doesn't implement (see package doc).
var legacyBytes = map[Key]byte{
	KeyReturn:         '\r',
	KeyEnter:          '\r',
	KeyTab:            '\t',
	KeyDeleteBackward: 0x7f,
}

// Modifiers is a toolkit-agnostic modifier set, matching
// internal/protocol/mouse's pattern — protocol packages don't import
// gpucontext (only internal/ui/gogpu does).
type Modifiers uint8

// Modifier bits.
const (
	ModShift Modifiers = 1 << iota
	ModAlt
	ModCtrl
	ModSuper
)

func (m Modifiers) contain(o Modifiers) bool { return m&o == o }

// kittyModifierBits per the spec: shift=1, alt=2, ctrl=4, super=8 (hyper and
// meta exist in the spec but have no equivalent in gpucontext.Modifiers, so
// are never set here).
func kittyModifierBits(mods Modifiers) int {
	var b int
	if mods.contain(ModShift) {
		b |= 1
	}
	if mods.contain(ModAlt) {
		b |= 2
	}
	if mods.contain(ModCtrl) {
		b |= 4
	}
	if mods.contain(ModSuper) {
		b |= 8
	}
	return b
}

// Event is a toolkit-agnostic key event.
type Event struct {
	Key       Key
	Modifiers Modifiers
	Pressed   bool // true = press, false = release
}

// Encode returns the CSI u sequence for ev under the given Kitty keyboard
// protocol flags. ok is false when this key/flags combination should fall
// through to the caller's legacy encoder instead — either because no
// Kitty flag is active at all, or because the key doesn't need
// disambiguation (a plain unmodified printable key, which should still go
// through the normal key.EditEvent text path unchanged).
func Encode(flags emu.KeyProtocol, ev Event) (out []byte, ok bool) {
	if flags == emu.KeyLegacy {
		return nil, false
	}
	// Only Disambiguate (and the event-type reporting layered on top of
	// it) is implemented — see the package doc for what's not.
	if flags&emu.KeyDisambiguateEscape == 0 {
		return nil, false
	}

	reportEvents := flags&emu.KeyReportEventTypes != 0
	// Without Report event types, only presses are reported (repeat is
	// treated as press). A release must not fall through to press-identical
	// CSI-u — the UI wires OnKeyRelease unconditionally, and duplicating
	// the press encoding would double every functional key (arrows, Esc, …).
	if !ev.Pressed && !reportEvents {
		return nil, false
	}

	if b, isLegacy := legacyBytes[ev.Key]; isLegacy {
		// Legacy keys don't disambiguate and (per the spec) don't
		// report release events under Disambiguate alone.
		if !ev.Pressed {
			return nil, false
		}
		if ev.Modifiers == 0 {
			return []byte{b}, true
		}
		// A modified Enter/Tab (e.g. Ctrl+Enter) is still ambiguous
		// with the plain key, so it does get CSI-u encoded even
		// though the unmodified case doesn't.
		code := int(b)
		return encodeCSIu(code, ev.Modifiers, ev.Pressed, reportEvents), true
	}

	if code, isFunctional := functionalCodepoints[ev.Key]; isFunctional {
		return encodeCSIu(code, ev.Modifiers, ev.Pressed, reportEvents), true
	}

	// A single-rune key name (letters, digits, punctuation) only needs
	// CSI-u encoding if it's ambiguous, i.e. carries Ctrl and/or Alt —
	// Ctrl+A collides with the control character 0x01, and legacy
	// encoding can't tell "Ctrl+A" apart from "the byte 0x01" the way
	// CSI-u can. A plain, unmodified (or shift-only) letter isn't
	// ambiguous and should keep going through the toolkit's text-input
	// callback as normal text, which correctly reflects case/layout in a
	// way this package can't (gpucontext.Key is purely positional — see
	// internal/ui/gogpu/keymap.go's doc comment — with no case or layout
	// information of its own).
	r := []rune(string(ev.Key))
	if len(r) != 1 {
		return nil, false
	}
	if !ev.Modifiers.contain(ModCtrl) && !ev.Modifiers.contain(ModAlt) {
		return nil, false
	}
	code := int(unicode.ToLower(r[0]))
	return encodeCSIu(code, ev.Modifiers, ev.Pressed, reportEvents), true
}

func encodeCSIu(code int, mods Modifiers, pressed, reportEvents bool) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b[%d", code)

	modVal := 1 + kittyModifierBits(mods)
	needModifiers := modVal != 1
	// Press is the default event type and is always omittable; release
	// (this package never emits repeat, see the package doc) needs the
	// ":3" event-type subfield, which forces the modifiers field to be
	// present too (event-type is a subfield of it) even if modVal==1.
	needEvent := reportEvents && !pressed

	if needModifiers || needEvent {
		fmt.Fprintf(&b, ";%d", modVal)
		if needEvent {
			b.WriteString(":3") // release; repeat (2) is never emitted, see package doc
		}
	}
	b.WriteByte('u')
	return []byte(b.String())
}
