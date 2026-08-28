package app

import (
	"time"

	"github.com/geckty/geckty/internal/protocol/paste"
	"github.com/geckty/geckty/internal/session"
)

// dispatchAction applies a keymap-matched action. Copy/paste are
// synchronous here since gogpu's clipboard calls return directly —
// see clipboard.go.
func (s *uiState) dispatchAction(action Action) {
	switch action {
	case ActionNewTab:
		_ = s.newTab()
	case ActionCloseTab:
		if id := s.mgr.ActiveID(); id >= 0 {
			_ = s.mgr.Close(id)
		}
	case ActionClosePane:
		_ = s.mgr.CloseActive()
	case ActionNextTab:
		s.mgr.Next()
	case ActionPrevTab:
		s.mgr.Prev()
	case ActionNextPane:
		s.mgr.NextPane()
	case ActionPrevPane:
		s.mgr.PrevPane()
	case ActionSplitVertical:
		s.splitActivePane(session.SplitVertical)
	case ActionSplitHorizontal:
		s.splitActivePane(session.SplitHorizontal)
	case ActionScrollToPrevPrompt:
		s.scrollPromptAndFlash(-1)
	case ActionScrollToNextPrompt:
		s.scrollPromptAndFlash(1)
	case ActionSelectLastCmdOutput:
		if active := s.mgr.Active(); active != nil && active.SelectLastCommandOutput() {
			s.flashScrollBar()
		}
	case ActionCopy:
		s.copySelection()
	case ActionPaste:
		s.pasteClipboard()
	case ActionSearchScrollback:
		if s.searchActive() {
			s.closeSearch()
		} else {
			s.closeURLHints()
			s.openSearch()
		}
	case ActionOpenURLHints:
		if s.hintsOverlayActive() {
			s.closeURLHints()
		} else {
			s.closeSearch()
			s.openURLHints()
		}
	case ActionShowScrollback:
		s.showScrollbackInPager()
	case ActionIncreaseFontSize:
		s.adjustFontZoom(1)
	case ActionDecreaseFontSize:
		s.adjustFontZoom(-1)
	case ActionResetFontSize:
		s.adjustFontZoom(0)
	}
}

func (s *uiState) flashScrollBar() {
	s.scrollBarUntil = time.Now().Add(1200 * time.Millisecond)
	s.app.RequestRedraw()
}

func (s *uiState) scrollPromptAndFlash(dir int) {
	if active := s.mgr.Active(); active != nil && active.ScrollToPrompt(dir) {
		s.flashScrollBar()
	}
}

func (s *uiState) copySelection() {
	if active := s.mgr.Active(); active != nil {
		if text, ok := active.SelectedText(); ok && text != "" {
			_ = clipboardWrite(s.app, text)
		}
	}
}

func (s *uiState) pasteClipboard() {
	if active := s.mgr.Active(); active != nil {
		if text, err := clipboardRead(s.app); err == nil && text != "" {
			active.Term.RLock()
			mode := active.Term.Mode()
			active.Term.RUnlock()
			_, _ = active.Write(paste.Encode(mode, text))
		}
	}
}
