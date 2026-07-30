// Package vt wraps the vendored internal/vt/emu (github.com/cfoust/cy's
// pkg/emu, see internal/vt/emu/NOTICE.md), the VT/ANSI state machine
// geckty uses as its terminal grid core.
package vt

import (
	"io"
	"sync"
	"time"

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

	// cmd is the current OSC 133 shell-integration command-run state (see
	// CommandState), folded in by Parse from emu's per-write event log
	// each call — see foldSemanticPrompts.
	cmd CommandState
}

// CommandState summarizes OSC 133 shell-integration state for a UI to
// render a "command running" indicator against — e.g. the active tab's
// pill in the tab bar (see internal/ui/gogpu/tabbar.go's paintTab).
//
// Deliberately doesn't track *which row* the command started on: OSC 133
// positions are screen-relative at the moment they're received, and would
// need continuous adjustment as output scrolls the screen to stay
// meaningful — a "mark" durable across scrollback is a real terminal
// feature (iTerm2, WezTerm) but needs metadata attached to history lines
// in the vendored emu package (internal/vt/emu/NOTICE.md's "no behavioral
// changes" policy for that vendored copy), not something to bolt on here.
type CommandState struct {
	// Running is true from OSC 133;C (command executed) until the
	// matching OSC 133;D (command finished).
	Running bool
	// ExitCode is the most recently finished command's exit code, or nil
	// if none has finished yet in this session or the code wasn't sent.
	// Retained across the next PromptStart/CommandStart so a UI can show
	// "last command failed" at the following prompt, and cleared once the
	// next command starts running.
	ExitCode *int
	// FinishedAt is when ExitCode was last set — a UI showing a brief
	// success/failure flash (rather than a permanent badge every tab
	// accumulates forever) compares this against time.Now() itself; see
	// CommandIndicatorFade in internal/ui/gogpu/tabbar.go.
	FinishedAt time.Time
}

// New creates a Terminal of the given size. Bytes written to w are the
// terminal's own replies (e.g. DA/DSR responses); the caller is expected to
// route w back to the originating PTY. osc52 may be nil, in which case OSC
// 52 clipboard sequences are parsed (so they can't corrupt grid state) but
// otherwise ignored — see emu.OSC52Handler. historyLimit caps scrollback
// physical lines (0 = unlimited).
func New(cols, rows int, w io.Writer, osc52 emu.OSC52Handler, historyLimit int) *Terminal {
	opts := []emu.TerminalOption{
		emu.WithSize(geom.Vec2{C: cols, R: rows}),
		emu.WithWriter(w),
		emu.WithOSC52Handler(osc52),
	}
	if historyLimit > 0 {
		opts = append(opts, emu.WithHistoryLimit(historyLimit))
	}
	return &Terminal{
		Terminal: emu.New(opts...),
	}
}

// Parse feeds shell output into the terminal, updating its grid state.
// Shadows the embedded emu.Terminal.Parse to add the write-lock side of
// Terminal's own synchronization (see the type doc comment), and to fold
// any OSC 133 events this call produced into cmd (see foldSemanticPrompts).
func (t *Terminal) Parse(p []byte) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := t.Terminal.Parse(p)
	t.foldSemanticPrompts()
	return n
}

// semanticPromptSource is satisfied by *emu.State (emu.New's concrete
// return type) — Changes() isn't part of the emu.Terminal interface
// Terminal embeds, so it's only reachable via this narrow, local
// assertion rather than widening that interface for one caller.
type semanticPromptSource interface {
	Changes() *emu.Dirty
}

// foldSemanticPrompts drains emu's per-write OSC 133 event log (nothing
// else in geckty currently consumes emu.Dirty, so this is also the only
// place that needs to clear it — see emu.Dirty.Reset's doc comment) into
// cmd's durable Running/ExitCode state. Called with t.mu held.
func (t *Terminal) foldSemanticPrompts() {
	src, ok := t.Terminal.(semanticPromptSource)
	if !ok {
		return
	}
	dirty := src.Changes()
	events := dirty.GetSemanticPrompts()
	if len(events) == 0 {
		return
	}
	for _, ev := range events {
		switch ev.Type {
		case emu.CommandExecuted:
			t.cmd.Running = true
			t.cmd.ExitCode = nil
		case emu.CommandFinished:
			t.cmd.Running = false
			t.cmd.ExitCode = ev.ExitCode
			t.cmd.FinishedAt = time.Now()
		}
	}
	dirty.SemanticPrompts = dirty.SemanticPrompts[:0]
}

// CommandState returns the terminal's current OSC 133 command-run state
// (see CommandState's doc comment). Callers needing a consistent snapshot
// alongside other Term state should call this within an RLock/RUnlock
// pair, same as Cell/Cursor/Size/History.
func (t *Terminal) CommandState() CommandState {
	return t.cmd
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
