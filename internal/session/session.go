// Package session bundles a PTY and its VT state machine into one
// per-tab unit, independent of any UI toolkit.
package session

import (
	"fmt"
	"log/slog"
	"strings"
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

	// HistoryLimit caps scrollback physical lines (0 = unlimited).
	HistoryLimit int

	// Clipboard controls OSC 52 policy.
	Clipboard ClipboardPolicy

	// ShellIntegration is forwarded to pty.Config.Integration — see that
	// field's doc comment. Only meaningful when Command is empty.
	ShellIntegration bool

	// Log is optional; nil uses slog.Default().
	Log *slog.Logger

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
	// Dir is the initial working directory at spawn (may be empty).
	Dir string

	writeMu     sync.Mutex
	onDirty     func()
	onExit      func(err error)
	scrollMu    sync.Mutex
	scrollLines int // 0 = live/bottom; positive = scrolled back N lines into history
	selMu       sync.Mutex
	sel         selectionState
	lastClick   clickRecord // guarded by selMu; see RegisterClick
	// selHistOffset tracks Term.HistoryOffset() so prune deltas can shift
	// selection AbsLines (see syncSelectionHistoryOffset).
	selHistOffset int
	// lastPromptJump is the AbsLine of the most recent ScrollToPrompt /
	// SelectLastCommandOutput target (-1 = none yet).
	lastPromptJump int
	osc52          *osc52Bridge
	gfx            *graphics
}

// New spawns a shell per cfg and wires its PTY output into a VT terminal of
// the requested size.
func New(cfg Config) (*Session, error) {
	const op = "session.New"
	cols, rows := cfg.Cols, cfg.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	p, err := pty.Open(pty.Config{
		Command:     cfg.Command,
		Env:         cfg.Env,
		Dir:         cfg.Dir,
		Cols:        uint16(cols),
		Rows:        uint16(rows),
		Integration: cfg.ShellIntegration,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	s := newWithPTY(p, cols, rows, cfg)
	s.Dir = cfg.Dir
	return s, nil
}

// newWithPTY builds a Session around an already-open PTY, letting tests
// substitute a fake pty.PTY instead of spawning a real shell.
func newWithPTY(p pty.PTY, cols, rows int, cfg Config) *Session {
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	s := &Session{
		PTY:            p,
		onDirty:        cfg.OnDirty,
		onExit:         cfg.OnExit,
		lastPromptJump: -1,
		osc52:          newOSC52Bridge(cfg.Clipboard, log.With(slog.String("op", "session.osc52"))),
	}
	s.Term = vt.New(cols, rows, writerFunc(s.Write), s.osc52, cfg.HistoryLimit)
	s.gfx = newGraphics(s)
	return s
}

// newTestSession builds a Session around p for tests. OnExit is left nil —
// tests that need exit handling call SetOnExit after construction.
func newTestSession(p pty.PTY, cols, rows int, onDirty func()) *Session {
	return newWithPTY(p, cols, rows, Config{
		OnDirty:   onDirty,
		Clipboard: ClipboardPolicy{WriteAllow: true, MaxSize: defaultMaxOSC52},
	})
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
			s.syncSelectionHistoryOffset()
			s.Term.RLock()
			altScreen := s.Term.Mode()&emu.ModeAltScreen != 0
			s.Term.RUnlock()
			if altScreen {
				s.ResetScroll()
				s.clearPlacements()
				s.ClearSelection()
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

// ScrollbackText returns History()+Screen() as newline-separated text
// (trailing spaces trimmed per line), for pager / remote-control export.
func (s *Session) ScrollbackText() string {
	s.Term.RLock()
	defer s.Term.RUnlock()

	history := s.Term.History()
	screen := s.Term.Screen()
	cols := s.Term.Size().C
	total := len(history) + len(screen)
	if total == 0 || cols <= 0 {
		return ""
	}
	var b strings.Builder
	for abs := 0; abs < total; abs++ {
		runes, ok := lineRunesAt(history, screen, abs, cols)
		if !ok {
			continue
		}
		b.WriteString(strings.TrimRight(string(runes), " "))
		if abs != total-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
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
