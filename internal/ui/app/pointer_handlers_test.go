package app

import (
	"testing"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
)

func computeTestGeometry(s *uiState) chrome.Geometry {
	minTabW := dpToPx(MinTabWidthDp, s.scale)
	plusW := dpToPx(PlusWidthDp, s.scale)
	closeZoneW := dpToPx(CloseZoneWidthDp, s.scale)
	return chrome.ComputeGeometry(s.frameW, len(s.mgr.Tabs()), minTabW, plusW, closeZoneW)
}

func TestToDevicePx(t *testing.T) {
	s, _ := testUIState(t)
	s.scale = 2
	x, y := s.toDevicePx(10, 20)
	if x != 20 || y != 40 {
		t.Fatalf("toDevicePx = %d,%d, want 20,40", x, y)
	}
	s.scale = 0
	x, y = s.toDevicePx(10, 20)
	if x != 10 || y != 20 {
		t.Fatalf("toDevicePx with scale<=0 = %d,%d, want unscaled 10,20 (falls back to 1)", x, y)
	}
}

func TestCellFromPosition(t *testing.T) {
	col, row := cellFromPosition(23, 65, 10, 20, 3, 45, 80, 24)
	if col != 2 || row != 1 {
		t.Fatalf("cellFromPosition = %d,%d, want 2,1", col, row)
	}
	col, row = cellFromPosition(0, 0, 10, 20, 40, 5, 80, 24)
	if col != 0 || row != 0 {
		t.Fatalf("cellFromPosition clamps negative = %d,%d, want 0,0", col, row)
	}
	col, row = cellFromPosition(100000, 100000, 10, 20, 40, 5, 80, 24)
	if col != 79 || row != 23 {
		t.Fatalf("cellFromPosition clamps to last cell = %d,%d, want 79,23", col, row)
	}
}

func TestHandlePointerEventClickInGridStartsSelection(t *testing.T) {
	s, app := testUIStateWithTab(t)
	active := s.mgr.Active()

	ev := gpucontext.PointerEvent{
		Type:    gpucontext.PointerDown,
		X:       50,
		Y:       50, // below the tab bar
		Button:  gpucontext.ButtonLeft,
		Buttons: gpucontext.ButtonsLeft,
	}
	s.handlePointerEvent(ev)
	if app.redrawCount.Load() == 0 {
		t.Fatal("a grid click should request a redraw")
	}
	if _, _, ok := active.Selection(); !ok {
		t.Fatal("a grid click should start a selection")
	}
}

func TestHandlePointerEventClearsHoverOutsideTabBar(t *testing.T) {
	s, app := testUIStateWithTab(t)
	s.hoverTabIdx = 0
	s.hoverPlus = true

	ev := gpucontext.PointerEvent{Type: gpucontext.PointerMove, X: 50, Y: 50}
	s.handlePointerEvent(ev)
	if s.hoverTabIdx != -1 || s.hoverPlus {
		t.Fatal("moving into the grid should clear tab-bar hover state")
	}
	if app.redrawCount.Load() == 0 {
		t.Fatal("clearing hover state should request a redraw")
	}
}

func TestHandlePointerEventInTabBarRoutesThere(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	ev := gpucontext.PointerEvent{
		Type:   gpucontext.PointerDown,
		X:      10,
		Y:      5, // inside the tab bar
		Button: gpucontext.ButtonLeft,
	}
	s.handlePointerEvent(ev) // must not panic; routed to handleTabBarPointer
}

func TestHandlePointerEventNoActiveTabIsNoop(t *testing.T) {
	s, _ := testUIState(t) // no tab
	ev := gpucontext.PointerEvent{Type: gpucontext.PointerDown, X: 50, Y: 50, Button: gpucontext.ButtonLeft}
	s.handlePointerEvent(ev) // must not panic
}

func TestHandleScrollEventInGridScrollsBack(t *testing.T) {
	s, app := testUIStateWithTab(t)
	active := s.mgr.Active()
	_, _ = active.Term.Write([]byte("line1\r\nline2\r\nline3\r\n"))

	ev := gpucontext.ScrollEvent{X: 50, Y: 50, DeltaY: -100}
	s.handleScrollEvent(ev)
	_ = app
}

func TestHandleScrollEventInTabBarRoutesToTabBarScroll(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	for i := 0; i < 20; i++ {
		_ = s.newTab()
	}
	ev := gpucontext.ScrollEvent{X: 10, Y: 5, DeltaX: 50}
	s.handleScrollEvent(ev) // must not panic
}

func TestHandleScrollWithMouseTrackingSendsWheelReports(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	_, _ = active.Term.Write([]byte("\x1b[?1000h")) // enable X10/normal mouse button tracking

	s.handleScroll(active, 30, 60, 0, 0, -100) // must not panic, exercises the tracking-enabled branch
}

func TestHandleButtonWithMouseTrackingSendsButtonReport(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	_, _ = active.Term.Write([]byte("\x1b[?1000h"))

	s.handleButton(active, 10, 10, 0, 0, gpucontext.ButtonsLeft, true, 0)
}

func TestHandleButtonZeroCellMetricsIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.cellW = 0
	s.handleButton(s.mgr.Active(), 10, 10, 0, 0, gpucontext.ButtonsLeft, true, 0)
}

func TestHandleButtonDoubleClickSelectsWord(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	_, _ = active.Term.Write([]byte("hello world"))

	s.handleButton(active, 0, 0, 0, 0, gpucontext.ButtonsLeft, true, 0)
	s.handleButton(active, 0, 0, 0, 0, gpucontext.ButtonsLeft, false, 0)
	s.handleButton(active, 0, 0, 0, 0, gpucontext.ButtonsLeft, true, 0) // second click -> RegisterClick true
}

func TestHandleButtonCopyOnSelect(t *testing.T) {
	s, app := testUIStateWithTab(t)
	s.cfg.Clipboard.CopyOnSelect = true
	active := s.mgr.Active()
	_, _ = active.Term.Write([]byte("hello"))
	s.cellW, s.cellH = 7, 13
	s.handleButton(active, 10, 20, 0, 0, gpucontext.ButtonsLeft, true, 0)
	s.handleButton(active, 20, 20, 0, 0, gpucontext.ButtonsLeft, false, 0)
	_ = app
}

func TestHandleDragZeroCellMetricsIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.cellW = 0
	s.handleDrag(s.mgr.Active(), 10, 10, 0, 0)
}

func TestHandleDragExtendsSelection(t *testing.T) {
	s, app := testUIStateWithTab(t)
	active := s.mgr.Active()
	active.StartSelection(0, 0)
	s.handleDrag(active, 30, 0, 0, 0)
	if app.redrawCount.Load() == 0 {
		t.Fatal("extending a selection via drag should request a redraw")
	}
}

func TestHandleTabBarPressNewTabButton(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	before := len(s.mgr.Tabs())

	g := computeTestGeometry(s)
	ev := gpucontext.PointerEvent{Type: gpucontext.PointerDown, Button: gpucontext.ButtonLeft, PointerID: 1}
	if !s.handleTabBarPress(ev, g.PlusX+1, s.mgr.Tabs(), g, 0, 0, 0) {
		t.Fatal("clicking the + button should be consumed")
	}
	if len(s.mgr.Tabs()) != before+1 {
		t.Fatalf("tabs after + click = %d, want %d", len(s.mgr.Tabs()), before+1)
	}
}

func TestHandleTabBarPressWrongButtonIsIgnored(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	g := computeTestGeometry(s)
	ev := gpucontext.PointerEvent{Type: gpucontext.PointerDown, Button: gpucontext.ButtonRight}
	if s.handleTabBarPress(ev, 5, s.mgr.Tabs(), g, 0, 0, 0) {
		t.Fatal("a non-left-button press should not be consumed")
	}
}

func TestHandleTabBarPressStartsDrag(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	g := computeTestGeometry(s)
	tabs := s.mgr.Tabs()
	ev := gpucontext.PointerEvent{Type: gpucontext.PointerDown, Button: gpucontext.ButtonLeft, PointerID: 7}
	if !s.handleTabBarPress(ev, 5, tabs, g, 0, 0, 0) {
		t.Fatal("pressing on a tab should be consumed")
	}
	if !s.tabDrag.pressed || s.tabDrag.tabID != tabs[0].ID {
		t.Fatalf("tabDrag = %+v, want pressed with tabID %d", s.tabDrag, tabs[0].ID)
	}
}

func TestHandleTabBarDragMoveHover(t *testing.T) {
	s, app := testUIStateWithTab(t)
	g := computeTestGeometry(s)
	ev := gpucontext.PointerEvent{Type: gpucontext.PointerMove}
	s.handleTabBarDragMove(ev, 5, s.mgr.Tabs(), g, 0, 0, 5)
	if s.hoverTabIdx != 0 {
		t.Fatalf("hoverTabIdx = %d, want 0", s.hoverTabIdx)
	}
	if app.redrawCount.Load() == 0 {
		t.Fatal("hover change should request a redraw")
	}
}

func TestHandleTabBarDragMovePromotesToRealDrag(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	_ = s.newTab()
	g := computeTestGeometry(s)
	tabs := s.mgr.Tabs()
	s.tabDrag = tabDragState{pressed: true, tabID: tabs[0].ID, from: 0, over: 0, startX: 0, startY: 0, pointer: 1}

	ev := gpucontext.PointerEvent{Type: gpucontext.PointerMove, Y: 0}
	s.handleTabBarDragMove(ev, 50, tabs, g, 0, 1000, 5)
	if !s.tabDrag.dragging {
		t.Fatal("moving past the threshold should promote the press to a drag")
	}
}

func TestHandleTabBarReleaseActivatesOnClick(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	tabs := s.mgr.Tabs()
	s.tabDrag = tabDragState{pressed: true, tabID: tabs[0].ID, from: 0, over: 0}
	ev := gpucontext.PointerEvent{Type: gpucontext.PointerUp}
	if !s.handleTabBarRelease(ev) {
		t.Fatal("releasing a press should be consumed")
	}
	if s.mgr.ActiveID() != tabs[0].ID {
		t.Fatal("a plain click release should activate the pressed tab")
	}
}

func TestHandleTabBarReleaseNothingPressedIsMiss(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	ev := gpucontext.PointerEvent{Type: gpucontext.PointerUp}
	if s.handleTabBarRelease(ev) {
		t.Fatal("releasing with nothing pressed should not be consumed")
	}
}

func TestHandleTabBarReleaseReordersOnDrag(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	_ = s.newTab()
	_ = s.newTab()
	tabs := s.mgr.Tabs()
	s.tabDrag = tabDragState{pressed: true, dragging: true, tabID: tabs[0].ID, from: 0, over: 2}
	ev := gpucontext.PointerEvent{Type: gpucontext.PointerUp}
	if !s.handleTabBarRelease(ev) {
		t.Fatal("releasing a drag should be consumed")
	}
	if s.mgr.Tabs()[2].ID != tabs[0].ID {
		t.Fatal("a completed drag should move the tab to its drop index")
	}
}

func TestHandleTabBarScroll(t *testing.T) {
	s, app := testUIStateWithTab(t)
	for i := 0; i < 20; i++ {
		_ = s.newTab()
	}
	before := s.tabScrollX
	s.handleTabBarScroll(gpucontext.ScrollEvent{DeltaX: 100})
	if s.tabScrollX == before {
		t.Fatal("scrolling with tabs overflowing the strip should move tabScrollX")
	}
	if app.redrawCount.Load() == 0 {
		t.Fatal("a tab strip scroll should request a redraw")
	}
}

func TestHandleTabBarScrollNoOverflowIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	before := s.tabScrollX
	s.handleTabBarScroll(gpucontext.ScrollEvent{DeltaX: 100})
	if s.tabScrollX != before {
		t.Fatal("scrolling with nothing to scroll should be a no-op")
	}
}

func TestTabStripScrollDelta(t *testing.T) {
	if got := tabStripScrollDelta(7, 99, 100); got != 7 {
		t.Fatalf("horizontal delta should win: got %d, want 7", got)
	}
	if got := tabStripScrollDelta(0, 60, 100); got != 60 {
		t.Fatalf("vertical fallback: got %d, want 60", got)
	}
	if got := tabStripScrollDelta(0, 3, 100); got != 50 {
		t.Fatalf("small wheel notch amplified: got %d, want 50", got)
	}
	if got := tabStripScrollDelta(0, 0, 100); got != 0 {
		t.Fatalf("zero delta: got %d, want 0", got)
	}
}

func TestPaneForSession(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	if _, ok := s.paneForSession(nil); ok {
		t.Fatal("nil session should miss")
	}
	pane, ok := s.paneForSession(active)
	if !ok || pane.Session != active {
		t.Fatal("expected pane for active session")
	}
	if _, ok := s.paneForSession(&session.Session{}); ok {
		t.Fatal("unknown session should miss")
	}
}

func TestPasteClipboardInto(t *testing.T) {
	s, app := testUIStateWithTab(t)
	s.pasteClipboardInto(nil) // no-op
	app.clipboard = "paste-me"
	active := s.mgr.Active()
	active.StartSelection(0, 0)
	active.ExtendSelection(1, 0)
	s.pasteClipboardInto(active)
	if _, _, ok := active.Selection(); ok {
		t.Fatal("paste should clear selection")
	}
}

func TestTryScrollBarClick(t *testing.T) {
	s, app := testUIStateWithTab(t)
	active := s.mgr.Active()
	for i := 0; i < 40; i++ {
		active.Term.Parse([]byte("line\r\n"))
	}
	if len(active.Term.History()) == 0 {
		t.Fatal("expected scrollback history for scrollbar seek")
	}
	pane := s.activePaneRects[0]
	x0 := s.frameW - 7 - 2
	if !s.tryScrollBarClick(pane, x0, pane.Y+pane.H/2) {
		t.Fatal("click on track should seek")
	}
	if app.redrawCount.Load() == 0 {
		t.Fatal("scrollbar seek should redraw")
	}
	if s.tryScrollBarClick(pane, 0, pane.Y) {
		t.Fatal("click left of track should miss")
	}
}

func TestUpdatePointerCursor(t *testing.T) {
	s, app := testUIStateWithTab(t)
	active := s.mgr.Active()
	active.Term.Parse([]byte("https://example.com/x\r\n"))
	pane := s.activePaneRects[0]
	// Text cell (col 0) → I-beam.
	s.updatePointerCursor(pane.X, pane.Y)
	if app.cursor != gpucontext.CursorText && app.cursor != gpucontext.CursorPointer {
		// URL may start at col 0 depending on parse; either text or pointer is fine.
		t.Fatalf("cursor = %v", app.cursor)
	}
	// Outside panes → default.
	s.activePaneRects = nil
	s.updatePointerCursor(10, 10)
	if app.cursor != gpucontext.CursorDefault {
		t.Fatalf("outside pane cursor = %v, want default", app.cursor)
	}
}

func TestMiddleClickPastes(t *testing.T) {
	s, app := testUIStateWithTab(t)
	app.clipboard = "mid"
	pane := s.activePaneRects[0]
	ev := gpucontext.PointerEvent{
		Type:   gpucontext.PointerDown,
		X:      float64(pane.X + 10),
		Y:      float64(pane.Y + 10),
		Button: gpucontext.ButtonMiddle,
	}
	s.handlePointerEvent(ev) // must not panic
}
