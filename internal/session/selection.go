package session

import (
	"strings"
	"time"

	"github.com/geckty/geckty/internal/vt/emu"
)

// doubleClickWindow is the maximum gap between two clicks in the same
// cell for RegisterClick to treat them as a double-click.
const doubleClickWindow = 400 * time.Millisecond

// clickRecord is the most recent click RegisterClick observed.
type clickRecord struct {
	at    time.Time
	pos   cellPos
	count int // consecutive clicks in the same cell within doubleClickWindow
}

// cellPos is a (column, absolute-line) position in History()+Screen().
// AbsLine 0 is the oldest retained history line; the live screen starts at
// len(History()). Coordinates stay valid across viewport scroll; when emu
// prunes history, Session.syncSelectionHistoryOffset shifts them.
type cellPos struct {
	Col, AbsLine int
}

// less reports whether p comes before o in reading order (top-to-bottom,
// left-to-right).
func (p cellPos) less(o cellPos) bool {
	if p.AbsLine != o.AbsLine {
		return p.AbsLine < o.AbsLine
	}
	return p.Col < o.Col
}

// selectionState is the text-selection state for one Session. Bounds use
// absolute History()+Screen() line indices so a selection survives
// scrollback navigation and can be copied after the highlighted region
// leaves the viewport.
type selectionState struct {
	dragging bool
	has      bool
	// complete is set once the selection represents something worth
	// keeping on release: either a real drag happened (ExtendSelection
	// was called at least once — it's only ever called from a
	// gpucontext.PointerMove event, which only fires on actual pointer
	// movement, so this can't be set by a stationary click) or SelectWord
	// produced
	// it directly. EndSelection uses this — not whether anchor still
	// equals head — to decide whether to drop the selection: a
	// single-character word from a double-click also has anchor==head,
	// but is a deliberate selection, not a plain click's accidental
	// leftover.
	complete bool
	rect     bool    // Alt-drag rectangular column span
	anchor   cellPos // where the drag/click started
	head     cellPos // current/last drag position
}

// CellPos is a (column, absolute-line) position in History()+Screen(),
// used by Selection's return value.
type CellPos struct {
	Col, AbsLine int
}

// RegisterClick records a press at (col, absLine) and returns the
// consecutive click count in that cell within doubleClickWindow: 1 =
// single (start drag), 2 = double (SelectWord), 3 = triple (SelectLine).
// A fourth rapid click restarts at 1.
func (s *Session) RegisterClick(col, absLine int) int {
	s.selMu.Lock()
	defer s.selMu.Unlock()

	now := time.Now()
	pos := cellPos{col, absLine}
	count := 1
	if !s.lastClick.at.IsZero() &&
		now.Sub(s.lastClick.at) <= doubleClickWindow &&
		s.lastClick.pos == pos {
		count = s.lastClick.count + 1
		if count > 3 {
			count = 1
		}
	}
	s.lastClick = clickRecord{at: now, pos: pos, count: count}
	return count
}

// StartSelection begins a new selection at (col, absLine), replacing any
// existing one. Call on a mouse-button press that isn't being reported to
// the shell (see internal/protocol/mouse.TrackingEnabled). absLine is an
// index into History()+Screen() (see ViewToAbsLine). When rect is true the
// selection is a column rectangle (Alt-drag) rather than a stream range.
func (s *Session) StartSelection(col, absLine int) {
	s.StartSelectionMode(col, absLine, false)
}

// StartSelectionMode is StartSelection with an explicit rectangular mode.
func (s *Session) StartSelectionMode(col, absLine int, rect bool) {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	s.sel = selectionState{dragging: true, has: true, rect: rect, anchor: cellPos{col, absLine}, head: cellPos{col, absLine}}
}

// ExtendSelection updates the selection's current endpoint. Call on a
// pointer drag while a selection is in progress.
func (s *Session) ExtendSelection(col, absLine int) {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	if !s.sel.has {
		return
	}
	s.sel.head = cellPos{col, absLine}
	s.sel.complete = true
}

// EndSelection marks the drag gesture finished. If the selection was
// never actually completed — StartSelection ran but neither a real drag
// (ExtendSelection) nor a word-select ever followed, i.e. a plain click —
// it's dropped rather than left in place as a lingering single-cell
// highlight: a click alone isn't a meaningful text selection in any
// terminal or editor, and leaving one behind was a real, reported bug
// (every plain click left a 1-character selection nothing ever cleared).
// A completed selection (real drag, or SelectWord) is left in place until
// ClearSelection or a new StartSelection, as before.
func (s *Session) EndSelection() {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	if !s.sel.complete {
		s.sel = selectionState{}
		return
	}
	s.sel.dragging = false
}

// SelectWord replaces the current selection with the word touching
// (col, absLine) — the terminal-standard double-click gesture. "Word"
// means a run of letters, digits, "_", and whatever extra characters
// wordChars lists (see config.SelectionConfig.WordChars); everything else,
// including a blank cell, is a boundary. If (col, absLine) itself is on a
// boundary (e.g. double-clicking whitespace), the resulting selection is
// just that one cell — there's no word there to expand into.
func (s *Session) SelectWord(col, absLine int, wordChars string) {
	s.Term.RLock()
	sz := s.Term.Size()
	lineRunes, ok := lineRunesAt(s.Term.History(), s.Term.Screen(), absLine, sz.C)
	s.Term.RUnlock()
	if !ok {
		return
	}

	if col < 0 {
		col = 0
	}
	if col >= sz.C {
		col = sz.C - 1
	}
	if sz.C == 0 {
		return
	}

	start, end := col, col
	if isWordChar(lineRunes[col], wordChars) {
		for start > 0 && isWordChar(lineRunes[start-1], wordChars) {
			start--
		}
		for end < sz.C-1 && isWordChar(lineRunes[end+1], wordChars) {
			end++
		}
	}

	s.selMu.Lock()
	s.sel = selectionState{has: true, complete: true, anchor: cellPos{start, absLine}, head: cellPos{end, absLine}}
	s.selMu.Unlock()
}

// SelectLine replaces the current selection with the full terminal line at
// absLine — the terminal-standard triple-click gesture.
func (s *Session) SelectLine(absLine int) {
	s.Term.RLock()
	cols := s.Term.Size().C
	s.Term.RUnlock()
	if cols <= 0 {
		return
	}
	s.selMu.Lock()
	s.sel = selectionState{
		has: true, complete: true,
		anchor: cellPos{0, absLine},
		head:   cellPos{cols - 1, absLine},
	}
	s.selMu.Unlock()
}

// isWordChar reports whether r counts as part of a word for SelectWord:
// letters, digits, "_", or any rune in extra.
func isWordChar(r rune, extra string) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		return true
	}
	return strings.ContainsRune(extra, r)
}

// ClearSelection removes the current selection, if any.
func (s *Session) ClearSelection() {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	s.sel = selectionState{}
}

// Selection returns the current selection's normalized bounds (start
// always at-or-before end in reading order). ok is false if there is no
// selection. AbsLine is an index into History()+Screen().
// For rectangular selections, Start.Col/End.Col are the left/right column
// span (not stream endpoints); use SelectionRect to detect that mode.
func (s *Session) Selection() (start, end CellPos, ok bool) {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	if !s.sel.has {
		return CellPos{}, CellPos{}, false
	}
	a, h := s.sel.anchor, s.sel.head
	if s.sel.rect {
		c0, c1 := a.Col, h.Col
		if c1 < c0 {
			c0, c1 = c1, c0
		}
		l0, l1 := a.AbsLine, h.AbsLine
		if l1 < l0 {
			l0, l1 = l1, l0
		}
		return CellPos{Col: c0, AbsLine: l0}, CellPos{Col: c1, AbsLine: l1}, true
	}
	if h.less(a) {
		a, h = h, a
	}
	return CellPos(a), CellPos(h), true
}

// SelectionRect reports whether the current selection is rectangular.
func (s *Session) SelectionRect() bool {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	return s.sel.has && s.sel.rect
}

// SelectedText extracts the text within the current selection from
// History()+Screen(). ok is false if there is no selection.
func (s *Session) SelectedText() (text string, ok bool) {
	s.selMu.Lock()
	if !s.sel.has {
		s.selMu.Unlock()
		return "", false
	}
	rect := s.sel.rect
	a, h := s.sel.anchor, s.sel.head
	s.selMu.Unlock()

	var start, end CellPos
	if rect {
		c0, c1 := a.Col, h.Col
		if c1 < c0 {
			c0, c1 = c1, c0
		}
		l0, l1 := a.AbsLine, h.AbsLine
		if l1 < l0 {
			l0, l1 = l1, l0
		}
		start, end = CellPos{Col: c0, AbsLine: l0}, CellPos{Col: c1, AbsLine: l1}
	} else {
		if h.less(a) {
			a, h = h, a
		}
		start, end = CellPos(a), CellPos(h)
	}

	s.Term.RLock()
	defer s.Term.RUnlock()

	history := s.Term.History()
	screen := s.Term.Screen()
	cols := s.Term.Size().C
	var b strings.Builder
	for abs := start.AbsLine; abs <= end.AbsLine; abs++ {
		colStart, colEnd := 0, cols-1
		if rect {
			colStart, colEnd = start.Col, end.Col
		} else {
			if abs == start.AbsLine {
				colStart = start.Col
			}
			if abs == end.AbsLine {
				colEnd = end.Col
			}
		}
		runes, lineOK := lineRunesAt(history, screen, abs, cols)
		if !lineOK {
			continue
		}
		var line strings.Builder
		for col := colStart; col <= colEnd && col < cols; col++ {
			line.WriteRune(runes[col])
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		if abs != end.AbsLine {
			b.WriteByte('\n')
		}
	}
	return b.String(), true
}

// ViewToAbsLine maps a viewport row (0 = top visible row) to an absolute
// History()+Screen() line index given the session's current scroll offset.
func (s *Session) ViewToAbsLine(viewRow int) int {
	offset := s.ScrollOffset()
	s.Term.RLock()
	defer s.Term.RUnlock()
	return viewToAbsLine(len(s.Term.History()), len(s.Term.Screen()), s.Term.Size().R, offset, viewRow)
}

// ViewportTopAbsLine returns the absolute line index of the first visible
// row for the current scroll offset (same addressing as ViewToAbsLine).
func (s *Session) ViewportTopAbsLine() int {
	offset := s.ScrollOffset()
	s.Term.RLock()
	defer s.Term.RUnlock()
	return viewportTop(len(s.Term.History()), len(s.Term.Screen()), s.Term.Size().R, offset)
}

// SelectionDragging reports whether a drag selection is in progress.
func (s *Session) SelectionDragging() bool {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	return s.sel.dragging && s.sel.has
}

// syncSelectionHistoryOffset shifts selection AbsLines when emu prunes
// scrollback from the front, and clears the selection if both ends were
// dropped. Call after Term.Parse.
func (s *Session) syncSelectionHistoryOffset() {
	s.Term.RLock()
	off := s.Term.HistoryOffset()
	s.Term.RUnlock()

	s.selMu.Lock()
	defer s.selMu.Unlock()
	if off > s.selHistOffset {
		drop := off - s.selHistOffset
		if s.sel.has {
			s.sel.anchor.AbsLine -= drop
			s.sel.head.AbsLine -= drop
			if s.sel.anchor.AbsLine < 0 && s.sel.head.AbsLine < 0 {
				s.sel = selectionState{}
			} else {
				if s.sel.anchor.AbsLine < 0 {
					s.sel.anchor = cellPos{}
				}
				if s.sel.head.AbsLine < 0 {
					s.sel.head = cellPos{}
				}
			}
		}
	}
	s.selHistOffset = off
}

func viewToAbsLine(histLen, screenLen, rows, scrollOffset, viewRow int) int {
	return viewportTop(histLen, screenLen, rows, scrollOffset) + viewRow
}

func viewportTop(histLen, screenLen, rows, scrollOffset int) int {
	if scrollOffset <= 0 {
		return histLen
	}
	total := histLen + screenLen
	top := total - rows - scrollOffset
	if top < 0 {
		return 0
	}
	return top
}

func lineRunesAt(history, screen []emu.Line, absLine, cols int) ([]rune, bool) {
	var line emu.Line
	switch {
	case absLine < 0:
		return nil, false
	case absLine < len(history):
		line = history[absLine]
	case absLine < len(history)+len(screen):
		line = screen[absLine-len(history)]
	default:
		return nil, false
	}
	out := make([]rune, cols)
	for c := 0; c < cols; c++ {
		ch := rune(' ')
		if c < len(line) {
			ch = line[c].Char
			if ch == 0 {
				ch = ' '
			}
		}
		out[c] = ch
	}
	return out, true
}
