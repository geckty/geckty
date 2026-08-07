package gogpu

import (
	"math"
	"time"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/protocol/mouse"
	"github.com/geckty/geckty/internal/protocol/paste"
	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
)

const (
	dragAutoScrollEdgeZoneDp = 40 // pointer proximity to strip edge that triggers auto-scroll during a tab drag
	dragAutoScrollStepDp     = 8  // strip scroll distance per drag-move event once inside the edge zone

	// Selection drag near the top/bottom of the grid scrolls scrollback so
	// the selection can extend past the visible viewport (Kitty-style).
	selEdgeZoneCells      = 1
	selEdgeScrollInterval = 50 * time.Millisecond
	selEdgeScrollLines    = 1
)

// handlePointerEvent routes pointer events to the tab bar or the grid:
// the tab bar if the press/drag started there or the pointer is currently
// in its Y range, otherwise the terminal grid (hit-testing panes when split).
func (s *uiState) handlePointerEvent(ev gpucontext.PointerEvent) {
	x, y := s.toDevicePx(ev.X, ev.Y)
	tabBarPx := s.tabBarHeightPx()
	inTabBar := y < tabBarPx

	handled := false
	if s.tabDrag.pressed || inTabBar {
		handled = s.handleTabBarPointer(ev, x)
	} else if s.hoverTabIdx >= 0 || s.hoverPlus {
		s.hoverTabIdx = -1
		s.hoverPlus = false
		s.app.RequestRedraw()
	}
	if handled || inTabBar {
		return
	}

	active := s.mgr.Active()
	if active == nil {
		return
	}
	switch ev.Type {
	case gpucontext.PointerDown:
		if pane, ok := s.paneAt(x, y); ok && pane.Session != nil {
			if s.tryScrollBarClick(pane, x, y) {
				return
			}
			if pane.Session != active {
				s.mgr.SetFocus(pane.Session)
				active = pane.Session
			}
			if ev.Button == gpucontext.ButtonMiddle || ev.Buttons.HasMiddle() {
				s.pasteClipboardInto(active)
				return
			}
			s.handleButton(active, x, y, pane.X, pane.Y, ev.Buttons, true, ev.Modifiers)
		}
	case gpucontext.PointerUp:
		if pane, ok := s.paneForSession(active); ok {
			s.handleButton(active, x, y, pane.X, pane.Y, ev.Buttons, false, ev.Modifiers)
		}
	case gpucontext.PointerMove:
		s.updatePointerCursor(x, y)
		if ev.Buttons.HasLeft() {
			if pane, ok := s.paneForSession(active); ok {
				s.handleDrag(active, x, y, pane.X, pane.Y)
			}
		}
	}
}

func (s *uiState) paneAt(x, y int) (session.PaneRect, bool) {
	for _, p := range s.activePaneRects {
		if x >= p.X && x < p.X+p.W && y >= p.Y && y < p.Y+p.H {
			return p, true
		}
	}
	return session.PaneRect{}, false
}

func (s *uiState) paneForSession(sess *session.Session) (session.PaneRect, bool) {
	if sess == nil {
		return session.PaneRect{}, false
	}
	for _, p := range s.activePaneRects {
		if p.Session == sess {
			return p, true
		}
	}
	return session.PaneRect{}, false
}

func (s *uiState) toDevicePx(x, y float64) (int, int) {
	scale := s.scale
	if scale <= 0 {
		scale = 1
	}
	return int(x * scale), int(y * scale)
}

func (s *uiState) handleScrollEvent(ev gpucontext.ScrollEvent) {
	x, y := s.toDevicePx(ev.X, ev.Y)
	tabBarPx := s.tabBarHeightPx()
	if y < tabBarPx {
		s.handleTabBarScroll(ev)
		return
	}
	pane, ok := s.paneAt(x, y)
	if !ok || pane.Session == nil {
		return
	}
	s.handleScroll(pane.Session, x, y, pane.X, pane.Y, ev.DeltaY)
}

// cellFromPosition converts a device-pixel window position into a 0-based
// (col, row) cell. originX/originY are the top-left of the pane grid in
// device pixels. Clamped to [0, cols-1] / [0, rows-1].
func cellFromPosition(x, y, cellWidth, cellHeight, originX, originY, cols, rows int) (col, row int) {
	col = (x - originX) / cellWidth
	row = (y - originY) / cellHeight
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	if cols > 0 && col > cols-1 {
		col = cols - 1
	}
	if rows > 0 && row > rows-1 {
		row = rows - 1
	}
	return col, row
}

func (s *uiState) handleScroll(sess *session.Session, x, y, originX, originY int, deltaY float64) {
	if s.cellW <= 0 || s.cellH <= 0 {
		return
	}
	if s.cfg != nil && s.cfg.Scrollback.WheelMultiplier > 0 {
		deltaY *= s.cfg.Scrollback.WheelMultiplier
	}
	lines := accumulateScrollLines(deltaY, s.cellH, &s.scrollAccumPx)
	if lines == 0 {
		return
	}

	sess.Term.RLock()
	mode := sess.Term.Mode()
	sz := sess.Term.Size()
	sess.Term.RUnlock()

	if !mouse.TrackingEnabled(mode) || s.lastMods.HasShift() {
		sess.ScrollBy(-lines)
		s.scrollBarUntil = time.Now().Add(1200 * time.Millisecond)
		s.app.RequestRedraw()
		return
	}

	dir := mouse.Up
	if lines > 0 {
		dir = mouse.Down
	}
	col, row := cellFromPosition(x, y, s.cellW, s.cellH, originX, originY, sz.C, sz.R)
	n := lines
	if n < 0 {
		n = -n
	}
	for i := 0; i < n; i++ {
		if b, ok := mouse.EncodeWheel(mode, dir, col+1, row+1, 0); ok {
			_, _ = sess.Write(b)
		}
	}
}

func (s *uiState) handleButton(sess *session.Session, x, y, originX, originY int, buttons gpucontext.Buttons, pressed bool, mods gpucontext.Modifiers) {
	if s.cellW <= 0 || s.cellH <= 0 {
		return
	}
	sess.Term.RLock()
	mode := sess.Term.Mode()
	sz := sess.Term.Size()
	sess.Term.RUnlock()

	col, row := cellFromPosition(x, y, s.cellW, s.cellH, originX, originY, sz.C, sz.R)

	if mouse.TrackingEnabled(mode) && !mods.HasShift() {
		if b, ok := mouse.EncodeButton(mode, mouse.ButtonLeft, pressed, col+1, row+1, 0); ok {
			_, _ = sess.Write(b)
		}
		return
	}

	if !buttons.HasLeft() && pressed {
		return
	}
	if pressed {
		absLine := sess.ViewToAbsLine(row)
		// Cmd/Ctrl+click opens a URL under the pointer (Kitty-style open_url).
		if mods.HasSuper() || mods.HasControl() {
			if u, ok := sess.URLAt(absLine, col); ok {
				openURL(u)
				s.app.RequestRedraw()
				return
			}
		}
		wordChars := ""
		if s.cfg != nil {
			wordChars = s.cfg.Selection.WordChars
		}
		switch sess.RegisterClick(col, absLine) {
		case 2:
			sess.SelectWord(col, absLine, wordChars)
		case 3:
			sess.SelectLine(absLine)
		default:
			sess.StartSelectionMode(col, absLine, mods.HasAlt())
		}
	} else {
		sess.EndSelection()
		if s.cfg != nil && s.cfg.Clipboard.CopyOnSelect {
			if text, ok := sess.SelectedText(); ok && text != "" {
				_ = clipboardWrite(s.app, text)
			}
		}
	}
	// Selection state changes don't flow through session.Config.OnDirty
	// (that only fires on new PTY output) and macOS doesn't run with
	// WithContinuousRender, so without an explicit redraw here the new
	// selection wouldn't paint until the next unrelated frame (PTY output
	// or the ~530ms blink tick) happened to land.
	s.app.RequestRedraw()
}

func (s *uiState) handleDrag(sess *session.Session, x, y, originX, originY int) {
	if s.cellW <= 0 || s.cellH <= 0 {
		return
	}
	sess.Term.RLock()
	mode := sess.Term.Mode()
	sz := sess.Term.Size()
	sess.Term.RUnlock()

	col, row := cellFromPosition(x, y, s.cellW, s.cellH, originX, originY, sz.C, sz.R)

	if b, ok := mouse.EncodeMotion(mode, mouse.ButtonLeft, col+1, row+1, 0); ok {
		_, _ = sess.Write(b)
		return
	}
	if mouse.TrackingEnabled(mode) {
		return
	}

	if s.maybeSelectionEdgeScroll(sess, y, originY, sz.R) {
		col, row = cellFromPosition(x, y, s.cellW, s.cellH, originX, originY, sz.C, sz.R)
	}
	sess.ExtendSelection(col, sess.ViewToAbsLine(row))
	// See handleButton's identical comment: selection changes need an
	// explicit redraw or they lag behind the pointer until some
	// unrelated frame happens to repaint.
	s.app.RequestRedraw()
}

// maybeSelectionEdgeScroll scrolls scrollback when the pointer sits in the
// top/bottom edge zone during a selection drag. Returns true if the
// viewport moved (caller should re-resolve the cell under the pointer).
func (s *uiState) maybeSelectionEdgeScroll(sess *session.Session, y, originY, rows int) bool {
	if !sess.SelectionDragging() || rows <= 0 || s.cellH <= 0 {
		return false
	}
	gridTop := originY
	gridBottom := gridTop + rows*s.cellH
	edge := selEdgeZoneCells * s.cellH
	if edge < 1 {
		edge = 1
	}
	now := time.Now()
	if now.Sub(s.selEdgeLast) < selEdgeScrollInterval {
		return false
	}
	delta := 0
	switch {
	case y < gridTop+edge:
		delta = selEdgeScrollLines // toward older history
	case y > gridBottom-edge:
		delta = -selEdgeScrollLines // toward live bottom
	default:
		return false
	}
	before := sess.ScrollOffset()
	after := sess.ScrollBy(delta)
	if after == before {
		return false
	}
	s.selEdgeLast = now
	s.scrollBarUntil = now.Add(1200 * time.Millisecond)
	return true
}

// tryScrollBarClick seeks scrollback when the pointer hits the overlay track.
func (s *uiState) tryScrollBarClick(pane session.PaneRect, x, y int) bool {
	if pane.Session == nil || s.frameW <= 0 {
		return false
	}
	trackW, margin := 7, 2
	x0 := s.frameW - trackW - margin
	if x < x0 {
		return false
	}
	pane.Session.Term.RLock()
	hist := len(pane.Session.Term.History())
	pane.Session.Term.RUnlock()
	if hist <= 0 {
		return false
	}
	y0 := pane.Y + margin
	y1 := pane.Y + pane.H - margin
	if y1 <= y0 {
		return false
	}
	frac := float64(y-y0) / float64(y1-y0)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	// Thumb top = scrolled fully into history; bottom = live (offset 0).
	want := int(float64(hist) * (1 - frac))
	cur := pane.Session.ScrollOffset()
	pane.Session.ScrollBy(want - cur)
	s.scrollBarUntil = time.Now().Add(1200 * time.Millisecond)
	s.app.RequestRedraw()
	return true
}

func (s *uiState) pasteClipboardInto(active *session.Session) {
	if active == nil {
		return
	}
	text, err := clipboardRead(s.app)
	if err != nil || text == "" {
		return
	}
	active.Term.RLock()
	mode := active.Term.Mode()
	active.Term.RUnlock()
	_, _ = active.Write(paste.Encode(mode, text))
	active.ClearSelection()
	s.app.RequestRedraw()
}

func (s *uiState) updatePointerCursor(x, y int) {
	setter, ok := s.app.(interface{ SetCursor(gpucontext.CursorShape) })
	if !ok {
		return
	}
	pane, hit := s.paneAt(x, y)
	if !hit || pane.Session == nil || s.cellW <= 0 || s.cellH <= 0 {
		setter.SetCursor(gpucontext.CursorDefault)
		return
	}
	col, row := cellFromPosition(x, y, s.cellW, s.cellH, pane.X, pane.Y, 0, 0)
	abs := pane.Session.ViewToAbsLine(row)
	if _, ok := pane.Session.URLAt(abs, col); ok {
		setter.SetCursor(gpucontext.CursorPointer)
		return
	}
	setter.SetCursor(gpucontext.CursorText)
}

// handleTabBarPointer handles a pointer event for the tab strip — select,
// close, new-tab, or drag-reorder preview. Returns whether the event was
// consumed (so a cleared miss can fall through to the terminal). It
// resolves the tab strip's geometry/scroll-clamp/activeIdx once up front
// (shared by all three event types) and dispatches the rest to
// handleTabBarPress/handleTabBarDragMove/handleTabBarRelease.
func (s *uiState) handleTabBarPointer(ev gpucontext.PointerEvent, x int) bool {
	drag := &s.tabDrag
	if drag.pressed && ev.Type != gpucontext.PointerDown && ev.PointerID != drag.pointer {
		return true
	}

	minTabW := dpToPx(MinTabWidthDp, s.scale)
	plusW := dpToPx(PlusWidthDp, s.scale)
	if !s.tabBarShowPlus() {
		plusW = 0
	}
	closeZoneW := dpToPx(CloseZoneWidthDp, s.scale)
	closeEdgePad := dpToPx(CloseEdgePadDp, s.scale)
	thresh := dpToPx(tabDragThresholdDp, s.scale)
	if thresh < 1 {
		thresh = 1
	}

	tabs := s.mgr.Tabs()
	if !s.tabBarShowTabs() {
		tabs = nil
	}
	g := chrome.ComputeGeometry(s.frameW, len(tabs), minTabW, plusW, closeZoneW)
	maxScroll := chrome.ScrollMax(g, len(tabs))
	if s.tabScrollX < 0 {
		s.tabScrollX = 0
	}
	if s.tabScrollX > maxScroll {
		s.tabScrollX = maxScroll
	}
	activeIdx := -1
	if activeID := s.mgr.ActiveID(); activeID >= 0 {
		for i := range tabs {
			if tabs[i].ID == activeID {
				activeIdx = i
				break
			}
		}
	}

	switch ev.Type {
	case gpucontext.PointerDown:
		return s.handleTabBarPress(ev, x, tabs, g, activeIdx, closeZoneW, closeEdgePad)
	case gpucontext.PointerMove:
		return s.handleTabBarDragMove(ev, x, tabs, g, activeIdx, maxScroll, thresh)
	case gpucontext.PointerUp, gpucontext.PointerCancel:
		return s.handleTabBarRelease(ev)
	}
	return false
}

// handleTabBarPress handles a PointerDown within the tab strip: hitting the
// "+" button opens a new tab; hitting an already-hovered tab's close ×
// closes it; a left-button press anywhere else on a tab starts a potential
// drag-reorder (see handleTabBarDragMove for when it actually becomes a
// drag). Any other button, or a press that misses every tab and the "+"
// button, is not consumed.
func (s *uiState) handleTabBarPress(ev gpucontext.PointerEvent, x int, tabs []session.Tab, g chrome.Geometry, activeIdx, closeZoneW, closeEdgePad int) bool {
	drag := &s.tabDrag
	if ev.Button != gpucontext.ButtonLeft {
		return false
	}
	if chrome.IsPlusHit(x, g) {
		*drag = tabDragState{}
		s.hoverTabIdx = -1
		s.hoverPlus = false
		_ = s.newTab()
		return true
	}
	idx := chrome.TabAtScrolledPinned(x, s.tabScrollX, g, len(tabs), activeIdx)
	if idx < 0 {
		*drag = tabDragState{}
		s.hoverTabIdx = -1
		s.hoverPlus = false
		return false
	}
	id := tabs[idx].ID
	tabLeft := chrome.TabVisualLeft(idx, activeIdx, s.tabScrollX, g, len(tabs))
	localX := x - tabLeft
	if s.hoverTabIdx == idx && chrome.IsCloseHit(localX, g, closeZoneW, closeEdgePad) {
		*drag = tabDragState{}
		s.hoverTabIdx = -1
		s.hoverPlus = false
		_ = s.mgr.Close(id)
		return true
	}
	s.hoverTabIdx = -1
	s.hoverPlus = false
	*drag = tabDragState{
		pressed: true,
		tabID:   id,
		from:    idx,
		over:    idx,
		startX:  float64(x),
		startY:  float64(ev.Y * s.scale),
		pointer: ev.PointerID,
	}
	return true
}

// handleTabBarDragMove handles PointerMove within the tab strip. While no
// button is pressed, it only updates hover state (for the "+" button and
// whichever tab, if any, the pointer sits over). Once a press has started
// (see handleTabBarPress), it promotes the press to a drag-reorder once the
// pointer clears tabDragThresholdDp, then updates the drop-index preview and
// auto-scrolls the strip when the pointer nears its edge.
func (s *uiState) handleTabBarDragMove(ev gpucontext.PointerEvent, x int, tabs []session.Tab, g chrome.Geometry, activeIdx, maxScroll, thresh int) bool {
	drag := &s.tabDrag
	if !drag.pressed {
		idx := chrome.TabAtScrolledPinned(x, s.tabScrollX, g, len(tabs), activeIdx)
		plusHit := chrome.IsPlusHit(x, g)
		if s.hoverTabIdx != idx || s.hoverPlus != plusHit {
			s.hoverTabIdx = idx
			s.hoverPlus = plusHit
			s.app.RequestRedraw()
		}
		return idx >= 0 || plusHit
	}
	if !drag.dragging {
		dx := float64(x) - drag.startX
		dy := float64(ev.Y*s.scale) - drag.startY
		if dx*dx+dy*dy < float64(thresh*thresh) {
			return true
		}
		drag.dragging = true
		s.hoverTabIdx = -1
		s.hoverPlus = false
		s.mgr.SetActive(drag.tabID)
	}
	drag.dx = x - int(drag.startX)
	edge := dpToPx(dragAutoScrollEdgeZoneDp, s.scale)
	if edge < 10 {
		edge = 10
	}
	if x < edge {
		s.tabScrollX -= dpToPx(dragAutoScrollStepDp, s.scale)
	} else if x > g.PlusX-edge {
		s.tabScrollX += dpToPx(dragAutoScrollStepDp, s.scale)
	}
	if s.tabScrollX < 0 {
		s.tabScrollX = 0
	}
	if s.tabScrollX > maxScroll {
		s.tabScrollX = maxScroll
	}
	dragLeft := drag.from*g.TabWidth - s.tabScrollX + drag.dx
	drag.over = chrome.DropIndexByOverlap(dragLeft, s.tabScrollX, g.TabWidth, len(tabs), drag.from, drag.over)
	s.app.RequestRedraw()
	return true
}

// handleTabBarRelease handles PointerUp/PointerCancel. A completed drag
// reorders the tab (if its drop index actually changed) and activates it;
// a plain click (press without a drag) just activates the pressed tab; a
// release with nothing pressed is not consumed.
func (s *uiState) handleTabBarRelease(ev gpucontext.PointerEvent) bool {
	drag := &s.tabDrag
	if !drag.pressed {
		*drag = tabDragState{}
		return false
	}
	wasDragging := drag.dragging
	id := drag.tabID
	from, over := drag.from, drag.over
	*drag = tabDragState{}
	switch {
	case wasDragging && ev.Type == gpucontext.PointerUp && from != over:
		s.mgr.MoveTo(id, over)
		s.mgr.SetActive(id)
	case !wasDragging && ev.Type == gpucontext.PointerUp:
		s.mgr.SetActive(id)
	}
	return true
}

func (s *uiState) handleTabBarScroll(ev gpucontext.ScrollEvent) {
	tabs := s.mgr.Tabs()
	if !s.tabBarShowTabs() {
		tabs = nil
	}
	minTabW := dpToPx(MinTabWidthDp, s.scale)
	plusW := dpToPx(PlusWidthDp, s.scale)
	closeZoneW := dpToPx(CloseZoneWidthDp, s.scale)
	g := chrome.ComputeGeometry(s.frameW, len(tabs), minTabW, plusW, closeZoneW)
	maxScroll := chrome.ScrollMax(g, len(tabs))
	if maxScroll == 0 {
		return
	}
	delta := tabStripScrollDelta(ev.DeltaX, ev.DeltaY, g.TabWidth)
	if delta == 0 {
		return
	}
	s.tabScrollX += delta
	if s.tabScrollX < 0 {
		s.tabScrollX = 0
	}
	if s.tabScrollX > maxScroll {
		s.tabScrollX = maxScroll
	}
	s.app.RequestRedraw()
}

// tabStripScrollDelta converts a scroll event into a pixel strip offset:
// prefer horizontal trackpad deltas; fall back to vertical wheel. Small
// wheel notches are amplified to half a tab so one click moves visibly.
func tabStripScrollDelta(deltaX, deltaY float64, tabW int) int {
	delta := int(math.Round(deltaX))
	if delta != 0 {
		return delta
	}
	delta = int(math.Round(deltaY))
	if delta == 0 || tabW <= 0 {
		return delta
	}
	half := tabW / 2
	if half < 1 {
		half = 1
	}
	if absInt(delta) < half {
		if delta > 0 {
			return half
		}
		return -half
	}
	return delta
}
