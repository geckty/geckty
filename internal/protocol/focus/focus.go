// Package focus encodes window focus in/out events per DECSET 1004, which
// the vendored emu tracks natively (emu.ModeFocus).
package focus

import "github.com/geckty/geckty/internal/vt/emu"

const (
	in  = "\x1b[I"
	out = "\x1b[O"
)

// Encode returns the focus-in ("\x1b[I") or focus-out ("\x1b[O") sequence
// if mode has emu.ModeFocus set, or nil if the shell hasn't asked for
// focus reporting — callers should not write a nil/empty result.
func Encode(mode emu.ModeFlag, gained bool) []byte {
	if mode&emu.ModeFocus == 0 {
		return nil
	}
	if gained {
		return []byte(in)
	}
	return []byte(out)
}
