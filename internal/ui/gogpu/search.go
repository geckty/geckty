package gogpu

import (
	"fmt"
	"image/color"
	"time"
	"unicode/utf8"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/session"
)

// searchState is the in-window scrollback find UI (Kitty-style overlay).
type searchState struct {
	active  bool
	query   string
	hit     session.SearchHit
	hasHit  bool
	count   int
	status  string // e.g. "no matches"
}

func (s *uiState) openSearch() {
	s.search = searchState{active: true}
	s.app.RequestRedraw()
}

func (s *uiState) closeSearch() {
	s.search = searchState{}
	s.app.RequestRedraw()
}

func (s *uiState) searchActive() bool {
	return s.search.active
}

// handleSearchKey consumes keys while the find overlay is open. Returns
// true when the key was handled (must not reach the shell).
func (s *uiState) handleSearchKey(key gpucontext.Key, mods gpucontext.Modifiers) bool {
	if !s.search.active {
		return false
	}
	switch key {
	case gpucontext.KeyEscape:
		s.closeSearch()
		return true
	case gpucontext.KeyEnter:
		s.searchStep(!mods.HasShift())
		return true
	case gpucontext.KeyN:
		if mods.HasControl() || mods.HasSuper() {
			return false
		}
		s.searchStep(!mods.HasShift())
		return true
	case gpucontext.KeyBackspace:
		if s.search.query == "" {
			return true
		}
		_, size := utf8.DecodeLastRuneInString(s.search.query)
		s.search.query = s.search.query[:len(s.search.query)-size]
		s.searchRefresh(true)
		return true
	}
	return false
}

// handleSearchText appends printable text to the find query.
func (s *uiState) handleSearchText(text string) bool {
	if !s.search.active || text == "" {
		return false
	}
	for _, r := range text {
		if r < 32 {
			continue
		}
		s.search.query += string(r)
	}
	s.searchRefresh(true)
	return true
}

func (s *uiState) searchRefresh(fromStart bool) {
	active := s.mgr.Active()
	if active == nil {
		return
	}
	s.search.status = ""
	s.search.count = active.CountInScrollback(s.search.query)
	if s.search.query == "" {
		s.search.hasHit = false
		s.search.hit = session.SearchHit{}
		s.app.RequestRedraw()
		return
	}
	afterAbs, afterCol := 0, 0
	if !fromStart && s.search.hasHit {
		afterAbs = s.search.hit.AbsLine
		afterCol = s.search.hit.Col + 1
	}
	hit, ok := active.FindInScrollback(s.search.query, afterAbs, afterCol, true)
	if !ok && !fromStart {
		// Wrap to first match.
		hit, ok = active.FindInScrollback(s.search.query, 0, 0, true)
	}
	s.search.hasHit = ok
	s.search.hit = hit
	if !ok {
		s.search.status = "no matches"
	} else {
		active.ScrollToAbsLine(hit.AbsLine)
		s.scrollBarUntil = time.Now().Add(1200 * time.Millisecond)
	}
	s.app.RequestRedraw()
}

func (s *uiState) searchStep(forward bool) {
	active := s.mgr.Active()
	if active == nil || s.search.query == "" {
		return
	}
	afterAbs, afterCol := 0, 0
	if s.search.hasHit {
		if forward {
			afterAbs = s.search.hit.AbsLine
			afterCol = s.search.hit.Col + s.search.hit.Len
		} else {
			afterAbs = s.search.hit.AbsLine
			afterCol = s.search.hit.Col
		}
	} else if !forward {
		afterAbs = len(active.Term.History()) + len(active.Term.Screen())
		afterCol = 0
	}
	hit, ok := active.FindInScrollback(s.search.query, afterAbs, afterCol, forward)
	if !ok {
		// Wrap.
		if forward {
			hit, ok = active.FindInScrollback(s.search.query, 0, 0, true)
		} else {
			total := len(active.Term.History()) + len(active.Term.Screen())
			hit, ok = active.FindInScrollback(s.search.query, total, 0, false)
		}
	}
	s.search.hasHit = ok
	s.search.hit = hit
	s.search.count = active.CountInScrollback(s.search.query)
	if !ok {
		s.search.status = "no matches"
	} else {
		s.search.status = ""
		active.ScrollToAbsLine(hit.AbsLine)
		s.scrollBarUntil = time.Now().Add(1200 * time.Millisecond)
	}
	s.app.RequestRedraw()
}

// paintSearchOverlay draws the find bar at the bottom of the frame and
// highlights the current hit when it intersects the viewport.
func (s *uiState) paintSearchOverlay(fw, fh, padPx, tabBarH int) {
	if !s.search.active {
		return
	}
	barH := s.cellH + padPx
	if barH < 20 {
		barH = 20
	}
	y0 := fh - barH
	if y0 < 0 {
		y0 = 0
	}
	bg := color.RGBA{R: 0x2a, G: 0x2c, B: 0x30, A: 0xff}
	fillRect(s.frame, fw, 0, y0, fw, fh, bg)

	label := "Find: " + s.search.query + "▋"
	if s.search.count > 0 && s.search.hasHit {
		label = fmt.Sprintf("Find: %s  (%d matches)▋", s.search.query, s.search.count)
	} else if s.search.status != "" {
		label = fmt.Sprintf("Find: %s  [%s]▋", s.search.query, s.search.status)
	}
	fg := toRGBA(s.palette.Foreground)
	if s.tabBar != nil {
		s.tabBar.drawText(s.frame, fw, fh, label, padPx, y0, fw-2*padPx, barH, fg)
	}

	if !s.search.hasHit || s.cellW <= 0 || s.cellH <= 0 {
		return
	}
	active := s.mgr.Active()
	if active == nil {
		return
	}
	top := active.ViewportTopAbsLine()
	active.Term.RLock()
	rows := active.Term.Size().R
	active.Term.RUnlock()
	abs := s.search.hit.AbsLine
	if abs < top || abs >= top+rows {
		return
	}
	viewRow := abs - top
	x0 := padPx + s.search.hit.Col*s.cellW
	yHit := tabBarH + padPx + viewRow*s.cellH
	x1 := x0 + s.search.hit.Len*s.cellW
	hi := color.RGBA{R: 0xea, G: 0xec, B: 0x23, A: 0x90}
	blendRect(s.frame, fw, x0, yHit, x1, yHit+s.cellH, hi)
}
