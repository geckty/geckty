package gogpu

import (
	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/protocol/kittykbd"
	"github.com/geckty/geckty/internal/vt/emu"
)

// kittyFunctionalKeys maps gpucontext's positional key codes to the string
// identities kittykbd.Key expects (originally gio key.Name glyphs — kept
// as-is since kittykbd treats them as an opaque wire vocabulary, not a gio
// type; see kittykbd's own doc comment).
var kittyFunctionalKeys = map[gpucontext.Key]kittykbd.Key{
	gpucontext.KeyLeft:      kittykbd.KeyLeftArrow,
	gpucontext.KeyRight:     kittykbd.KeyRightArrow,
	gpucontext.KeyUp:        kittykbd.KeyUpArrow,
	gpucontext.KeyDown:      kittykbd.KeyDownArrow,
	gpucontext.KeyEnter:     kittykbd.KeyReturn,
	gpucontext.KeyEscape:    kittykbd.KeyEscape,
	gpucontext.KeyHome:      kittykbd.KeyHome,
	gpucontext.KeyEnd:       kittykbd.KeyEnd,
	gpucontext.KeyBackspace: kittykbd.KeyDeleteBackward,
	gpucontext.KeyDelete:    kittykbd.KeyDeleteForward,
	gpucontext.KeyPageUp:    kittykbd.KeyPageUp,
	gpucontext.KeyPageDown:  kittykbd.KeyPageDown,
	gpucontext.KeyTab:       kittykbd.KeyTab,
	gpucontext.KeyF1:        kittykbd.KeyF1,
	gpucontext.KeyF2:        kittykbd.KeyF2,
	gpucontext.KeyF3:        kittykbd.KeyF3,
	gpucontext.KeyF4:        kittykbd.KeyF4,
	gpucontext.KeyF5:        kittykbd.KeyF5,
	gpucontext.KeyF6:        kittykbd.KeyF6,
	gpucontext.KeyF7:        kittykbd.KeyF7,
	gpucontext.KeyF8:        kittykbd.KeyF8,
	gpucontext.KeyF9:        kittykbd.KeyF9,
	gpucontext.KeyF10:       kittykbd.KeyF10,
	gpucontext.KeyF11:       kittykbd.KeyF11,
	gpucontext.KeyF12:       kittykbd.KeyF12,
}

// keyToChar maps a single-character key (letter/digit/punctuation) to the
// one-rune string kittykbd.Encode expects for Ctrl/Alt-disambiguation —
// built once from keymap.go's singleCharBindings (its inverse).
var keyToChar = func() map[gpucontext.Key]string {
	m := make(map[gpucontext.Key]string, len(singleCharBindings))
	for s, k := range singleCharBindings {
		m[k] = s
	}
	return m
}()

// EncodeKitty converts a key press into Kitty-keyboard-protocol bytes under
// the given emu.KeyProtocol flags (see vt.Terminal.KeyState()). ok is false
// whenever kittykbd.Encode declines (no Kitty flag active, the key isn't
// ambiguous, or unimplemented flag combination — see kittykbd's package
// doc), in which case the caller should fall back to EncodeKey/EncodeText.
func EncodeKitty(flags emu.KeyProtocol, key gpucontext.Key, mods gpucontext.Modifiers, pressed bool) ([]byte, bool) {
	var kk kittykbd.Key
	if fk, ok := kittyFunctionalKeys[key]; ok {
		kk = fk
	} else if ch, ok := keyToChar[key]; ok {
		kk = kittykbd.Key(ch)
	} else {
		return nil, false
	}
	return kittykbd.Encode(flags, kittykbd.Event{
		Key:       kk,
		Modifiers: kittyModifiers(mods),
		Pressed:   pressed,
	})
}

func kittyModifiers(m gpucontext.Modifiers) kittykbd.Modifiers {
	var out kittykbd.Modifiers
	if m.HasShift() {
		out |= kittykbd.ModShift
	}
	if m.HasAlt() {
		out |= kittykbd.ModAlt
	}
	if m.HasControl() {
		out |= kittykbd.ModCtrl
	}
	if m.HasSuper() {
		out |= kittykbd.ModSuper
	}
	return out
}
