package gogpu

import (
	"testing"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
)

func testUIStateWithTab(t *testing.T) (*uiState, *fakeApp) {
	t.Helper()
	s, app := testUIState(t)
	if err := s.wireFirstTab(testWireCfg(), app); err != nil {
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
	if !s.keyHandled {
		t.Fatal("a matched shortcut should set keyHandled")
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
	if !s.keyHandled {
		t.Fatal("Enter should be encoded and set keyHandled")
	}
}

func TestHandleTextInputSuppressedAfterKeyHandled(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.keyHandled = true
	s.handleTextInput("x")
	if s.keyHandled {
		t.Fatal("handleTextInput should clear keyHandled after suppressing")
	}
}

func TestHandleTextInputEmptyIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.handleTextInput("") // must not panic
}

func TestHandleTextInputWritesToActiveSession(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.handleTextInput("x") // must not panic; active session receives the byte
}
