package gogpu

import (
	"testing"

	"github.com/gogpu/gpucontext"
)

func TestLoadSymbolFallbackFaceMayBeNil(t *testing.T) {
	// On CI without system symbol fonts this is nil; on developer machines
	// it usually resolves. Either way it must not panic.
	_ = loadSymbolFallbackFace(13, 1)
}

func TestGlyphEntryForPrimaryASCII(t *testing.T) {
	p := testPainter()
	p.ensureAtlas()
	if _, ok := p.glyphEntryFor(false, false, 'A'); !ok {
		t.Fatal("expected primary face to have glyph for 'A'")
	}
}

func TestConfirmCloseEscapeCancels(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.confirmClose = true
	if !s.handleConfirmCloseKey(gpucontext.KeyEscape) {
		t.Fatal("Escape should be handled")
	}
	if s.confirmClose {
		t.Fatal("Escape should clear confirmClose")
	}
}

func TestConfirmCloseSwallowsOtherKeys(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.confirmClose = true
	if !s.handleConfirmCloseKey(gpucontext.KeyA) {
		t.Fatal("confirm overlay should swallow keys")
	}
	if !s.confirmClose {
		t.Fatal("non-Enter/Esc key should not dismiss confirm")
	}
}

func TestConfirmCloseEnterQuits(t *testing.T) {
	s, app := testUIStateWithTab(t)
	s.confirmClose = true
	if !s.handleConfirmCloseKey(gpucontext.KeyEnter) {
		t.Fatal("Enter should be handled")
	}
	if s.confirmClose {
		t.Fatal("Enter should clear confirmClose")
	}
	if !app.quit.Load() {
		t.Fatal("Enter should Quit the app")
	}
}

func TestSetOnCloseConfirmWhenMultipleTabs(t *testing.T) {
	s, app := testUIStateWithTab(t)
	s.cfg.Window.ConfirmClose = true
	s.dispatchAction(ActionNewTab)
	win := &fakeWindow{}
	s.wireWindow(win)
	if win.onClose == nil {
		t.Fatal("expected OnClose callback")
	}
	if win.onClose() {
		t.Fatal("OnClose with ConfirmClose and 2+ tabs should return false")
	}
	if !s.confirmClose {
		t.Fatal("OnClose should arm confirmClose overlay")
	}
	_ = app
}
