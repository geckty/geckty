package session

import (
	"strings"
	"time"
)

// doubleClickWindow is the maximum gap between two clicks in the same
// cell for RegisterClick to treat them as a double-click.
const doubleClickWindow = 400 * time.Millisecond

// clickRecord is the most recent click RegisterClick observed.
type clickRecord struct {
	at  time.Time
	pos cellPos
}

// cellPos is a (column, row) position in the terminal grid.
type cellPos struct {
	Col, Row int
}

// less reports whether p comes before o in reading order (top-to-bottom,
// left-to-right).
func (p cellPos) less(o cellPos) bool {
	if p.Row != o.Row {
		return p.Row < o.Row
	}
	return p.Col < o.Col
}

// selectionState is the text-selection state for one Session's terminal
// grid. Coordinates are fixed at selection time and not re-validated
// against later terminal output — if the underlying content scrolls or
// changes while a selection exists, the selection can point at stale
// content. This matches plain terminal behavior when not scrolled;
// content-tracking selections are a further refinement, not attempted
// here.
type selectionState struct {
	dragging bool
	has      bool
	// complete is set once the selection represents something worth
	// keeping on release: either a real drag happened (ExtendSelection
	// was called at least once — it's only ever called from a pointer
	// Drag event, which gio only fires on actual pointer movement, so
	// this can't be set by a stationary click) or SelectWord produced
	// it directly. EndSelection uses this — not whether anchor still
	// equals head — to decide whether to drop the selection: a
	// single-character word from a double-click also has anchor==head,
	// but is a deliberate selection, not a plain click's accidental
	// leftover.
	complete bool
	anchor   cellPos // where the drag/click started
	head     cellPos // current/last drag position
}

// CellPos is a (column, row) position in the terminal grid, used by
// Selection's return value.
type CellPos struct {
	Col, Row int
}

// RegisterClick records a press at (col, row) and reports whether it
// forms a double-click with the immediately preceding one — the same
// cell, within doubleClickWindow. Call once per button press, before
// deciding between StartSelection and SelectWord.
//
// Each call updates the tracked "last click" (resetting it on a detected
// double, so a third rapid click in the same cell starts a fresh count
// rather than also registering as a double).
func (s *Session) RegisterClick(col, row int) bool {
	s.selMu.Lock()
	defer s.selMu.Unlock()

	now := time.Now()
	pos := cellPos{col, row}
	isDouble := !s.lastClick.at.IsZero() &&
		now.Sub(s.lastClick.at) <= doubleClickWindow &&
		s.lastClick.pos == pos

	if isDouble {
		s.lastClick = clickRecord{}
	} else {
		s.lastClick = clickRecord{at: now, pos: pos}
	}
	return isDouble
}

// StartSelection begins a new selection at (col, row), replacing any
// existing one. Call on a mouse-button press that isn't being reported to
// the shell (see internal/protocol/mouse.TrackingEnabled).
func (s *Session) StartSelection(col, row int) {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	s.sel = selectionState{dragging: true, has: true, anchor: cellPos{col, row}, head: cellPos{col, row}}
}

// ExtendSelection updates the selection's current endpoint. Call on a
// pointer drag while a selection is in progress.
func (s *Session) ExtendSelection(col, row int) {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	if !s.sel.has {
		return
	}
	s.sel.head = cellPos{col, row}
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

// SelectWord replaces the current selection with the word touching (col,
// row) — the terminal-standard double-click gesture. "Word" means a run
// of letters, digits, "_", and whatever extra characters wordChars lists
// (see config.SelectionConfig.WordChars); everything else, including a
// blank cell, is a boundary. If (col, row) itself is on a boundary (e.g.
// double-clicking whitespace), the resulting selection is just that one
// cell — there's no word there to expand into.
func (s *Session) SelectWord(col, row int, wordChars string) {
	s.Term.RLock()
	sz := s.Term.Size()
	if row < 0 || row >= sz.R {
		s.Term.RUnlock()
		return
	}
	line := make([]rune, sz.C)
	for c := 0; c < sz.C; c++ {
		ch := s.Term.Cell(c, row).Char
		if ch == 0 {
			ch = ' '
		}
		line[c] = ch
	}
	s.Term.RUnlock()

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
	if isWordChar(line[col], wordChars) {
		for start > 0 && isWordChar(line[start-1], wordChars) {
			start--
		}
		for end < sz.C-1 && isWordChar(line[end+1], wordChars) {
			end++
		}
	}

	s.selMu.Lock()
	s.sel = selectionState{has: true, complete: true, anchor: cellPos{start, row}, head: cellPos{end, row}}
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
// selection.
func (s *Session) Selection() (start, end CellPos, ok bool) {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	if !s.sel.has {
		return CellPos{}, CellPos{}, false
	}
	a, h := s.sel.anchor, s.sel.head
	if h.less(a) {
		a, h = h, a
	}
	return CellPos(a), CellPos(h), true
}

// SelectedText extracts the text within the current selection from the
// live terminal grid. ok is false if there is no selection.
func (s *Session) SelectedText() (text string, ok bool) {
	start, end, has := s.Selection()
	if !has {
		return "", false
	}

	s.Term.RLock()
	defer s.Term.RUnlock()

	sz := s.Term.Size()
	var b strings.Builder
	for row := start.Row; row <= end.Row; row++ {
		if row < 0 || row >= sz.R {
			continue
		}
		colStart, colEnd := 0, sz.C-1
		if row == start.Row {
			colStart = start.Col
		}
		if row == end.Row {
			colEnd = end.Col
		}
		var line strings.Builder
		for col := colStart; col <= colEnd && col < sz.C; col++ {
			c := s.Term.Cell(col, row).Char
			if c == 0 {
				c = ' '
			}
			line.WriteRune(c)
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		if row != end.Row {
			b.WriteByte('\n')
		}
	}
	return b.String(), true
}
