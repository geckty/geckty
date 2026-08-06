package gogpu

import (
	"image/color"

	"github.com/gogpu/gpucontext"
)

// hintLabels are overlay keys: digits 1-9, then a-z (35 max).
var hintLabels = func() []string {
	out := make([]string, 0, 35)
	for c := '1'; c <= '9'; c++ {
		out = append(out, string(c))
	}
	for c := 'a'; c <= 'z'; c++ {
		out = append(out, string(c))
	}
	return out
}()

func (s *uiState) openURLHints() {
	active := s.mgr.Active()
	if active == nil {
		return
	}
	hits := active.CollectURLs(len(hintLabels))
	labels := make([]string, len(hits))
	for i := range hits {
		labels[i] = hintLabels[i]
	}
	s.hintsActive = true
	s.hints = hits
	s.hintsLabels = labels
	s.app.RequestRedraw()
}

func (s *uiState) closeURLHints() {
	s.hintsActive = false
	s.hints = nil
	s.hintsLabels = nil
	s.app.RequestRedraw()
}

func (s *uiState) hintsOverlayActive() bool {
	return s.hintsActive
}

// handleHintsKey consumes keys while the URL-hints overlay is open.
func (s *uiState) handleHintsKey(key gpucontext.Key, mods gpucontext.Modifiers) bool {
	if !s.hintsActive {
		return false
	}
	if key == gpucontext.KeyEscape {
		s.closeURLHints()
		return true
	}
	if mods&(gpucontext.ModControl|gpucontext.ModSuper|gpucontext.ModAlt) != 0 {
		return true // swallow while overlay is open
	}
	ch := keyToChar[key]
	if ch == "" {
		return true
	}
	label := ch
	if len(label) == 1 && label[0] >= 'A' && label[0] <= 'Z' {
		label = string(label[0] + ('a' - 'A'))
	}
	for i, l := range s.hintsLabels {
		if l == label {
			openURL(s.hints[i].URL)
			s.closeURLHints()
			return true
		}
	}
	return true
}

// paintHintsOverlay draws numbered/lettered labels over visible URL hits.
func (s *uiState) paintHintsOverlay(fw, fh int) {
	if !s.hintsActive || len(s.hints) == 0 || s.cellW <= 0 || s.cellH <= 0 {
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
	ox, oy := s.contentOX, s.contentOY
	if pane, ok := s.paneForSession(active); ok {
		ox, oy = pane.X, pane.Y
	}
	fg := color.RGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
	bg := color.RGBA{R: 0xea, G: 0xec, B: 0x23, A: 0xff}
	for i, hit := range s.hints {
		if hit.AbsLine < top || hit.AbsLine >= top+rows {
			continue
		}
		viewRow := hit.AbsLine - top
		x0 := ox + hit.Col*s.cellW
		y0 := oy + viewRow*s.cellH
		label := s.hintsLabels[i]
		w := s.cellW * len(label)
		if w < s.cellW {
			w = s.cellW
		}
		fillRect(s.frame, fw, x0, y0, x0+w, y0+s.cellH, bg)
		if s.tabBar != nil {
			s.tabBar.drawText(s.frame, fw, fh, label, x0, y0, w, s.cellH, fg)
		}
	}
}
