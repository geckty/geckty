// Package paste encodes pasted text per bracketed paste mode (DECSET
// 2004), which the vendored emu tracks natively (emu.ModeBracketedPaste).
package paste

import "github.com/geckty/geckty/internal/vt/emu"

const (
	start = "\x1b[200~"
	end   = "\x1b[201~"
)

// Encode wraps text in bracketed-paste markers if mode has
// emu.ModeBracketedPaste set, so the receiving program can distinguish
// pasted text from typed input (and avoid, e.g., auto-indent mangling a
// multi-line paste). Returns text unmodified otherwise.
//
// Not sanitized: clipboard content containing a literal "\x1b[201~" would
// terminate the bracketed region early from the receiving program's
// perspective, after which the rest of the pasted text is read as regular
// keystrokes (a known bracketed-paste injection vector if the trailing
// content includes a newline). xterm and kitty don't sanitize this either
// — the convention is the paste-end marker only needs to be well-formed
// for legitimate clipboard content, not treated as an adversarial input
// boundary — but it's worth knowing about.
func Encode(mode emu.ModeFlag, text string) []byte {
	if mode&emu.ModeBracketedPaste == 0 {
		return []byte(text)
	}
	out := make([]byte, 0, len(start)+len(text)+len(end))
	out = append(out, start...)
	out = append(out, text...)
	out = append(out, end...)
	return out
}
