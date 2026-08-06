package gogpu

import (
	"runtime"
	"testing"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
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
