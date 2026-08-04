package gogpu

import (
	"testing"
	"time"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
)

func testUIStateWithTab(t *testing.T) (*uiState, *fakeApp) {
	t.Helper()
	s, app := testUIState(t)
	s.cfg = testWireCfg()
	if err := s.wireFirstTab(app); err != nil {
		t.Fatalf("wireFirstTab: %v", err)
	}
	t.Cleanup(func() { _ = s.mgr.CloseActive() })
	return s, app
}

func TestDispatchActionNewTab(t *testing.T) {
	s, app := testUIStateWithTab(t)
	before := len(s.mgr.Tabs())
	s.dispatchAction(ActionNewTab)
	if len(s.mgr.Tabs()) != before+1 {
		t.Fatalf("ActionNewTab: tabs = %d, want %d", len(s.mgr.Tabs()), before+1)
	}
	_ = app
}

func TestDispatchActionNextPrevTab(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.dispatchAction(ActionNewTab)
	first := s.mgr.ActiveID()
	s.dispatchAction(ActionNextTab)
	second := s.mgr.ActiveID()
	if second == first {
		t.Fatal("ActionNextTab should change the active tab when more than one exists")
	}
	s.dispatchAction(ActionPrevTab)
	if s.mgr.ActiveID() != first {
		t.Fatal("ActionPrevTab should move back to the original active tab")
	}
}

func TestDispatchActionCloseTab(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.dispatchAction(ActionCloseTab)
	// Manager auto-quits the app once empty (via wireSessionManager's
	// onChange callback) rather than leaving a zero-tab state to assert
	// on directly; just confirm this didn't panic and mgr is still usable.
	_ = s.mgr.Tabs()
}

func TestDispatchActionCopyNoSelectionIsNoop(t *testing.T) {
	s, app := testUIStateWithTab(t)
	// No selection has been made, so ActionCopy must return before ever
	// reaching the real clipboard (clipboardWrite shells out to pbcopy/
	// clip/xclip on non-darwin — a test must never touch the host's real
	// clipboard as a side effect).
	s.dispatchAction(ActionCopy)
	if app.clipboard != "" {
		t.Fatalf("clipboard = %q, want untouched with no selection", app.clipboard)
	}
}

func TestDispatchActionCopyNoActiveTabIsNoop(t *testing.T) {
	s, _ := testUIState(t) // no tab wired at all -> mgr.Active() == nil
	s.dispatchAction(ActionCopy)
}

func TestDispatchActionPasteNoActiveTabIsNoop(t *testing.T) {
	s, _ := testUIState(t)        // no tab wired at all -> mgr.Active() == nil
	s.dispatchAction(ActionPaste) // must not panic despite no active tab
}

func TestHandleKeyPressShortcutTakesPrecedence(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	before := len(s.mgr.Tabs())
	// Default non-darwin keybinding: Ctrl+Shift+T -> new_tab (see
	// internal/config/defaults.go's defaultKeybindings on non-macOS).
	keymap, err := NewKeymap([]config.Keybinding{{Key: "T", Mods: []string{"ctrl", "shift"}, Action: "new_tab"}})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	s.keymap = keymap

	s.handleKeyPress(gpucontext.KeyT, gpucontext.ModControl|gpucontext.ModShift)
	if s.keyEcho != "T" {
		t.Fatalf("a matched shortcut should set keyEcho to its base char, got %q", s.keyEcho)
	}
	if len(s.mgr.Tabs()) != before+1 {
		t.Fatalf("tabs after shortcut = %d, want %d", len(s.mgr.Tabs()), before+1)
	}
}

func TestHandleKeyPressFallsThroughToEncoding(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	keymap, err := NewKeymap(nil)
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	s.keymap = keymap

	s.handleKeyPress(gpucontext.KeyEnter, 0)
	// Enter is a control encoding — must not stick a keyEcho that can
	// swallow later IME/layout text.
	if s.keyEcho != "" {
		t.Fatalf("Enter must not leave a sticky keyEcho, got %q", s.keyEcho)
	}
}

func TestHandleTextInputSuppressesOnlyAMatchingEcho(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.keyEcho = "x"
	s.handleTextInput("x")
	if s.keyEcho != "" {
		t.Fatal("handleTextInput should clear keyEcho after consuming a matching echo")
	}
}

func TestHandleTextInputDeliversMismatchedTextDespiteStaleEcho(t *testing.T) {
	// The actual bug this guards against: a stale keyEcho left over from
	// an unrelated key (e.g. Enter) must never swallow real text that
	// doesn't match it — composed/IME text arriving shortly after an
	// unrelated control key used to be dropped just because *some* key was
	// recently "handled" (see keyEcho's doc comment).
	s, _ := testUIStateWithTab(t)
	s.keyEcho = "\r"
	s.handleTextInput("й") // must not panic; must be written, not swallowed
	if s.keyEcho != "" {
		t.Fatal("handleTextInput should still clear a stale, non-matching keyEcho")
	}
}

func TestHandleTextInputStaleEchoTTLDoesNotSwallow(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.keyEcho = "a"
	s.keyEchoAt = time.Now().Add(-time.Second) // expired
	s.handleTextInput("a")
	if s.keyEcho != "" {
		t.Fatal("expired keyEcho must be cleared")
	}
}

func TestHandleTextInputWritesToActiveSession(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.handleTextInput("x") // must not panic; active session receives the byte
}

func TestSetKeyEchoOnlyKeepsAlphanumeric(t *testing.T) {
	s, _ := testUIState(t)
	s.setKeyEcho("A")
	if s.keyEcho != "A" || s.keyEchoAt.IsZero() {
		t.Fatalf("alphanumeric echo = %q, want A with timestamp", s.keyEcho)
	}
	s.setKeyEcho("\x1b[A")
	if s.keyEcho != "" {
		t.Fatalf("escape sequence must clear keyEcho, got %q", s.keyEcho)
	}
	s.setKeyEcho("")
	if s.keyEcho != "" {
		t.Fatal("empty setKeyEcho should clear")
	}
	s.setKeyEcho("!")
	if s.keyEcho != "" {
		t.Fatalf("punctuation must not stick as keyEcho, got %q", s.keyEcho)
	}
}

func TestHandleKeyPressArrowWritesWithoutStickyEcho(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	keymap, err := NewKeymap(nil)
	if err != nil {
		t.Fatal(err)
	}
	s.keymap = keymap
	s.handleKeyPress(gpucontext.KeyUp, 0)
	if s.keyEcho != "" {
		t.Fatalf("Up must not leave sticky keyEcho, got %q", s.keyEcho)
	}
}

func TestHandleKeyReleaseLegacyIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	// KeyLegacy (default): EncodeKitty returns ok=false for releases.
	s.handleKeyRelease(gpucontext.KeyUp, 0)
}

func TestHandleKeyPressNoActiveTabIsNoop(t *testing.T) {
	s, _ := testUIState(t)
	keymap, err := NewKeymap(nil)
	if err != nil {
		t.Fatal(err)
	}
	s.keymap = keymap
	s.handleKeyPress(gpucontext.KeyEnter, 0)
	s.handleKeyRelease(gpucontext.KeyEnter, 0)
}

func TestContentPadDp(t *testing.T) {
	s, _ := testUIState(t)
	s.cfg = nil
	if s.contentPadDp() != chromeContentPadDp {
		t.Fatalf("nil cfg pad = %d, want default %d", s.contentPadDp(), chromeContentPadDp)
	}
	s.cfg = config.Default()
	s.cfg.Window.Padding = 12
	if s.contentPadDp() != 12 {
		t.Fatalf("pad = %d, want 12", s.contentPadDp())
	}
}

func TestApplyPendingConfig(t *testing.T) {
	s, _ := testUIState(t)
	s.applyPendingConfig() // nil pending — no-op

	cfg := config.Default()
	cfg.Window.Padding = 20
	s.pendingCfg.Store(cfg)
	s.applyPendingConfig()
	if s.cfg.Window.Padding != 20 {
		t.Fatalf("padding = %d, want 20 after applyPendingConfig", s.cfg.Window.Padding)
	}
	if s.pendingCfg.Load() != nil {
		t.Fatal("pendingCfg should be cleared")
	}
}

func TestHandleKeyReleaseWithKittyEventTypes(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	// Enable Disambiguate + Report event types via CSI > flags u push.
	active.Term.Parse([]byte("\x1b[>3u"))
	s.handleKeyRelease(gpucontext.KeyEscape, 0) // must not panic; may write CSI-u release
}
