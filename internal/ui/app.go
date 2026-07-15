// Package ui hosts the gio window and event loop. It is the only package
// in geckty allowed to import gioui.org/*, keeping internal/session and
// internal/vt UI-toolkit-agnostic.
package ui

import (
	"bytes"
	"context"
	"image"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"golang.org/x/image/math/fixed"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/plugin"
	"github.com/geckty/geckty/internal/protocol/focus"
	"github.com/geckty/geckty/internal/protocol/mouse"
	"github.com/geckty/geckty/internal/protocol/paste"
	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
	"github.com/geckty/geckty/internal/ui/grid"
	"github.com/geckty/geckty/internal/ui/input"
	"github.com/geckty/geckty/internal/ui/theme"
)

// pluginStatusInterval is how often loaded plugins' draw_statusbar hook is
// called — its own independent cadence, not tied to frame painting or
// terminal dirty events (see loadPlugins' doc comment).
const pluginStatusInterval = time.Second

const clipboardMIME = "application/text"

// tag identifies the terminal surface as an event.Op target for keyboard
// focus/routing. A single package-level type is enough since all tabs
// share one window and one focus target — only the active session's PTY
// receives whatever's typed.
type tag struct{}

// Run opens the geckty window, spawns a shell session per cfg, and blocks
// running the gio event loop until the window closes.
func Run(cfg *config.Config) error {
	w := new(app.Window)
	w.Option(
		app.Title("geckty"),
		app.Size(unit.Dp(cfg.Window.Width), unit.Dp(cfg.Window.Height)),
	)

	palette, err := theme.NewPalette(cfg.Colors)
	if err != nil {
		return err
	}

	keymap, err := input.NewKeymap(cfg.Keybindings)
	if err != nil {
		return err
	}

	var mgr *session.Manager
	onManagerChange := func() {
		w.Invalidate()
		if mgr != nil && len(mgr.Tabs()) == 0 {
			w.Perform(system.ActionClose)
		}
	}
	mgr = session.NewManager(onManagerChange)

	const initialCols, initialRows = 80, 24
	cols, rows := initialCols, initialRows

	// Debounce PTY resize across tabs (see resizeDebouncer).
	resizeDebouncer := newResizeDebouncer(resizeDebounceDelay, func(cols, rows int) {
		for _, tb := range mgr.Tabs() {
			if err := tb.Session.Resize(cols, rows); err != nil {
				slog.Warn("session resize failed", slog.Any("error", err))
			}
		}
	})
	defer resizeDebouncer.Stop()

	homeDir, _ := os.UserHomeDir()
	newTab := func() error {
		_, err := mgr.New(session.Config{
			Command: cfg.ShellCommand(),
			Dir:     homeDir,
			Cols:    cols,
			Rows:    rows,
			OnDirty: w.Invalidate, // safe from the PTY read goroutine
		})
		return err
	}
	if err := newTab(); err != nil {
		return err
	}

	theme.SetMonospaceFamily(cfg.Font.Family)

	th := material.NewTheme()
	// System fonts stay enabled; gofont is a last-resort collection so a
	// missing Cascadia/Consolas never falls into a proportional face.
	shaper := text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Shaper = shaper

	painter := &grid.Painter{
		Theme:    th,
		Palette:  palette,
		FontSize: unit.Sp(cfg.Font.Size),
	}
	tabBar := chrome.NewTabBar()

	pluginHost, stopPlugins := loadPlugins(cfg, w)
	defer stopPlugins()

	t := new(tag)
	requestedFocus := false
	var lastPxPerEm int
	windowFocused := true
	var scrollAccumPx float32
	var tabDrag tabDragState
	var tabScrollX int
	hoverTabIdx := -1
	var scrollBarUntil time.Time
	var scrollBarHide *time.Timer
	const scrollBarHold = 1200 * time.Millisecond
	defer func() {
		if scrollBarHide != nil {
			scrollBarHide.Stop()
		}
	}()
	var blinkOn atomic.Bool
	blinkOn.Store(true)
	blinkDone := make(chan struct{})
	defer close(blinkDone)
	go func() {
		ticker := time.NewTicker(530 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				blinkOn.Store(!blinkOn.Load())
				w.Invalidate()
			case <-blinkDone:
				return
			}
		}
	}()

	for {
		e := w.Event()
		switch e := e.(type) {
		case app.DestroyEvent:
			for _, tb := range mgr.Tabs() {
				_ = tb.Session.Close()
			}
			return e.Err

		case app.ConfigEvent:
			if e.Config.Focused != windowFocused {
				windowFocused = e.Config.Focused
				if active := mgr.Active(); active != nil {
					active.Term.RLock()
					mode := active.Term.Mode()
					active.Term.RUnlock()
					if b := focus.Encode(mode, windowFocused); b != nil {
						_, _ = active.Write(b)
					}
				}
			}

		case app.FrameEvent:
			gtx := app.NewContext(new(op.Ops), e)

			// Window-sized event.Op for hit-testing; Y splits tab bar vs grid.
			hit := clip.Rect{Max: e.Size}.Push(gtx.Ops)
			event.Op(gtx.Ops, t)
			hit.Pop()
			key.InputHintOp{Tag: t, Hint: key.HintText}.Add(gtx.Ops)
			if !requestedFocus {
				gtx.Execute(key.FocusCmd{Tag: t})
				requestedFocus = true
			}

			chromeHeightPx := gtx.Dp(chrome.Height)
			padPx := gtx.Dp(grid.ContentPad)
			lineHeightPx := painter.LineHeightPx(gtx)
			showScrollBar := time.Now().Before(scrollBarUntil)

			pointerCtx := pointerContext{
				cellWidth:      painter.CellWidth,
				lineHeight:     lineHeightPx,
				chromeHeightPx: chromeHeightPx,
				padX:           padPx,
				padY:           padPx,
				wordChars:      cfg.Selection.WordChars,
				revealScrollBar: func() {
					scrollBarUntil = time.Now().Add(scrollBarHold)
					if scrollBarHide != nil {
						scrollBarHide.Stop()
					}
					scrollBarHide = time.AfterFunc(scrollBarHold+time.Millisecond, w.Invalidate)
				},
			}
			tabGeometry := chrome.ComputeGeometry(e.Size.X, len(mgr.Tabs()), gtx.Dp(chrome.MinTabWidth), gtx.Dp(chrome.PlusWidth), gtx.Dp(chrome.CloseZoneWidth))
			if max := chrome.ScrollMax(tabGeometry, len(mgr.Tabs())); tabScrollX > max {
				tabScrollX = max
			}
			if tabScrollX < 0 {
				tabScrollX = 0
			}

			filters := append(input.Filters(t), keymap.Filters(t)...)
			// ScrollX must be open: Gio clamps scroll deltas to the
			// filter ranges (zero ScrollX → horizontal trackpad = 0).
			filters = append(filters, pointer.Filter{
				Target:  t,
				Kinds:   pointer.Scroll | pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Cancel,
				ScrollX: pointer.ScrollRange{Min: -1 << 30, Max: 1 << 30},
				ScrollY: pointer.ScrollRange{Min: -1 << 30, Max: 1 << 30},
			})
			filters = append(filters, transfer.TargetFilter{Target: t, Type: clipboardMIME})
			for {
				ev, ok := gtx.Event(filters...)
				if !ok {
					break
				}
				switch ev := ev.(type) {
				case key.Event:
					if action, ok := keymap.Match(ev); ok {
						dispatchAction(gtx, t, mgr, newTab, action)
						break
					}
					// Fetched fresh, not cached from frame start: a
					// tab-switch action earlier in this same event
					// batch must route subsequent keystrokes to the
					// newly active tab, not a stale reference.
					if active := mgr.Active(); active != nil {
						writeKeyEvent(active, ev)
					}
				case key.EditEvent:
					// Ignore empty EditEvents (some platforms emit them
					// alongside modifier chords like Cmd+C/V — clearing
					// the selection or writing "" then broke copy/paste).
					if ev.Text == "" {
						break
					}
					if active := mgr.Active(); active != nil {
						_, _ = active.Write(input.EncodeText(ev.Text))
						active.ClearSelection()
					}
				case pointer.Event:
					// Tab gesture stays on the tab handler even outside
					// the strip (termizard). A stuck press that is cleared
					// as a miss returns handled=false so the same Press
					// can fall through to the terminal.
					inTabBar := ev.Position.Y < float32(chromeHeightPx)
					handled := false
					if tabDrag.pressed || inTabBar {
						handled = handleTabBarPointer(gtx, t, mgr, newTab, ev, tabGeometry, gtx.Dp(chrome.CloseZoneWidth), gtx.Dp(chrome.CloseEdgePad), &tabDrag, &tabScrollX, &hoverTabIdx)
					} else if hoverTabIdx >= 0 {
						hoverTabIdx = -1
						gtx.Execute(op.InvalidateCmd{})
					}
					if !handled && !inTabBar {
						if active := mgr.Active(); active != nil {
							handlePointer(active, ev, pointerCtx, &scrollAccumPx)
						}
					}
				case transfer.DataEvent:
					if active := mgr.Active(); active != nil {
						handlePasteData(active, ev)
					}
				}
			}

			// Cell width depends on DPI (PxPerEm). Remeasure when scaling
			// changes so Windows 125%/150% displays don't keep a stale width
			// (too-wide cells → too few ConPTY columns → path "…").
			pxPerEm := gtx.Sp(painter.FontSize)
			if painter.CellWidth <= 0 || pxPerEm != lastPxPerEm {
				painter.CellWidth = measureCellWidth(gtx, shaper, painter.FontSize)
				lastPxPerEm = pxPerEm
			}

			gridSizePx := image.Pt(e.Size.X-2*padPx, e.Size.Y-chromeHeightPx-2*padPx)
			newCols, newRows := gridSize(gridSizePx, painter.CellWidth, lineHeightPx)
			if newCols != cols || newRows != rows {
				cols, rows = newCols, newRows
				resizeDebouncer.Trigger(cols, rows)
			}

			// Drain any OSC 52 clipboard writes queued by a session's
			// PTY read goroutine (see session.Session.TakeClipboardWrite
			// and internal/session/osc52.go — clipboard.WriteCmd can
			// only be executed here, from the UI goroutine during frame
			// processing). Checked for every tab, not just the active
			// one: a background tab legitimately setting the clipboard
			// (e.g. a long-running build finishing) shouldn't need to
			// be focused first.
			for _, tb := range mgr.Tabs() {
				if data, ok := tb.Session.TakeClipboardWrite(); ok {
					gtx.Execute(clipboard.WriteCmd{Type: clipboardMIME, Data: io.NopCloser(bytes.NewReader(data))})
				}
			}

			gtx.Constraints = layout.Exact(e.Size)
			// Fill client area so ContentPad doesn't show Gio's clear color.
			paint.FillShape(gtx.Ops, palette.Background, clip.Rect{Max: e.Size}.Op())
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					drag := chrome.DragVisual{
						Active:   tabDrag.dragging,
						From:     tabDrag.from,
						Over:     tabDrag.over,
						DX:       tabDrag.dx,
						ScrollX:  tabScrollX,
						TabID:    tabDrag.tabID,
						HoverIdx: hoverTabIdx,
					}
					return tabBar.Layout(gtx, th, palette, mgr.Tabs(), mgr.ActiveID(), pluginStatusText(pluginHost), drag)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					size := gtx.Constraints.Max
					paint.FillShape(gtx.Ops, palette.Background, clip.Rect{Max: size}.Op())
					active := mgr.Active()
					if active == nil {
						return layout.Dimensions{Size: size}
					}
					return layout.Inset{
						Top:    grid.ContentPad,
						Right:  grid.ContentPad,
						Bottom: grid.ContentPad,
						Left:   grid.ContentPad,
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return painter.Paint(gtx, active.Term, active.ScrollOffset(), gridSelection(active), gridPlacements(active), blinkOn.Load(), showScrollBar)
					})
				}),
			)

			e.Frame(gtx.Ops)
		}
	}
}

// loadPlugins loads cfg.Plugins (opt-in) and ticks status text at
// pluginStatusInterval, invalidating only when the text changes. Failures
// are logged and skipped. Returns a nil host when Plugins is empty.
func loadPlugins(cfg *config.Config, w *app.Window) (*plugin.Host, func()) {
	if len(cfg.Plugins) == 0 {
		return nil, func() {}
	}

	ctx := context.Background()
	h, err := plugin.NewHost(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "plugin host init failed", slog.Any("error", err))
		return nil, func() {}
	}

	for _, dir := range cfg.Plugins {
		p, err := h.Load(ctx, dir)
		if err != nil {
			slog.ErrorContext(ctx, "plugin load failed", "dir", dir, slog.Any("error", err))
			continue
		}
		if err := p.Activate(ctx); err != nil {
			slog.ErrorContext(ctx, "plugin activate failed", "plugin", p.Name, slog.Any("error", err))
		}
		// Draw once synchronously so the first frame already has
		// something to show instead of waiting a full
		// pluginStatusInterval for the ticker's first tick.
		if err := p.DrawStatusbar(ctx); err != nil {
			slog.ErrorContext(ctx, "plugin draw_statusbar failed", "plugin", p.Name, slog.Any("error", err))
		}
	}

	ticker := time.NewTicker(pluginStatusInterval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				changed := false
				for _, p := range h.Plugins() {
					before := p.StatusText()
					if err := p.DrawStatusbar(ctx); err != nil {
						continue
					}
					if p.StatusText() != before {
						changed = true
					}
				}
				if changed {
					w.Invalidate()
				}
			case <-done:
				return
			}
		}
	}()

	stop := func() {
		ticker.Stop()
		close(done)
		_ = h.Close(ctx)
	}
	return h, stop
}

// pluginStatusText joins every loaded plugin's current StatusText (see
// internal/plugin.Plugin.StatusText) with a separator, skipping any that
// have nothing to show (no draw_statusbar export, or it never called
// statusbar_draw). h may be nil (no plugins configured), in which case
// the result is always "".
func pluginStatusText(h *plugin.Host) string {
	if h == nil {
		return ""
	}
	var parts []string
	for _, p := range h.Plugins() {
		if text := p.StatusText(); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "   ")
}

// writeKeyEvent encodes ev and writes it to sess, trying the Kitty
// keyboard protocol first (see internal/protocol/kittykbd) and falling
// back to the legacy encoder — EncodeKitty declines (ok=false) whenever no
// Kitty flag is active, the key isn't ambiguous, or the specific
// flag/key combination isn't implemented, in which case legacy encoding
// is exactly as correct as it always was.
func writeKeyEvent(sess *session.Session, ev key.Event) {
	sess.Term.RLock()
	keyState := sess.Term.KeyState()
	sess.Term.RUnlock()

	if b, ok := input.EncodeKitty(keyState, ev); ok {
		_, _ = sess.Write(b)
		return
	}
	if b, ok := input.Encode(ev); ok {
		_, _ = sess.Write(b)
	}
}

// dispatchAction applies a keymap-matched action: tab management, or
// clipboard copy/paste. Copy reads the active session's selection and
// writes it to the OS clipboard directly; paste only *requests* the
// clipboard contents here (clipboard.ReadCmd is async) — the actual write
// to the shell happens later in handlePasteData, when the resulting
// transfer.DataEvent arrives.
func dispatchAction(gtx layout.Context, t event.Tag, mgr *session.Manager, newTab func() error, action input.Action) {
	switch action {
	case input.ActionNewTab:
		_ = newTab()
	case input.ActionCloseTab:
		_ = mgr.CloseActive()
	case input.ActionNextTab:
		mgr.Next()
	case input.ActionPrevTab:
		mgr.Prev()
	case input.ActionCopy:
		if active := mgr.Active(); active != nil {
			if text, ok := active.SelectedText(); ok && text != "" {
				gtx.Execute(clipboard.WriteCmd{Type: clipboardMIME, Data: io.NopCloser(strings.NewReader(text))})
			}
		}
	case input.ActionPaste:
		gtx.Execute(clipboard.ReadCmd{Tag: t})
	}
}

// handlePasteData completes a paste requested by dispatchAction: reads the
// clipboard content transfer.DataEvent delivered it, wraps it per
// bracketed-paste mode if the shell asked for that (see
// internal/protocol/paste), and writes it to sess.
func handlePasteData(sess *session.Session, ev transfer.DataEvent) {
	r := ev.Open()
	defer func() { _ = r.Close() }()
	content, err := io.ReadAll(r)
	if err != nil {
		return
	}

	sess.Term.RLock()
	mode := sess.Term.Mode()
	sess.Term.RUnlock()

	_, _ = sess.Write(paste.Encode(mode, string(content)))
}

// gridSelection converts sess's current selection (if any) into the
// grid.Selection shape Painter.Paint expects.
func gridSelection(sess *session.Session) grid.Selection {
	start, end, ok := sess.Selection()
	if !ok {
		return grid.Selection{}
	}
	sel := grid.Selection{Active: true}
	sel.Start.Col, sel.Start.Row = start.Col, start.Row
	sel.End.Col, sel.End.Row = end.Col, end.Row
	return sel
}

// gridPlacements converts sess's currently decoded Kitty-graphics images
// (see session.Session.Placements) into the grid.Placement shape
// Painter.Paint expects.
func gridPlacements(sess *session.Session) []grid.Placement {
	placements := sess.Placements()
	if len(placements) == 0 {
		return nil
	}
	out := make([]grid.Placement, len(placements))
	for i, p := range placements {
		out[i] = grid.Placement{
			Seq:     p.Seq,
			Image:   p.Image,
			AbsLine: p.AbsLine,
			Col:     p.Col,
			Cols:    p.Cols,
			Rows:    p.Rows,
		}
	}
	return out
}

// measureCellWidth shapes a representative monospace glyph to derive the
// fixed advance in device pixels. Prefer Floor over Ceil — Ceil inflated
// Windows DPI cells so ConPTY got too few columns and PowerShell used "…".
func measureCellWidth(gtx layout.Context, shaper *text.Shaper, fontSize unit.Sp) int {
	shaper.LayoutString(text.Parameters{
		Font:    font.Font{Typeface: theme.MonospaceTypeface},
		PxPerEm: fixed.I(gtx.Sp(fontSize)),
	}, "M")
	if g, ok := shaper.NextGlyph(); ok {
		w := g.Advance.Floor()
		if w < 1 {
			return 1
		}
		return w
	}
	return gtx.Sp(fontSize)
}

// tabDragThresholdDp is how far the pointer must move from the press
// point before a tab press becomes a drag-reorder (termizard uses 6
// logical px; we convert via gtx.Dp so Retina scales match).
const tabDragThresholdDp = unit.Dp(6)

// tabDragState tracks an in-progress tab press / drag-reorder across
// frames — see handleTabBarPointer. Zero value means "no gesture".
//
// Unlike an earlier version that called MoveTo on every slot cross
// (jumpy, no visual feedback), drag is preview-only until release:
// DX follows the pointer, Over tracks the drop slot, and MoveTo commits
// once — matching termizard's dragDX / dragOverIdx / reorderTab flow.
type tabDragState struct {
	pressed  bool
	dragging bool
	tabID    int
	from     int
	over     int
	dx       int
	startX   float32
	startY   float32
	pointer  pointer.ID
}

// handleTabBarPointer handles a pointer event for the tab strip —
// select, close, new-tab, or drag-reorder preview. Returns whether the
// event was consumed (so a cleared miss can fall through to the
// terminal). GrabCmd keeps the gesture alive when the pointer leaves
// the strip.
func handleTabBarPointer(gtx layout.Context, tag event.Tag, mgr *session.Manager, newTab func() error, ev pointer.Event, g chrome.Geometry, closeZoneWidthPx, closeEdgePadPx int, drag *tabDragState, scrollX *int, hoverIdx *int) bool {
	if drag.pressed && ev.Kind != pointer.Press && ev.PointerID != drag.pointer {
		return true
	}
	x := int(ev.Position.X)
	thresh := gtx.Dp(tabDragThresholdDp)
	if thresh < 1 {
		thresh = 1
	}
	maxScroll := chrome.ScrollMax(g, len(mgr.Tabs()))
	if *scrollX < 0 {
		*scrollX = 0
	}
	if *scrollX > maxScroll {
		*scrollX = maxScroll
	}
	tabs := mgr.Tabs()
	activeIdx := -1
	if activeID := mgr.ActiveID(); activeID >= 0 {
		for i := range tabs {
			if tabs[i].ID == activeID {
				activeIdx = i
				break
			}
		}
	}

	switch ev.Kind {
	case pointer.Scroll:
		if maxScroll == 0 {
			return false
		}
		// termizard: trackpads send Scroll.X; mice send vertical wheel —
		// treat both as strip scroll. One notch ≈ half a compact tab.
		delta := tabStripScrollDelta(ev.Scroll.X, ev.Scroll.Y, g.TabWidth)
		if delta == 0 {
			return true
		}
		*scrollX += delta
		if *scrollX < 0 {
			*scrollX = 0
		}
		if *scrollX > maxScroll {
			*scrollX = maxScroll
		}
		gtx.Execute(op.InvalidateCmd{})
		return true
	case pointer.Press:
		if !ev.Buttons.Contain(pointer.ButtonPrimary) {
			return false
		}
		if chrome.IsPlusHit(x, g) {
			*drag = tabDragState{}
			*hoverIdx = -1
			_ = newTab()
			return true
		}
		idx := chrome.TabAtScrolledPinned(x, *scrollX, g, len(tabs), activeIdx)
		if idx < 0 {
			// Miss (gap / outside cluster): clear a stuck press and
			// report unhandled so the terminal can take this Press.
			*drag = tabDragState{}
			*hoverIdx = -1
			return false
		}
		id := tabs[idx].ID
		tabLeft := chrome.TabVisualLeft(idx, activeIdx, *scrollX, g, len(tabs))
		localX := x - tabLeft
		// Close only when the × is showing (that tab is hovered).
		if *hoverIdx == idx && chrome.IsCloseHit(localX, g, closeZoneWidthPx, closeEdgePadPx) {
			*drag = tabDragState{}
			*hoverIdx = -1
			_ = mgr.Close(id)
			return true
		}
		*hoverIdx = -1
		*drag = tabDragState{
			pressed: true,
			tabID:   id,
			from:    idx,
			over:    idx,
			startX:  ev.Position.X,
			startY:  ev.Position.Y,
			pointer: ev.PointerID,
		}
		gtx.Execute(pointer.GrabCmd{Tag: tag, ID: ev.PointerID})
		return true

	case pointer.Move, pointer.Drag:
		if !drag.pressed {
			// Track any tab (including active) so the close × can appear.
			idx := chrome.TabAtScrolledPinned(x, *scrollX, g, len(tabs), activeIdx)
			if *hoverIdx != idx {
				*hoverIdx = idx
				gtx.Execute(op.InvalidateCmd{})
			}
			return idx >= 0 || chrome.IsPlusHit(x, g)
		}
		if !drag.dragging {
			dx := ev.Position.X - drag.startX
			dy := ev.Position.Y - drag.startY
			if dx*dx+dy*dy < float32(thresh*thresh) {
				return true
			}
			drag.dragging = true
			*hoverIdx = -1
			mgr.SetActive(drag.tabID)
		}
		drag.dx = int(ev.Position.X - drag.startX)
		// Edge auto-scroll while dragging (termizard behavior).
		edge := gtx.Dp(unit.Dp(40))
		if edge < 10 {
			edge = 10
		}
		if x < edge {
			*scrollX -= gtx.Dp(unit.Dp(8))
		} else if x > g.PlusX-edge {
			*scrollX += gtx.Dp(unit.Dp(8))
		}
		if *scrollX < 0 {
			*scrollX = 0
		}
		if *scrollX > maxScroll {
			*scrollX = maxScroll
		}
		dragLeft := drag.from*g.TabWidth - *scrollX + drag.dx
		drag.over = chrome.DropIndexByOverlap(dragLeft, *scrollX, g.TabWidth, len(tabs), drag.from, drag.over)
		gtx.Execute(op.InvalidateCmd{})
		return true

	case pointer.Release, pointer.Cancel:
		if !drag.pressed {
			*drag = tabDragState{}
			return false
		}
		wasDragging := drag.dragging
		id := drag.tabID
		from, over := drag.from, drag.over
		*drag = tabDragState{}
		switch {
		case wasDragging && ev.Kind == pointer.Release && from != over:
			mgr.MoveTo(id, over)
			mgr.SetActive(id)
		case !wasDragging && ev.Kind == pointer.Release:
			mgr.SetActive(id)
		}
		return true
	}
	return false
}

// tabStripScrollDelta converts a pointer scroll into a pixel strip offset
// (termizard handleScroll): prefer horizontal trackpad deltas; fall back to
// vertical wheel. Small wheel notches are amplified to half a tab so one
// click moves visibly — trackpad X is left unamplified so continuous
// gestures stay smooth.
func tabStripScrollDelta(scrollX, scrollY float32, tabW int) int {
	delta := int(math.Round(float64(scrollX)))
	if delta != 0 {
		return delta
	}
	delta = int(math.Round(float64(scrollY)))
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

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// pointerContext bundles the per-frame geometry and config that
// handleScroll/handleButton/handleDrag need to convert a pointer event
// into a terminal cell and decide what gesture applies — built once per
// frame (see the FrameEvent case) since none of it varies between the
// pointer events one frame might carry.
type pointerContext struct {
	cellWidth      int
	lineHeight     int
	chromeHeightPx int
	padX, padY     int
	// wordChars is config.SelectionConfig.WordChars, threaded through to
	// session.Session.SelectWord for double-click word selection.
	wordChars string
	// revealScrollBar shows the transient scrollback scrollbar.
	revealScrollBar func()
}

// handlePointer dispatches one pointer event to scroll, button-report, or
// selection handling, whichever applies.
func handlePointer(sess *session.Session, ev pointer.Event, pc pointerContext, accumPx *float32) {
	if pc.cellWidth <= 0 || pc.lineHeight <= 0 {
		return
	}

	switch ev.Kind {
	case pointer.Scroll:
		handleScroll(sess, ev, pc, accumPx)
	case pointer.Press:
		handleButton(sess, ev, pc, true)
	case pointer.Release:
		handleButton(sess, ev, pc, false)
	case pointer.Drag:
		handleDrag(sess, ev, pc)
	}
}

// cellFromPosition converts a pointer position from window pixel space
// (event.Op's hit-region here is the whole window, not just the grid) into
// a 0-based (col, row) cell, subtracting the tab bar's height first —
// without this offset every mouse-reported/selected row would be off by
// however many rows chrome.Height occupies. Clamped to [0, cols-1] and
// [0, rows-1]: a pointer position past the grid's right or bottom edge is
// clamped to the nearest valid cell rather than left out of range.
//
// The upper clamp matters more than it looks: a plain drag very
// frequently ends a pixel or two past the last row or column (nobody
// stops a mouse drag at an exact pixel boundary), and an unclamped
// out-of-range row/col silently dropped the last row of a selection from
// SelectedText — a real, reported bug ("cuts off characters") — since
// session.Session.SelectedText skips any row outside [0, rows).
func cellFromPosition(pos f32.Point, cellWidth, lineHeight, chromeHeightPx, padX, padY, cols, rows int) (col, row int) {
	col = (int(pos.X) - padX) / cellWidth
	row = (int(pos.Y) - chromeHeightPx - padY) / lineHeight
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

// handleScroll sends wheel escapes when mouse tracking is on, otherwise
// scrolls scrollback. accumPx keeps fractional trackpad remainders.
func handleScroll(sess *session.Session, ev pointer.Event, pc pointerContext, accumPx *float32) {
	lines := accumulateScrollLines(ev.Scroll.Y, pc.lineHeight, accumPx)
	if lines == 0 {
		return
	}

	sess.Term.RLock()
	mode := sess.Term.Mode()
	sz := sess.Term.Size()
	sess.Term.RUnlock()

	if !mouse.TrackingEnabled(mode) {
		// Negative: scrolling down (positive lines) moves toward the
		// live bottom, i.e. decreases the scrollback offset.
		sess.ScrollBy(-lines)
		if pc.revealScrollBar != nil {
			pc.revealScrollBar()
		}
		return
	}

	dir := mouse.Up
	if lines > 0 {
		dir = mouse.Down
	}
	col, row := cellFromPosition(ev.Position, pc.cellWidth, pc.lineHeight, pc.chromeHeightPx, pc.padX, pc.padY, sz.C, sz.R)
	n := lines
	if n < 0 {
		n = -n
	}
	for i := 0; i < n; i++ {
		// SGR/legacy mouse coordinates are 1-based.
		if b, ok := mouse.EncodeWheel(mode, dir, col+1, row+1, 0); ok {
			_, _ = sess.Write(b)
		}
	}
}

// handleButton reports a left-button press/release to the shell if mouse
// tracking is enabled, or drives local text selection otherwise: pressing
// starts a selection (or, on a double-click within the same cell, selects
// the whole word there — see session.Session.SelectWord), releasing ends
// the drag gesture.
//
// Known gap: only the primary (left) button is reported to the shell or
// used for selection at all right now — middle/right clicks are silently
// ignored rather than reported as mouse.ButtonMiddle/ButtonRight (the
// encoder already supports both; app.go just doesn't route ev.Buttons
// through yet) or given their usual platform behavior (paste-from-
// selection on middle-click, a context menu on right-click).
func handleButton(sess *session.Session, ev pointer.Event, pc pointerContext, pressed bool) {
	sess.Term.RLock()
	mode := sess.Term.Mode()
	sz := sess.Term.Size()
	sess.Term.RUnlock()

	col, row := cellFromPosition(ev.Position, pc.cellWidth, pc.lineHeight, pc.chromeHeightPx, pc.padX, pc.padY, sz.C, sz.R)

	if mouse.TrackingEnabled(mode) {
		if b, ok := mouse.EncodeButton(mode, mouse.ButtonLeft, pressed, col+1, row+1, 0); ok {
			_, _ = sess.Write(b)
		}
		return
	}

	if !ev.Buttons.Contain(pointer.ButtonPrimary) && pressed {
		return
	}
	if pressed {
		if sess.RegisterClick(col, row) {
			sess.SelectWord(col, row, pc.wordChars)
		} else {
			sess.StartSelection(col, row)
		}
	} else {
		sess.EndSelection()
	}
}

// handleDrag reports mouse motion (while a button is held) to the shell if
// the shell asked for drag reporting, or extends the local text selection
// otherwise.
func handleDrag(sess *session.Session, ev pointer.Event, pc pointerContext) {
	sess.Term.RLock()
	mode := sess.Term.Mode()
	sz := sess.Term.Size()
	sess.Term.RUnlock()

	col, row := cellFromPosition(ev.Position, pc.cellWidth, pc.lineHeight, pc.chromeHeightPx, pc.padX, pc.padY, sz.C, sz.R)

	if b, ok := mouse.EncodeMotion(mode, mouse.ButtonLeft, col+1, row+1, 0); ok {
		_, _ = sess.Write(b)
		return
	}
	if !mouse.TrackingEnabled(mode) {
		sess.ExtendSelection(col, row)
	}
}

// accumulateScrollLines converts a scroll delta in pixels into a whole
// line count, carrying the fractional remainder in accumPx across calls so
// trackpad deltas (many small events per line) aren't rounded away one
// event at a time.
func accumulateScrollLines(deltaPx float32, lineHeight int, accumPx *float32) int {
	if lineHeight <= 0 {
		return 0
	}
	*accumPx += deltaPx
	lines := int(*accumPx) / lineHeight
	*accumPx -= float32(lines * lineHeight)
	return lines
}

// gridSize converts a window size in pixels into a terminal column/row
// count, given the measured monospace cell dimensions. Always returns at
// least 1x1 so a not-yet-measured or degenerate cell size can't produce a
// zero-size terminal.
func gridSize(size image.Point, cellWidth, lineHeight int) (cols, rows int) {
	if cellWidth <= 0 || lineHeight <= 0 {
		return 1, 1
	}
	cols = size.X / cellWidth
	rows = size.Y / lineHeight
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return cols, rows
}
