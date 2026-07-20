// Package gogpu hosts geckty's window and event loop on top of
// github.com/gogpu/gogpu + github.com/gogpu/gpucontext, replacing the
// previous gioui.org-based internal/ui package. The terminal grid and tab
// bar are rendered as direct RGBA pixel writes into a persistent per-
// window buffer (see painter.go/tabbar.go/buffer.go), uploaded as one GPU
// texture per dirty frame — mirroring the architecture already proven on
// Windows by the sibling termizard project. No GPU clip primitive is ever
// used for glyph/cell/chrome painting: clipping is bounds-checked pixel
// math instead, which is what makes this renderer immune to the gio
// D3D11 per-cell-clip corruption bug that motivated this migration.
package gogpu

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	gogpulib "github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/plugin"
	"github.com/geckty/geckty/internal/protocol/focus"
	"github.com/geckty/geckty/internal/protocol/paste"
	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
	"github.com/geckty/geckty/internal/ui/theme"
)

// pluginStatusInterval is how often loaded plugins' draw_statusbar hook is
// called — its own independent cadence, not tied to frame painting or
// terminal dirty events (see loadPlugins' doc comment).
const pluginStatusInterval = time.Second

// uiState holds everything the draw/input callbacks need — the gogpu
// equivalent of the local variables closed over by the old gio Run()'s
// for-loop, promoted to a struct since gogpu's callback-registration API
// needs each handler as its own function value rather than one big loop.
type uiState struct {
	cfg     *config.Config
	app     gpuApp
	win     gpuWindow
	keymap  *Keymap
	palette theme.Palette
	painter *Painter
	tabBar  *TabBar
	mgr     *session.Manager
	newTab  func() error

	resizeDebouncer *resizeDebouncer
	pluginHost      *plugin.Host
	stopPlugins     func()

	cols, rows int

	blinkOn atomic.Bool

	// Per-window render state.
	frame          []byte
	frameW, frameH int
	tex            *gogpulib.Texture

	// Font/cell metrics, refreshed in onDraw when scale or configured size
	// changes (mirrors the old gio code's cellWidthMeasured-once-per-DPI
	// logic, generalized to also cover the tab bar's own smaller font).
	scale             float64
	fontSizeCurrent   float64
	cellW, cellH, asc int

	// Tab-bar interaction state — same shape as the old gio app.go's
	// tabDragState/tabScrollX/hoverTabIdx/scrollBarUntil.
	tabDrag        tabDragState
	tabScrollX     int
	hoverTabIdx    int
	hoverPlus      bool
	scrollBarUntil time.Time

	scrollAccumPx float64

	// keyHandled suppresses the text-input callback that (on some
	// platforms) still fires after a shortcut key press was consumed —
	// same purpose as termizard's identically-named flag.
	keyHandled bool

	// perWindowKeyPressed / perWindowTextInputed dedupe the per-window
	// SetOnKeyPress/SetOnTextInput callbacks (wireWindow) against the
	// EventSource-based ones (wireEventSource): on Linux, gogpu reports
	// WindowID=0 for keyboard events, so the per-window callbacks are
	// silently never invoked there — EventSource is the only path that
	// actually fires on Linux. On Windows/macOS the per-window callbacks
	// do fire, and each sets its own flag so the EventSource-side handler
	// skips re-dispatching the same event. Separate flags (not one shared
	// flag) because SetOnKeyPress fires before the following WM_CHAR/
	// TextInput on some platforms — sharing a flag would cause the
	// EventSource side to wrongly swallow that following character. Same
	// fix termizard already shipped for this exact gogpu behavior.
	perWindowKeyPressed  bool
	perWindowTextInputed bool

	// pendingTexSync / inLiveResize implement the Windows live-resize
	// freeze: repainting a full frame on every WM_SIZE tick during a drag
	// is expensive and was the source of a text-truncation-looking bug in
	// the old renderer (stale cols/rows briefly visible mid-drag). While
	// InSizeMove() is true, keep presenting the last good texture
	// unchanged; once it ends, do one full repaint at the final size and
	// only then tell the PTY about the new size.
	pendingTexSync bool
}

// Backend implements the ui.Backend interface (internal/ui/backend.go) by
// delegating to Run — see that interface's doc comment for why cmd/geckty/
// main.go routes through it rather than calling Run directly.
type Backend struct{}

// Run implements ui.Backend.
func (Backend) Run(cfg *config.Config) error {
	return Run(cfg)
}

// Run opens the geckty window, spawns a shell session per cfg, and blocks
// running the gogpu event loop until the window closes. Unlike the old
// gio-based Run, this itself drives the platform event pump (gogpu.App.Run
// is both "start the toolkit's main loop" and "run this session's frame/
// input loop" in one call) — callers should invoke it directly on the
// main goroutine, not via a separate app.Main()-style call.
func Run(cfg *config.Config) error {
	palette, err := theme.NewPalette(cfg.Colors)
	if err != nil {
		return err
	}
	keymap, err := NewKeymap(cfg.Keybindings)
	if err != nil {
		return err
	}

	s, gapp := buildUIState(cfg, palette, keymap)

	s.wireSessionManager(gapp)
	defer s.resizeDebouncer.Stop()

	if err := s.wireFirstTab(cfg, gapp); err != nil {
		return err
	}

	s.pluginHost, s.stopPlugins = loadPlugins(cfg, gapp)
	defer s.stopPlugins()

	stopBlink := s.startBlinkLoop(gapp)
	defer stopBlink()

	s.wireLifecycleCallbacks(gapp)
	s.wireEventSource(gapp)

	return gapp.Run()
}

// buildUIState constructs the gogpu App and its uiState — the fields that
// don't depend on session/plugin/blink wiring, which are set up separately
// by wireSessionManager/wireFirstTab/startBlinkLoop/wireLifecycleCallbacks
// so Run itself reads as a short sequence of named setup steps rather than
// one long function.
func buildUIState(cfg *config.Config, palette theme.Palette, keymap *Keymap) (*uiState, gpuApp) {
	winW, winH := cfg.Window.Width, cfg.Window.Height
	if winW <= 0 {
		winW = 1200
	}
	if winH <= 0 {
		winH = 800
	}

	appCfg := gogpulib.DefaultConfig().
		WithTitle("geckty").
		WithSize(winW, winH).
		WithResizable(true).
		WithVSync(true).
		// On Windows, RequestRedraw called from a goroutine is dropped by
		// the Win32 message pump between frames (the same issue termizard's
		// gogpu backend documents) — continuous render is the workaround.
		// Re-verify against future gogpu releases.
		WithContinuousRender(runtime.GOOS == osWindows)

	gapp := gogpulib.NewApp(appCfg)
	s := &uiState{
		cfg:         cfg,
		app:         gapp,
		keymap:      keymap,
		palette:     palette,
		painter:     &Painter{Palette: palette},
		tabBar:      NewTabBar(),
		cols:        80,
		rows:        24,
		hoverTabIdx: -1,
	}
	s.blinkOn.Store(true)
	return s, gapp
}

// wireSessionManager creates s.mgr and s.resizeDebouncer. The manager's
// close callback requests a redraw and quits the app once the last tab
// closes; the debouncer coalesces PTY/emu resizes across every open tab
// (see resizeDebouncer's own doc comment for why per-frame resize would be
// too expensive to apply directly).
func (s *uiState) wireSessionManager(gapp gpuApp) {
	s.mgr = session.NewManager(func() {
		gapp.RequestRedraw()
		if s.mgr != nil && len(s.mgr.Tabs()) == 0 {
			gapp.Quit()
		}
	})
	s.resizeDebouncer = newResizeDebouncer(resizeDebounceDelay, func(cols, rows int) {
		for _, tb := range s.mgr.Tabs() {
			if err := tb.Session.Resize(cols, rows); err != nil {
				slog.Warn("session resize failed", slog.Any("error", err))
			}
		}
	})
}

// wireFirstTab sets s.newTab (spawning cfg.ShellCommand() in the user's
// home directory at the current grid size) and opens the first tab,
// returning any error from that initial spawn.
func (s *uiState) wireFirstTab(cfg *config.Config, gapp gpuApp) error {
	homeDir, _ := os.UserHomeDir()
	s.newTab = func() error {
		_, err := s.mgr.New(session.Config{
			Command: cfg.ShellCommand(),
			Dir:     homeDir,
			Cols:    s.cols,
			Rows:    s.rows,
			OnDirty: gapp.RequestRedraw,
		})
		return err
	}
	return s.newTab()
}

// startBlinkLoop starts the cursor-blink ticker goroutine and returns a
// stop func that terminates it — mirrors loadPlugins' stopPlugins pattern
// so Run's shutdown sequence reads the same way for both.
func (s *uiState) startBlinkLoop(gapp gpuApp) (stop func()) {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(530 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.blinkOn.Store(!s.blinkOn.Load())
				gapp.RequestRedraw()
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

// wireLifecycleCallbacks registers the app-level focus/close/surface-ready
// callbacks: forward focus-mode changes to the active session, close every
// tab's session on window close, and wire the primary window's own event
// callbacks (see wireWindow) once its GPU surface is ready.
func (s *uiState) wireLifecycleCallbacks(gapp gpuApp) {
	gapp.OnFocus(func(focused bool) {
		if active := s.mgr.Active(); active != nil {
			active.Term.RLock()
			mode := active.Term.Mode()
			active.Term.RUnlock()
			if b := focus.Encode(mode, focused); b != nil {
				_, _ = active.Write(b)
			}
		}
	})
	gapp.OnClose(func() {
		for _, tb := range s.mgr.Tabs() {
			_ = tb.Session.Close()
		}
	})
	gapp.OnSurfaceAvailable(func() {
		win := gapp.PrimaryWindow()
		if win == nil {
			return
		}
		s.wireWindow(win)
	})
}

func (s *uiState) wireWindow(win gpuWindow) {
	s.win = win
	win.SetOnDraw(s.onDraw)
	win.SetOnResize(func(_, _ int) { s.app.RequestRedraw() })
	win.SetOnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		s.perWindowKeyPressed = true
		s.handleKeyPress(key, mods)
	})
	win.SetOnTextInput(func(text string) {
		s.perWindowTextInputed = true
		s.handleTextInput(text)
	})
	win.SetOnPointer(s.handlePointerEvent)
	win.SetOnScroll(s.handleScrollEvent)
	win.SetOnClose(func() bool { return true })
}

// wireEventSource registers keyboard handling on gapp's global EventSource
// in addition to wireWindow's per-window callbacks, and — on Linux only —
// pointer/scroll handling too. This works around a real gogpu behavior
// (confirmed by termizard's own field experience against the same
// library): on Linux, keyboard/pointer/scroll events carry WindowID=0, so
// the per-window SetOnKeyPress/SetOnTextInput/SetOnPointer/SetOnScroll
// callbacks registered in wireWindow are silently never invoked there —
// EventSource dispatches unconditionally regardless of WindowID and is the
// only input path that actually works on Linux. On Windows/macOS, the
// per-window callbacks fire first and set the perWindowKeyPressed/
// perWindowTextInputed flags so this function's keyboard handlers skip
// re-dispatching the same event; pointer/scroll are deliberately NOT also
// routed through EventSource on those platforms, since the per-window
// callbacks already work there and doing both would double-dispatch.
func (s *uiState) wireEventSource(gapp gpuApp) {
	src := gapp.EventSource()
	src.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		if s.perWindowKeyPressed {
			s.perWindowKeyPressed = false
			return
		}
		s.handleKeyPress(key, mods)
	})
	src.OnTextInput(func(text string) {
		if s.perWindowTextInputed {
			s.perWindowTextInputed = false
			return
		}
		s.handleTextInput(text)
	})
	if runtime.GOOS != "linux" {
		return
	}
	if psrc, ok := src.(gpucontext.PointerEventSource); ok {
		psrc.OnPointer(s.handlePointerEvent)
	}
	if ssrc, ok := src.(gpucontext.ScrollEventSource); ok {
		ssrc.OnScrollEvent(s.handleScrollEvent)
	}
}

// handleKeyPress matches a shortcut first, then falls through to Kitty/
// legacy encoding for the active session — the same precedence as the old
// gio code's key.Event case, adapted to gpucontext's split key/text-input
// callbacks (see keyHandled's doc comment).
func (s *uiState) handleKeyPress(key gpucontext.Key, mods gpucontext.Modifiers) {
	s.keyHandled = false
	if action, ok := s.keymap.Match(key, mods); ok {
		s.dispatchAction(action)
		s.keyHandled = true
		return
	}
	active := s.mgr.Active()
	if active == nil {
		return
	}
	active.Term.RLock()
	keyState := active.Term.KeyState()
	active.Term.RUnlock()
	if b, ok := EncodeKitty(keyState, key, mods, true); ok {
		_, _ = active.Write(b)
		s.keyHandled = true
		return
	}
	if b, ok := EncodeKey(key, mods); ok {
		_, _ = active.Write(b)
		s.keyHandled = true
	}
}

func (s *uiState) handleTextInput(text string) {
	if s.keyHandled {
		s.keyHandled = false
		return
	}
	if text == "" {
		return
	}
	active := s.mgr.Active()
	if active == nil {
		return
	}
	_, _ = active.Write(EncodeText(text))
	active.ClearSelection()
}

// dispatchAction applies a keymap-matched action. Copy/paste are
// synchronous here (unlike gio's clipboard.WriteCmd/ReadCmd + async
// transfer.DataEvent round trip) since gogpu's clipboard calls return
// directly — see clipboard.go.
func (s *uiState) dispatchAction(action Action) {
	switch action {
	case ActionNewTab:
		_ = s.newTab()
	case ActionCloseTab:
		_ = s.mgr.CloseActive()
	case ActionNextTab:
		s.mgr.Next()
	case ActionPrevTab:
		s.mgr.Prev()
	case ActionCopy:
		if active := s.mgr.Active(); active != nil {
			if text, ok := active.SelectedText(); ok && text != "" {
				_ = clipboardWrite(s.app, text)
			}
		}
	case ActionPaste:
		if active := s.mgr.Active(); active != nil {
			if text, err := clipboardRead(s.app); err == nil && text != "" {
				active.Term.RLock()
				mode := active.Term.Mode()
				active.Term.RUnlock()
				_, _ = active.Write(paste.Encode(mode, text))
			}
		}
	}
}

// onDraw is the gogpu FrameEvent equivalent: measure fonts/cell metrics,
// lay out chrome + grid, paint into the persistent frame buffer, and
// present it as a texture.
func (s *uiState) onDraw(ctx *gogpulib.Context) {
	scale := ctx.ScaleFactor()
	s.ensureFonts(scale)

	fw, fh := ctx.FramebufferSize()
	if fw <= 0 || fh <= 0 || s.cellW <= 0 || s.cellH <= 0 {
		s.app.RequestRedraw()
		return
	}

	inLiveResize := s.app.InSizeMove()
	if s.syncLiveResizeFreeze(ctx, inLiveResize) {
		return
	}
	needFinalSync := s.consumeLiveResizeSync(inLiveResize)

	tabBarH := dpToPx(TabBarHeightDp, scale)
	padPx := dpToPx(chromeContentPadDp, scale)
	newCols, newRows := gridSize(image.Pt(fw-2*padPx, fh-tabBarH-2*padPx), s.cellW, s.cellH)

	s.paintFrame(fw, fh, tabBarH, padPx)
	s.triggerResizeIfNeeded(newCols, newRows, inLiveResize, needFinalSync)
	s.drainClipboardWrites()
	s.uploadAndPresent(ctx, fw, fh)
}

// syncLiveResizeFreeze implements the Windows live-resize freeze:
// repainting on every WM_SIZE tick is expensive and briefly shows a stale
// cols/rows grid (the old renderer's text-truncation-during-resize
// complaint) — instead, keep presenting the last good texture unchanged
// while the drag is in progress, and let onDraw do one full repaint once it
// ends. Returns true when it handled the frame itself, so onDraw should
// return without painting.
func (s *uiState) syncLiveResizeFreeze(ctx *gogpulib.Context, inLiveResize bool) bool {
	if runtime.GOOS != osWindows || !inLiveResize || s.tex == nil {
		return false
	}
	s.pendingTexSync = true
	bg := s.palette.Background
	ctx.Clear(float32(bg.R)/255, float32(bg.G)/255, float32(bg.B)/255, float32(bg.A)/255)
	if err := ctx.DrawTexture(s.tex, 0, 0); err != nil {
		s.app.RequestRedraw()
	}
	return true
}

// consumeLiveResizeSync tracks pendingTexSync across frames and reports
// whether this frame is the one full repaint owed now that a live resize
// just ended. Kept separate from syncLiveResizeFreeze rather than combined
// into one two-value return: "did we freeze this frame" and "do we owe a
// final sync now" are easy to mix up if merged into a single call.
func (s *uiState) consumeLiveResizeSync(inLiveResize bool) (needFinalSync bool) {
	if inLiveResize {
		s.pendingTexSync = true
		return false
	}
	if s.pendingTexSync {
		s.pendingTexSync = false
		return true
	}
	return false
}

// paintFrame (re)sizes s.frame to fw*fh RGBA8 pixels, clears it to the
// background color, then paints the tab bar and (if a session is active)
// the terminal grid and scrollbar overlay into it.
func (s *uiState) paintFrame(fw, fh, tabBarH, padPx int) {
	if needed := fw * fh * 4; needed > cap(s.frame) {
		s.frame = make([]byte, needed, needed+needed/4)
	} else {
		s.frame = s.frame[:needed]
	}
	s.frameW, s.frameH = fw, fh
	bg := toRGBA(s.palette.Background)
	fillRect(s.frame, fw, 0, 0, fw, fh, bg)

	tabs := s.mgr.Tabs()
	drag := chrome.DragVisual{
		Active:   s.tabDrag.dragging,
		From:     s.tabDrag.from,
		Over:     s.tabDrag.over,
		DX:       s.tabDrag.dx,
		ScrollX:  s.tabScrollX,
		TabID:    s.tabDrag.tabID,
		HoverIdx: s.hoverTabIdx,
	}
	s.tabBar.Layout(s.frame, fw, fh, tabBarH, s.palette, tabs, s.mgr.ActiveID(), pluginStatusText(s.pluginHost), drag, s.hoverPlus)

	if active := s.mgr.Active(); active != nil {
		s.painter.Paint(s.frame, fw, fh, padPx, tabBarH+padPx, active.Term, active.ScrollOffset(), gridSelection(active), gridPlacements(active), s.blinkOn.Load())
		s.paintScrollBarOverlay(fw, fh, tabBarH+padPx, active, time.Now().Before(s.scrollBarUntil))
	}
}

// triggerResizeIfNeeded tells s.resizeDebouncer about a new grid size,
// collapsing onDraw's two resize-trigger call sites (a size change mid-
// frame, and the live-resize-just-ended final sync) into one place.
func (s *uiState) triggerResizeIfNeeded(newCols, newRows int, inLiveResize, needFinalSync bool) {
	if newCols != s.cols || newRows != s.rows {
		s.cols, s.rows = newCols, newRows
		if !inLiveResize || needFinalSync {
			s.resizeDebouncer.Trigger(s.cols, s.rows)
		}
	}
	if needFinalSync {
		s.resizeDebouncer.Trigger(s.cols, s.rows)
	}
}

// drainClipboardWrites flushes any OSC 52 clipboard writes queued by a
// session's PTY read goroutine — checked for every tab, not just the
// active one.
func (s *uiState) drainClipboardWrites() {
	for _, tb := range s.mgr.Tabs() {
		if data, ok := tb.Session.TakeClipboardWrite(); ok {
			_ = clipboardWrite(s.app, string(data))
		}
	}
}

// uploadAndPresent uploads s.frame as the window's GPU texture — recreating
// it if the framebuffer size changed since the last frame — and presents
// it.
func (s *uiState) uploadAndPresent(ctx *gogpulib.Context, fw, fh int) {
	if s.tex != nil && (s.tex.Width() != fw || s.tex.Height() != fh) {
		s.tex.Destroy()
		s.tex = nil
	}
	if s.tex == nil {
		tex, err := ctx.Renderer().NewTextureFromRGBA(fw, fh, s.frame)
		if err != nil {
			s.app.RequestRedraw()
			return
		}
		s.tex = tex
	} else if err := s.tex.UpdateData(s.frame); err != nil {
		s.app.RequestRedraw()
		return
	}

	bg := toRGBA(s.palette.Background)
	ctx.Clear(float32(bg.R)/255, float32(bg.G)/255, float32(bg.B)/255, 1)
	if err := ctx.PresentTexture(s.tex); err != nil {
		s.app.RequestRedraw()
	}
}

const chromeContentPadDp = 8

// ensureFonts (re)loads the grid + tab-bar font faces when the scale
// factor or configured font size changes, mirroring the old gio code's
// once-per-DPI cell measurement, generalized to cover both fonts.
func (s *uiState) ensureFonts(scale float64) {
	size := s.cfg.Font.Size
	if size <= 0 {
		size = 13
	}
	if s.painter.Face != nil && s.fontSizeCurrent == float64(size) && s.scale == scale {
		return
	}
	b := loadFontBundle(s.cfg.Font.Family, float64(size), scale)
	s.painter.Face = b.face
	s.painter.CellWidth = b.cellW
	s.painter.CellHeight = b.cellH
	s.painter.Ascent = b.ascent
	s.cellW, s.cellH, s.asc = b.cellW, b.cellH, b.ascent

	tb := loadFontBundle(s.cfg.Font.Family, 12, scale)
	s.tabBar.Face = tb.face
	s.tabBar.Ascent = tb.ascent
	s.tabBar.Scale = scale

	s.fontSizeCurrent = float64(size)
	s.scale = scale
}

// paintScrollBarOverlay draws a translucent scrollbar track spanning the
// grid's full height plus a thumb sized proportionally to the visible
// fraction of scrollback (screen rows / (history + screen rows)), clamped
// to a minimum height so it stays grabbable even with very long history.
// Thumb position interpolates from the bottom (hist==scrollOffset, i.e. at
// the live screen) to the top (scrolled all the way back). Draws nothing
// when show is false (see scrollBarUntil's fade-out timer) or there's no
// history to scroll through.
func (s *uiState) paintScrollBarOverlay(fw, fh, originY int, active *session.Session, show bool) {
	if !show {
		return
	}
	active.Term.RLock()
	hist := len(active.Term.History())
	sz := active.Term.Size()
	active.Term.RUnlock()
	if hist <= 0 {
		return
	}
	trackW := 7
	margin := 2
	x0, y0 := fw-trackW-margin, originY+margin
	x1, y1 := fw-margin, fh-margin
	if x1-x0 < 2 || y1-y0 < 8 {
		return
	}
	fillRoundRect(s.frame, fw, x0, y0, x1, y1, (x1-x0)/2, color32(0xff, 0xff, 0xff, 0x28))

	total := hist + sz.R
	if total <= 0 {
		return
	}
	scrollOffset := active.ScrollOffset()
	thumbH := (y1 - y0) * sz.R / total
	if thumbH < 24 {
		thumbH = 24
	}
	if thumbH > y1-y0 {
		thumbH = y1 - y0
	}
	travel := (y1 - y0) - thumbH
	thumbY := y1 - thumbH
	if hist > 0 {
		thumbY = y0 + travel*(hist-scrollOffset)/hist
	}
	fillRoundRect(s.frame, fw, x0, thumbY, x1, thumbY+thumbH, (x1-x0)/2, color32(0xff, 0xff, 0xff, 0x70))
}

// gridSelection converts sess's current selection (if any) into the
// Selection shape Painter.Paint expects.
func gridSelection(sess *session.Session) Selection {
	start, end, ok := sess.Selection()
	if !ok {
		return Selection{}
	}
	sel := Selection{Active: true}
	sel.Start.Col, sel.Start.Row = start.Col, start.Row
	sel.End.Col, sel.End.Row = end.Col, end.Row
	return sel
}

// gridPlacements converts sess's currently decoded Kitty-graphics images
// into the Placement shape Painter.Paint expects.
func gridPlacements(sess *session.Session) []Placement {
	placements := sess.Placements()
	if len(placements) == 0 {
		return nil
	}
	out := make([]Placement, 0, len(placements))
	for _, p := range placements {
		if p.Image == nil {
			continue
		}
		out = append(out, Placement{
			Seq:     p.Seq,
			Image:   toRGBAImage(p.Image),
			AbsLine: p.AbsLine,
			Col:     p.Col,
			Cols:    p.Cols,
			Rows:    p.Rows,
		})
	}
	return out
}

func toRGBAImage(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	b := img.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, img, b.Min, draw.Src)
	return rgba
}

// gridSize converts a pixel size into a terminal column/row count, given
// the measured monospace cell dimensions. Always returns at least 1x1.
func gridSize(size image.Point, cellWidth, cellHeight int) (cols, rows int) {
	if cellWidth <= 0 || cellHeight <= 0 {
		return 1, 1
	}
	cols = size.X / cellWidth
	rows = size.Y / cellHeight
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return cols, rows
}

func color32(r, g, b, a uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: a}
}

// loadPlugins loads cfg.Plugins (opt-in) and ticks status text at
// pluginStatusInterval, invalidating only when the text changes. Failures
// are logged and skipped. Returns a nil host when Plugins is empty.
func loadPlugins(cfg *config.Config, app gpuApp) (*plugin.Host, func()) {
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
					app.RequestRedraw()
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

// tabDragThresholdDp is how far the pointer must move from the press point
// before a tab press becomes a drag-reorder.
const tabDragThresholdDp = 6

// tabDragState tracks an in-progress tab press / drag-reorder across
// frames — see handleTabBarPointer. Zero value means "no gesture".
type tabDragState struct {
	pressed  bool
	dragging bool
	tabID    int
	from     int
	over     int
	dx       int
	startX   float64
	startY   float64
	pointer  int
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// accumulateScrollLines converts a scroll delta in pixels into a whole
// line count, carrying the fractional remainder in accumPx across calls so
// trackpad deltas (many small events per line) aren't rounded away one
// event at a time.
func accumulateScrollLines(deltaPx float64, lineHeight int, accumPx *float64) int {
	if lineHeight <= 0 {
		return 0
	}
	*accumPx += deltaPx
	lines := int(*accumPx) / lineHeight
	*accumPx -= float64(lines * lineHeight)
	return lines
}
