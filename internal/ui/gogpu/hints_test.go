package gogpu

import (
	"runtime"
	"testing"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/session"
)

func TestOpenURLHintsAndShowScrollbackKeymap(t *testing.T) {
	k, err := NewKeymap(config.Default().Keybindings)
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	eMods := gpucontext.ModControl | gpucontext.ModShift
	if runtime.GOOS == "darwin" {
		eMods = gpucontext.ModSuper | gpucontext.ModShift
	}
	a, ok := k.Match(gpucontext.KeyE, eMods)
	if !ok || a != ActionOpenURLHints {
		t.Fatalf("Match E = %q, %v; want open_url_hints", a, ok)
	}
	a, ok = k.Match(gpucontext.KeyH, gpucontext.ModControl|gpucontext.ModShift)
	if !ok || a != ActionShowScrollback {
		t.Fatalf("Match H = %q, %v; want show_scrollback", a, ok)
	}
}

func TestHandleHintsKeyEscape(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.hintsActive = true
	if !s.handleHintsKey(gpucontext.KeyEscape, 0) {
		t.Fatal("expected Escape to be handled")
	}
	if s.hintsActive {
		t.Fatal("Escape should close hints")
	}
}

func TestDispatchOpenURLHintsToggles(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.dispatchAction(ActionOpenURLHints)
	if !s.hintsOverlayActive() {
		t.Fatal("open_url_hints should activate overlay")
	}
	s.dispatchAction(ActionOpenURLHints)
	if s.hintsOverlayActive() {
		t.Fatal("open_url_hints again should close overlay")
	}
}

func TestHandleHintsKeySelectsAndSwallows(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.hintsActive = true
	s.hints = []session.URLHit{{URL: "", AbsLine: 0, Col: 0}}
	s.hintsLabels = []string{"1"}

	if !s.handleHintsKey(gpucontext.KeyA, gpucontext.ModControl) {
		t.Fatal("mods should be swallowed")
	}
	if !s.hintsActive {
		t.Fatal("control key must not close hints")
	}
	if !s.handleHintsKey(gpucontext.KeyZ, 0) {
		t.Fatal("unmatched key should still be consumed")
	}
	if !s.hintsActive {
		t.Fatal("unmatched key should not close")
	}
	// Empty URL short-circuits openURL (no external process).
	if !s.handleHintsKey(gpucontext.Key1, 0) {
		t.Fatal("matching label should be handled")
	}
	if s.hintsActive {
		t.Fatal("matching label should close hints")
	}
}

func TestPaintHintsOverlay(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	active.Term.Parse([]byte("see https://example.com/x ok\r\n"))
	s.openURLHints()
	if !s.hintsOverlayActive() || len(s.hints) == 0 {
		t.Fatal("expected URL hints from screen content")
	}
	s.frame = make([]byte, 400*200*4)
	s.frameW, s.frameH = 400, 200
	s.paintHintsOverlay(400, 200)
	s.closeURLHints()
	if s.hintsOverlayActive() {
		t.Fatal("closeURLHints should deactivate")
	}
}