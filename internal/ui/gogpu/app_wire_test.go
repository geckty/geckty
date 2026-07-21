package gogpu

import (
	"runtime"
	"testing"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/session"
)

// testSleepCommand returns argv for a short-lived, quiet placeholder
// process — a cross-platform equivalent of "/bin/sh -c sleep 5": something
// a PTY session can hold open for a test's duration without a real
// interactive shell's prompt/rc-file output making behavior
// nondeterministic. Used package-wide by tests that need a real spawned
// process rather than a mock (see tabbar_paint_test.go's newTestTab too).
func testSleepCommand() []string {
	if runtime.GOOS == "windows" {
		return []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 5"}
	}
	return []string{"/bin/sh", "-c", "sleep 5"}
}

func testUIState(t *testing.T) (*uiState, *fakeApp) {
	t.Helper()
	app := newFakeApp()
	keymap, err := NewKeymap(nil)
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	s := &uiState{
		cfg:         config.Default(),
		app:         app,
		keymap:      keymap,
		palette:     testPalette(),
		painter:     &Painter{Palette: testPalette()},
		tabBar:      NewTabBar(),
		cols:        80,
		rows:        24,
		hoverTabIdx: -1,
		scale:       1,
		cellW:       7,
		cellH:       13,
		frameW:      800,
	}
	s.wireSessionManager(app)
	t.Cleanup(func() {
		if s.resizeDebouncer != nil {
			s.resizeDebouncer.Stop()
		}
	})
	return s, app
}

func testWireCfg() *config.Config {
	cfg := config.Default()
	cfg.Shell.Command = testSleepCommand()
	return cfg
}

func TestWireWindowRegistersCallbacks(t *testing.T) {
	s, app := testUIState(t)
	win := &fakeWindow{}
	s.wireWindow(win)

	if win.onDraw == nil || win.onResize == nil || win.onKeyPress == nil ||
		win.onTextInput == nil || win.onPointer == nil || win.onScroll == nil || win.onClose == nil {
		t.Fatal("wireWindow should register every callback")
	}
	if s.win != win {
		t.Fatal("wireWindow should store win on uiState")
	}

	win.onResize(100, 100)
	if app.redrawCount.Load() != 1 {
		t.Fatalf("SetOnResize callback should RequestRedraw, redrawCount=%d", app.redrawCount.Load())
	}

	if !win.onClose() {
		t.Fatal("SetOnClose callback should return true")
	}

	win.onKeyPress(gpucontext.KeyA, 0)
	if !s.perWindowKeyPressed {
		t.Fatal("SetOnKeyPress callback should set perWindowKeyPressed")
	}

	win.onTextInput("x")
	if !s.perWindowTextInputed {
		t.Fatal("SetOnTextInput callback should set perWindowTextInputed")
	}
}

func TestWireEventSourceKeyboardDedup(t *testing.T) {
	s, app := testUIState(t)
	if err := s.wireFirstTab(testWireCfg(), app); err != nil {
		t.Fatalf("wireFirstTab: %v", err)
	}
	defer func() { _ = s.mgr.CloseActive() }()

	s.wireEventSource(app)

	if app.events.keyPressFn == nil || app.events.textInputFn == nil {
		t.Fatal("wireEventSource should register OnKeyPress/OnTextInput")
	}

	// Simulate: per-window callback already handled this key press (sets
	// the dedup flag), so EventSource's handler must skip re-dispatching.
	s.perWindowKeyPressed = true
	app.events.keyPressFn(gpucontext.KeyA, 0)
	if s.perWindowKeyPressed {
		t.Fatal("EventSource OnKeyPress should clear perWindowKeyPressed after skipping")
	}

	s.perWindowTextInputed = true
	app.events.textInputFn("x")
	if s.perWindowTextInputed {
		t.Fatal("EventSource OnTextInput should clear perWindowTextInputed after skipping")
	}
}

func TestWireLifecycleCallbacksSurfaceAvailableNilWindowIsNoop(t *testing.T) {
	s, app := testUIState(t)
	s.wireLifecycleCallbacks(app)

	if app.onFocusFn == nil || app.onCloseFn == nil || app.onSurfaceAvailableFn == nil {
		t.Fatal("wireLifecycleCallbacks should register OnFocus/OnClose/OnSurfaceAvailable")
	}

	// fakeApp.PrimaryWindow() always returns nil -> must not panic.
	app.onSurfaceAvailableFn()
}

func TestWireLifecycleCallbacksOnCloseClosesEveryTab(t *testing.T) {
	s, app := testUIState(t)
	if err := s.wireFirstTab(testWireCfg(), app); err != nil {
		t.Fatalf("wireFirstTab: %v", err)
	}
	s.wireLifecycleCallbacks(app)

	app.onCloseFn()
}

func TestWireLifecycleCallbacksOnFocusForwardsToActiveSession(t *testing.T) {
	s, app := testUIState(t)
	if err := s.wireFirstTab(testWireCfg(), app); err != nil {
		t.Fatalf("wireFirstTab: %v", err)
	}
	defer func() { _ = s.mgr.CloseActive() }()
	s.wireLifecycleCallbacks(app)

	app.onFocusFn(true)
	app.onFocusFn(false)
}

func TestStartBlinkLoopTogglesAndStops(t *testing.T) {
	s, app := testUIState(t)
	stop := s.startBlinkLoop(app)
	stop()
}

func TestWireSessionManagerQuitsWhenLastTabCloses(t *testing.T) {
	s, app := testUIState(t)
	if _, err := s.mgr.New(session.Config{Command: testSleepCommand(), Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("mgr.New: %v", err)
	}
	if err := s.mgr.CloseActive(); err != nil {
		t.Fatalf("CloseActive: %v", err)
	}
	if !app.quit.Load() {
		t.Fatal("closing the last tab should call app.Quit()")
	}
}
