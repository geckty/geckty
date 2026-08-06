package session

import (
	"strings"
	"unicode"
)

// SearchHit is one case-insensitive match of a query in History()+Screen().
type SearchHit struct {
	AbsLine int
	Col     int // starting column
	Len     int // rune length of the match (== len([]rune(query)) for non-empty query)
}

// FindInScrollback searches History()+Screen() for query (case-insensitive).
// When forward is true, the first hit at-or-after (afterAbs, afterCol) wins;
// otherwise the last hit strictly before that cursor. Empty query yields ok=false.
func (s *Session) FindInScrollback(query string, afterAbs, afterCol int, forward bool) (hit SearchHit, ok bool) {
	q := []rune(strings.ToLower(query))
	if len(q) == 0 {
		return SearchHit{}, false
	}

	s.Term.RLock()
	defer s.Term.RUnlock()

	history := s.Term.History()
	screen := s.Term.Screen()
	cols := s.Term.Size().C
	total := len(history) + len(screen)
	if total == 0 || cols <= 0 {
		return SearchHit{}, false
	}

	if forward {
		startLine := afterAbs
		if startLine < 0 {
			startLine = 0
		}
		for abs := startLine; abs < total; abs++ {
			runes, lineOK := lineRunesAt(history, screen, abs, cols)
			if !lineOK {
				continue
			}
			colStart := 0
			if abs == afterAbs {
				colStart = afterCol
			}
			if col, found := indexRunesFold(runes, q, colStart); found {
				return SearchHit{AbsLine: abs, Col: col, Len: len(q)}, true
			}
		}
		return SearchHit{}, false
	}

	endLine := afterAbs
	if endLine >= total {
		endLine = total - 1
	}
	for abs := endLine; abs >= 0; abs-- {
		runes, lineOK := lineRunesAt(history, screen, abs, cols)
		if !lineOK {
			continue
		}
		colLimit := cols
		if abs == afterAbs {
			colLimit = afterCol
		}
		if col, found := lastIndexRunesFold(runes, q, colLimit); found {
			return SearchHit{AbsLine: abs, Col: col, Len: len(q)}, true
		}
	}
	return SearchHit{}, false
}

// CountInScrollback returns how many non-overlapping case-insensitive
// matches of query exist in History()+Screen().
func (s *Session) CountInScrollback(query string) int {
	q := []rune(strings.ToLower(query))
	if len(q) == 0 {
		return 0
	}
	s.Term.RLock()
	defer s.Term.RUnlock()

	history := s.Term.History()
	screen := s.Term.Screen()
	cols := s.Term.Size().C
	total := len(history) + len(screen)
	n := 0
	for abs := 0; abs < total; abs++ {
		runes, ok := lineRunesAt(history, screen, abs, cols)
		if !ok {
			continue
		}
		col := 0
		for {
			i, found := indexRunesFold(runes, q, col)
			if !found {
				break
			}
			n++
			col = i + len(q)
		}
	}
	return n
}

// ScrollToAbsLine adjusts scrollOffset so absLine is visible near the
// middle of the viewport when possible. Returns the resulting offset.
func (s *Session) ScrollToAbsLine(absLine int) int {
	s.Term.RLock()
	histLen := len(s.Term.History())
	screenLen := len(s.Term.Screen())
	rows := s.Term.Size().R
	s.Term.RUnlock()
	if rows <= 0 {
		return s.ScrollOffset()
	}
	total := histLen + screenLen
	top := absLine - rows/2
	if top < 0 {
		top = 0
	}
	maxTop := total - rows
	if maxTop < 0 {
		maxTop = 0
	}
	if top > maxTop {
		top = maxTop
	}
	// scrollOffset = total - rows - top (0 = live bottom).
	want := total - rows - top
	if want < 0 {
		want = 0
	}
	maxOff := histLen
	if want > maxOff {
		want = maxOff
	}
	s.scrollMu.Lock()
	s.scrollLines = want
	off := s.scrollLines
	s.scrollMu.Unlock()
	return off
}

func indexRunesFold(line, query []rune, fromCol int) (col int, ok bool) {
	if fromCol < 0 {
		fromCol = 0
	}
	if len(query) == 0 || fromCol > len(line)-len(query) {
		return 0, false
	}
	for i := fromCol; i <= len(line)-len(query); i++ {
		if runesEqualFold(line[i:i+len(query)], query) {
			return i, true
		}
	}
	return 0, false
}

func lastIndexRunesFold(line, query []rune, beforeCol int) (col int, ok bool) {
	if len(query) == 0 {
		return 0, false
	}
	maxStart := len(line) - len(query)
	if beforeCol-len(query) < maxStart {
		maxStart = beforeCol - len(query)
	}
	if maxStart < 0 {
		return 0, false
	}
	for i := maxStart; i >= 0; i-- {
		if runesEqualFold(line[i:i+len(query)], query) {
			return i, true
		}
	}
	return 0, false
}

func runesEqualFold(a, needleLower []rune) bool {
	if len(a) != len(needleLower) {
		return false
	}
	for i := range a {
		if unicode.ToLower(a[i]) != needleLower[i] {
			return false
		}
	}
	return true
}
