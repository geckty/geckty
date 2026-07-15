// Package session bundles a PTY and its VT state machine into one
// per-tab unit, independent of any UI toolkit.
package session

import (
	"sync"

	"github.com/geckty/geckty/internal/pty"
	"github.com/geckty/geckty/internal/vt"
	"github.com/geckty/geckty/internal/vt/emu"
)

// Config describes how to start a Session.
type Config struct {
	Command    []string
	Env        []string
	Dir        string
	Cols, Rows int

	// OnDirty is called (from the read goroutine) whenever new terminal
	// output may require a repaint. It must not block.
	OnDirty func()

	// OnExit is called once, when the shell process exits or the PTY
	// read loop otherwise ends.
	OnExit func(err error)
}

// Session is one shell instance: a PTY and the VT state it drives.
//
// All writes back to the shell (keystrokes, mouse reports, focus events,
// pasted text, protocol responses, and the VT engine's own replies) go
// through Write, which is the single serialization point guarding
// concurrent writers from interleaving on the PTY.
type Session struct {
	PTY  pty.PTY
	Term *vt.Terminal

	writeMu     sync.Mutex
	onDirty     func()
	onExit      func(err error)
	scrollMu    sync.Mutex
	scrollLines int // 0 = live/bottom; positive = scrolled back N lines into history
	selMu       sync.Mutex
	sel         selectionState
	lastClick   clickRecord // guarded by selMu; see RegisterClick
	osc52       *osc52Bridge
	gfx         *graphics
}

// New spawns a shell per cfg and wires its PTY output into a VT terminal of
// the requested size.
func New(cfg Config) (*Session, error) {
	cols, rows := cfg.Cols, cfg.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	p, err := pty.Open(pty.Config{
		Command: cfg.Command,
		Env:     cfg.Env,
		Dir:     cfg.Dir,
		Cols:    uint16(cols),
		Rows:    uint16(rows),
	})
	if err != nil {
		return nil, err
	}

	return newWithPTY(p, cols, rows, cfg.OnDirty, cfg.OnExit), nil
}

// newWithPTY builds a Session around an already-open PTY, letting tests
// substitute a fake pty.PTY instead of spawning a real shell.
func newWithPTY(p pty.PTY, cols, rows int, onDirty func(), onExit func(error)) *Session {
	s := &Session{
		PTY:     p,
		onDirty: onDirty,
		onExit:  onExit,
		osc52:   newOSC52Bridge(),
	}
	// emu's own replies (DA/DSR responses, etc.) are written via
	// s.Write, the same guarded path input/protocol encoders use, so
	// they can never interleave mid-sequence with a keystroke or paste.
	s.Term = vt.New(cols, rows, writerFunc(s.Write), s.osc52)
	s.gfx = newGraphics(s)
	return s
}

// SetOnExit replaces the exit callback. Must be called before Run starts
// (i.e. synchronously, right after construction) — Run only reads onExit
// after its read loop ends, so there's no race as long as SetOnExit isn't
// called concurrently with an already-running Run. session.Manager uses
// this to wire tab auto-removal in after a session exists but before its
// read loop starts, since the callback needs the tab id Manager only
// assigns once New has returned.
func (s *Session) SetOnExit(f func(error)) {
	s.onExit = f
}

// Run reads PTY output and feeds it to the VT terminal until the PTY closes
// or the process exits. Call it in its own goroutine; it blocks.
func (s *Session) Run() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.PTY.Read(buf)
		if n > 0 {
			s.Term.Parse(buf[:n])
			// Fed the identical bytes in parallel, unfiltered, for
			// Kitty-graphics APC sequences emu doesn't recognize
			// at all (see internal/protocol.Sniffer's doc
			// comment). s.handleAPC (internal/session/graphics.go)
			// may itself call s.Write for a protocol response, so
			// this must not hold any lock Write could contend on.
			_, _ = s.gfx.sniffer.Write(buf[:n])
			// Full-screen apps (vim, less, htop) manage their own
			// scrolling and don't expect the terminal to be
			// showing scrollback underneath them — drop back to
			// live view whenever alt screen engages. The same
			// applies to Kitty-graphics placements from the main
			// screen.
			s.Term.RLock()
			altScreen := s.Term.Mode()&emu.ModeAltScreen != 0
			s.Term.RUnlock()
			if altScreen {
				s.ResetScroll()
				s.clearPlacements()
			}
			if s.onDirty != nil {
				s.onDirty()
			}
		}
		if err != nil {
			if s.onExit != nil {
				s.onExit(err)
			}
			return
		}
	}
}

// ScrollBy adjusts the scrollback offset by delta lines (positive scrolls
// back into history, negative scrolls toward the live bottom), clamped to
// [0, len(history)]. It returns the resulting offset.
func (s *Session) ScrollBy(delta int) int {
	s.Term.RLock()
	maxOffset := len(s.Term.History())
	s.Term.RUnlock()

	s.scrollMu.Lock()
	defer s.scrollMu.Unlock()
	s.scrollLines += delta
	if s.scrollLines < 0 {
		s.scrollLines = 0
	}
	if s.scrollLines > maxOffset {
		s.scrollLines = maxOffset
	}
	return s.scrollLines
}

// ScrollOffset returns the current scrollback offset (0 = live/bottom).
func (s *Session) ScrollOffset() int {
	s.scrollMu.Lock()
	defer s.scrollMu.Unlock()
	return s.scrollLines
}

// ResetScroll returns to the live/bottom view.
func (s *Session) ResetScroll() {
	s.scrollMu.Lock()
	s.scrollLines = 0
	s.scrollMu.Unlock()
}

// Write sends p to the shell. Safe for concurrent use — every writer
// (keystrokes, mouse/focus/paste encoders, protocol responses, and emu's
// own replies) goes through this one guarded path.
func (s *Session) Write(p []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.PTY.Write(p)
}

// Resize changes both the PTY's and the VT terminal's size.
func (s *Session) Resize(cols, rows int) error {
	if err := s.PTY.Resize(uint16(cols), uint16(rows)); err != nil {
		return err
	}
	s.Term.ResizeCells(cols, rows)
	// Re-clamp against the (possibly now different) history length —
	// ResizeCells can change how many physical lines exist via rewrap.
	s.ScrollBy(0)
	return nil
}

// Close terminates the shell and releases the PTY.
func (s *Session) Close() error {
	return s.PTY.Close()
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
