package input

import (
	"gioui.org/io/key"

	"github.com/geckty/geckty/internal/protocol/kittykbd"
	"github.com/geckty/geckty/internal/vt/emu"
)

// EncodeKitty converts a gio key.Event into Kitty-keyboard-protocol bytes
// under the given emu.KeyProtocol flags (see vt.Terminal.KeyState()). ok is
// false whenever kittykbd.Encode declines — no Kitty flag active, the key
// isn't ambiguous, or the flag combination isn't implemented (see
// internal/protocol/kittykbd's package doc for scope) — in which case the
// caller should fall back to Encode (legacy) / EncodeText as usual.
func EncodeKitty(flags emu.KeyProtocol, e key.Event) ([]byte, bool) {
	return kittykbd.Encode(flags, kittykbd.Event{
		Key:       kittykbd.Key(e.Name),
		Modifiers: kittyModifiers(e.Modifiers),
		Pressed:   e.State == key.Press,
	})
}

func kittyModifiers(m key.Modifiers) kittykbd.Modifiers {
	var out kittykbd.Modifiers
	if m.Contain(key.ModShift) {
		out |= kittykbd.ModShift
	}
	if m.Contain(key.ModAlt) {
		out |= kittykbd.ModAlt
	}
	if m.Contain(key.ModCtrl) {
		out |= kittykbd.ModCtrl
	}
	if m.Contain(key.ModSuper) || m.Contain(key.ModCommand) {
		out |= kittykbd.ModSuper
	}
	return out
}
