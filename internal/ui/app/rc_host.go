package app

import (
	"fmt"
	"strconv"

	"github.com/geckty/geckty/internal/protocol/paste"
	"github.com/geckty/geckty/internal/rc"
)

// rcHost adapts uiState to rc.Host for the optional remote-control socket.
type rcHost struct{ s *uiState }

func (h rcHost) NewTab() error {
	if h.s.newTab == nil {
		return fmt.Errorf("no tab factory")
	}
	return h.s.newTab()
}

func (h rcHost) CloseTab() error {
	return h.s.mgr.CloseActive()
}

func (h rcHost) SendText(text string) error {
	active := h.s.mgr.Active()
	if active == nil {
		return fmt.Errorf("no active session")
	}
	active.Term.RLock()
	mode := active.Term.Mode()
	active.Term.RUnlock()
	_, err := active.Write(paste.Encode(mode, text))
	h.s.app.RequestRedraw()
	return err
}

func (h rcHost) GetText() (string, error) {
	active := h.s.mgr.Active()
	if active == nil {
		return "", fmt.Errorf("no active session")
	}
	if text, ok := active.SelectedText(); ok && text != "" {
		return text, nil
	}
	return active.ScrollbackText(), nil
}

func (h rcHost) ListTabs() ([]string, error) {
	tabs := h.s.mgr.Tabs()
	activeID := h.s.mgr.ActiveID()
	out := make([]string, len(tabs))
	for i, t := range tabs {
		mark := ""
		if t.ID == activeID {
			mark = "*"
		}
		out[i] = strconv.Itoa(t.ID) + mark
	}
	return out, nil
}

var _ rc.Host = rcHost{}
