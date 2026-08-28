package app

import (
	"testing"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
)

func TestAdjustFontZoom(t *testing.T) {
	s, app := testUIStateWithTab(t)
	base := float64(s.cfg.Font.Size)
	s.adjustFontZoom(2)
	if s.fontZoomDelta != 2 {
		t.Fatalf("fontZoomDelta = %v, want 2", s.fontZoomDelta)
	}
	if s.fontSizeCurrent != 0 {
		t.Fatal("adjustFontZoom should bust the ensureFonts cache")
	}
	if app.redrawCount.Load() == 0 {
		t.Fatal("adjustFontZoom should request a redraw")
	}
	s.adjustFontZoom(0)
	if s.fontZoomDelta != 0 {
		t.Fatalf("reset fontZoomDelta = %v, want 0", s.fontZoomDelta)
	}
	s.fontZoomDelta = fontZoomMaxPt - base
	s.adjustFontZoom(10)
	if base+s.fontZoomDelta != fontZoomMaxPt {
		t.Fatalf("zoomed size = %v, want clamped %v", base+s.fontZoomDelta, fontZoomMaxPt)
	}
}

func TestDispatchFontZoomActions(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.dispatchAction(ActionIncreaseFontSize)
	if s.fontZoomDelta != 1 {
		t.Fatalf("increase delta = %v, want 1", s.fontZoomDelta)
	}
	s.dispatchAction(ActionDecreaseFontSize)
	if s.fontZoomDelta != 0 {
		t.Fatalf("decrease delta = %v, want 0", s.fontZoomDelta)
	}
	s.dispatchAction(ActionIncreaseFontSize)
	s.dispatchAction(ActionResetFontSize)
	if s.fontZoomDelta != 0 {
		t.Fatalf("reset delta = %v, want 0", s.fontZoomDelta)
	}
}

func TestHandleKeyPressFontZoomSwallowsTextEcho(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	k, err := NewKeymap([]config.Keybinding{
		{Key: "=", Mods: []string{"ctrl"}, Action: string(ActionIncreaseFontSize)},
		{Key: "-", Mods: []string{"ctrl"}, Action: string(ActionDecreaseFontSize)},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.keymap = k
	s.handleKeyPress(gpucontext.KeyEqual, gpucontext.ModControl)
	s.handleTextInput("=")
	if s.fontZoomDelta != 1 {
		t.Fatalf("fontZoomDelta = %v, want 1", s.fontZoomDelta)
	}
	s.handleKeyPress(gpucontext.KeyMinus, gpucontext.ModControl)
	s.handleTextInput("-")
	if s.fontZoomDelta != 0 {
		t.Fatalf("fontZoomDelta after decrease = %v, want 0", s.fontZoomDelta)
	}
}

func TestKeymapFontZoomActions(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "=", Mods: []string{"ctrl"}, Action: string(ActionIncreaseFontSize)},
		{Key: "-", Mods: []string{"ctrl"}, Action: string(ActionDecreaseFontSize)},
		{Key: "0", Mods: []string{"ctrl"}, Action: string(ActionResetFontSize)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := k.Match(gpucontext.KeyEqual, gpucontext.ModControl); !ok || a != ActionIncreaseFontSize {
		t.Fatalf("Match(=) = %q, %v", a, ok)
	}
	if a, ok := k.Match(gpucontext.KeyMinus, gpucontext.ModControl); !ok || a != ActionDecreaseFontSize {
		t.Fatalf("Match(-) = %q, %v", a, ok)
	}
	if a, ok := k.Match(gpucontext.Key0, gpucontext.ModControl); !ok || a != ActionResetFontSize {
		t.Fatalf("Match(0) = %q, %v", a, ok)
	}
}
