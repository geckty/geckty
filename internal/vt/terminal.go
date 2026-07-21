// Package vt wraps the vendored internal/vt/emu (github.com/cfoust/cy's
// pkg/emu, see internal/vt/emu/NOTICE.md), the VT/ANSI state machine
// geckty uses as its terminal grid core.
package vt

import (
	"io"
	"sync"

	"github.com/geckty/geckty/internal/vt/emu"
	"github.com/geckty/geckty/internal/vt/emu/geom"
)

// Terminal is a VT100+ state machine bound to a fixed-size cell grid.
//
// emu.Terminal's own View accessors (Cell, Cursor, Size, History, ...) do
// not synchronize with Parse themselves — the concrete *emu.State has a
// Lock/Unlock pair for exactly this, but they aren't part of the
// emu.Terminal interface this type embeds, so they aren't reachable
// through it. Terminal adds its own mutex instead: Parse and ResizeCells
// take a write lock, and RLock/RUnlock let a reader (the UI painter) hold
// a consistent snapshot across a whole frame's worth of Cell/Cursor/Size/
// History calls, rather than each accessor racing independently against
// the PTY read loop's concurrent Parse calls.
type Terminal struct {
	emu.Terminal
	mu sync.RWMutex
}

// New creates a Terminal of the given size. Bytes written to w are the
// terminal's own replies (e.g. DA/DSR responses); the caller is expected to
// route w back to the originating PTY. osc52 may be nil, in which case OSC
// 52 clipboard sequences are parsed (so they can't corrupt grid state) but
// otherwise ignored — see emu.OSC52Handler.
func New(cols, rows int, w io.Writer, osc52 emu.OSC52Handler) *Terminal {
	return &Terminal{
		Terminal: emu.New(
			emu.WithSize(geom.Vec2{C: cols, R: rows}),
			emu.WithWriter(w),
			emu.WithOSC52Handler(osc52),
		),
	}
}

// Parse feeds shell output into the terminal, updating its grid state.
// Shadows the embedded emu.Terminal.Parse to add the write-lock side of
// Terminal's own synchronization (see the type doc comment).
func (t *Terminal) Parse(p []byte) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Terminal.Parse(p)
}

// ResizeCells changes the terminal's column/row count.
//
// Named distinctly from the embedded emu.Terminal.Resize(geom.Vec2) rather
// than overriding it: a same-named method with a different signature would
// shadow the embedded one entirely, so *Terminal would stop satisfying
// emu.View (whose Resize(geom.Vec2) is part of the interface).
func (t *Terminal) ResizeCells(cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Resize(geom.Vec2{C: cols, R: rows})
}

// RLock starts a read pass (e.g. one rendered frame) that observes a
// consistent snapshot instead of racing against a concurrent Parse or
// ResizeCells call. Always paired with a matching RUnlock.
func (t *Terminal) RLock() { t.mu.RLock() }

// RUnlock ends a read pass started by RLock.
func (t *Terminal) RUnlock() { t.mu.RUnlock() }
