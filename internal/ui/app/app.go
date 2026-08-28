// Package app hosts geckty's window and event loop on top of
// github.com/gogpu/gogpu + github.com/gogpu/gpucontext. The terminal grid
// and tab bar are CPU-rasterized into a persistent RGBA buffer and uploaded
// as one GPU texture per dirty frame. Widget types for a future gogpu/ui
// present path live in termview/chrome; the live present path is PresentTexture.
package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	gogpulib "github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/assets"
	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/plugin"
	"github.com/geckty/geckty/internal/protocol/focus"
	"github.com/geckty/geckty/internal/rc"
	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
	"github.com/geckty/geckty/internal/ui/raster"
	"github.com/geckty/geckty/internal/ui/termview"
	"github.com/geckty/geckty/internal/ui/theme"
	"github.com/geckty/geckty/internal/vt"
	"github.com/geckty/geckty/internal/vt/emu"
)

// pluginStatusInterval is how often loaded plugins' draw_statusbar hook is
// called — its own independent cadence, not tied to frame painting or
// terminal dirty events (see loadPlugins' doc comment).
const pluginStatusInterval = time.Second

// uiState holds everything the draw/input callbacks need, promoted to a
// struct since gogpu's callback-registration API needs each handler as its
// own function value rather than one big event loop.
type uiState struct {
	cfg     *config.Config
	app     gpuApp
	win     gpuWindow
	keymap  *Keymap
	thm     theme.Theme
	painter *Painter
	tabBar  *TabBar
	mgr     *session.Manager
	newTab  func() error

	resizeDebouncer *resizeDebouncer
	pluginHost      *plugin.Host
	stopPlugins     func()

	// pendingCfg is the hand-off slot a background config.Config.Watch
	// goroutine stores a freshly reloaded Config into (see wireConfigReload)
	// — never read or mutated off the draw goroutine. applyPendingConfig,
	// called at the top of onDraw, is the only place that drains it, so
	// every actual field it touches on uiState (cfg, palette, keymap, the
	// font cache) is only ever written from that one goroutine.
	pendingCfg atomic.Pointer[config.Config]

	cols, rows int

	blinkOn atomic.Bool

	// Per-window render state.
	frame          []byte
	frameRGBA      *image.RGBA // compositor path: view of frame for canvas.DrawImage
	frameW, frameH int
	tex            *gogpulib.Texture

	compositor *uiCompositorBridge // set when GECKTY_UI_COMPOSITOR=1

	// Font/cell metrics, refreshed in onDraw when scale or configured size
	// changes (also covers the tab bar's own smaller font).
	scale               float64
	fontSizeCurrent     float64
	fontZoomDelta       float64 // runtime pt offset from cfg.Font.Size (Cmd/Ctrl+/-)
	fontZoomResizeCols  int     // >0: resize window on next frame to keep this grid
	fontZoomResizeRows  int
	fontZoomPendingDIPW int // staged logical size; applied from OnUpdate (main thread)
	fontZoomPendingDIPH int
	cellW, cellH, asc   int

	// lastPaintFH/Rows suppress partial dirty repaint while the window or
	// grid is changing (resize drag — especially growing downward).
	lastPaintFH   int
	lastPaintRows int

	// Tab-bar interaction state.
	tabDrag         tabDragState
	tabScrollX      int
	hoverTabIdx     int
	hoverPlus       bool
	scrollBarUntil  time.Time
	selEdgeLast     time.Time // last edge auto-scroll tick while dragging a selection
	search          searchState
	hintsActive     bool
	hints           []session.URLHit
	hintsLabels     []string
	confirmClose    bool // pending multi-tab quit confirmation overlay
	visualBellUntil time.Time
	lastMods        gpucontext.Modifiers // latest known mods for Shift+wheel override

	scrollAccumPx float64

	// keyEcho holds text that a following SetOnTextInput should discard as
	// an OS echo of the key we already handled. Only set for short-lived
	// printable echoes (shortcuts / single-byte writes), with a TTL so a
	// stale value after layout switch / IME never swallows real input.
	keyEcho   string
	keyEchoAt time.Time

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
	perWindowKeyReleased bool
	perWindowTextInputed bool

	// pendingTexSync / inLiveResize implement the Windows live-resize
	// freeze: repainting a full frame on every WM_SIZE tick during a drag
	// is expensive and was the source of a text-truncation-looking bug in
	// the old renderer (stale cols/rows briefly visible mid-drag). While
	// InSizeMove() is true, keep presenting the last good texture
	// unchanged; once it ends, do one full repaint at the final size and
	// only then tell the PTY about the new size.
	pendingTexSync bool

	// Content area for the active tab's pane layout (device pixels).
	contentOX, contentOY, contentW, contentH int
	activePaneRects                          []session.PaneRect
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
// running the gogpu event loop until the window closes. gogpu.App.Run is
// both "start the toolkit's main loop" and "run this session's frame/
// input loop" in one call — callers should invoke it directly on the
// main goroutine.
func Run(cfg *config.Config) error {
	if useUICompositor() {
		return runWithUICompositor(cfg)
	}
	thm, err := buildTheme(cfg)
	if err != nil {
		return err
	}
	keymap, err := NewKeymap(cfg.Keybindings)
	if err != nil {
		return err
	}

	s, gapp := buildUIState(cfg, thm, keymap)

	s.wireSessionManager(gapp)
	defer s.resizeDebouncer.Stop()

	if err := s.wireFirstTab(gapp); err != nil {
		return err
	}

	s.pluginHost, s.stopPlugins = loadPlugins(cfg, gapp)
	defer s.stopPlugins()

	stopBlink := s.startBlinkLoop(gapp)
	defer stopBlink()

	stopConfigReload := s.wireConfigReload(gapp)
	defer stopConfigReload()

	s.wireLifecycleCallbacks(gapp)
	s.wireFontZoomResize(gapp)
	s.wireEventSource(gapp)

	stopRC := s.wireRemoteControl()
	defer stopRC()

	return gapp.Run()
}

// wireRemoteControl starts the optional GECKTY_SOCKET / GECKTY_LISTEN
// listener. Returns a no-op stop when unset.
func (s *uiState) wireRemoteControl() (stop func()) {
	path := rc.SocketPath()
	if path == "" {
		return func() {}
	}
	stop, err := rc.ListenAndServe(path, rcHost{s: s})
	if err != nil {
		slog.Warn("remote control listen", slog.String("path", path), slog.Any("error", err))
		return func() {}
	}
	return stop
}

// buildUIState constructs the gogpu App and its uiState — the fields that
// don't depend on session/plugin/blink wiring, which are set up separately
// by wireSessionManager/wireFirstTab/startBlinkLoop/wireLifecycleCallbacks
// so Run itself reads as a short sequence of named setup steps rather than
// one long function.
func buildUIState(cfg *config.Config, thm theme.Theme, keymap *Keymap) (*uiState, gpuApp) {
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
	if icon := loadAppIcon(); icon != nil {
		appCfg = appCfg.WithIcon(icon)
	}

	gapp := gogpulib.NewApp(appCfg)
	s := &uiState{
		cfg:         cfg,
		app:         gapp,
		keymap:      keymap,
		thm:         thm,
		painter:     &Painter{Palette: thm.Palette, URLUnderline: thm.UI.URLUnderline},
		tabBar:      NewTabBar(),
		cols:        80,
		rows:        24,
		hoverTabIdx: -1,
	}
	s.blinkOn.Store(true)
	return s, gapp
}

// loadAppIcon decodes the embedded app icon (assets.Icon — see that
// package's doc comment) for use as the window's taskbar/decoration icon.
// Returns nil (leaving the window iconless rather than failing startup)
// if the embedded bytes ever fail to decode — a "can't happen" case since
// they're compiled in, not read from a runtime path, but not worth a
// fatal error over a cosmetic feature if it ever did.
func loadAppIcon() image.Image {
	icon, err := png.Decode(bytes.NewReader(assets.Icon))
	if err != nil {
		slog.Warn("decode embedded app icon", slog.Any("error", err))
		return nil
	}
	return icon
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
	s.resizeDebouncer = newResizeDebouncer(resizeDebounceDelay, func(_, _ int) {
		s.resizeAllPanes()
	})
}

// gridCellMetrics returns the cell pixel size used for layout and paint.
// Painter metrics win when set so row/col counts match paintRow placement.
func (s *uiState) gridCellMetrics() (cellW, cellH int) {
	cellW, cellH = s.cellW, s.cellH
	if s.painter != nil {
		if s.painter.CellWidth > 0 {
			cellW = s.painter.CellWidth
		}
		if s.painter.CellHeight > 0 {
			cellH = s.painter.CellHeight
		}
	}
	return cellW, cellH
}

func (s *uiState) paneGridSize(leaf session.PaneRect) (cols, rows int) {
	cw, ch := s.gridCellMetrics()
	return gridSize(image.Pt(leaf.W, leaf.H), cw, ch)
}

// ensurePaneGrid resizes session to the leaf's pixel rect before paint.
func (s *uiState) ensurePaneGrid(sess *session.Session, leaf session.PaneRect) (cols, rows int) {
	if sess == nil {
		return 0, 0
	}
	cols, rows = s.paneGridSize(leaf)
	sz := sess.Term.Size()
	if cols == sz.C && rows == sz.R {
		return cols, rows
	}
	if err := sess.Resize(cols, rows); err != nil {
		slog.Warn("session resize failed", slog.Any("error", err))
	}
	return cols, rows
}

// resizeAllPanes applies the current content rectangle to every tab's
// pane tree, resizing each leaf session to its cell grid.
func (s *uiState) resizeAllPanes() {
	if s.mgr == nil || s.contentW <= 0 || s.contentH <= 0 {
		return
	}
	cw, ch := s.gridCellMetrics()
	if cw <= 0 || ch <= 0 {
		return
	}
	s.mgr.EachTabLayout(s.contentOX, s.contentOY, s.contentW, s.contentH, func(leaves []session.PaneRect) {
		for _, leaf := range leaves {
			if leaf.Session == nil {
				continue
			}
			cols, rows := gridSize(image.Pt(leaf.W, leaf.H), cw, ch)
			sz := leaf.Session.Term.Size()
			if cols == sz.C && rows == sz.R {
				continue
			}
			if err := leaf.Session.Resize(cols, rows); err != nil {
				slog.Warn("session resize failed", slog.Any("error", err))
			}
		}
	})
}

// syncResizeBeforePaint resizes PTY/emu to match the window before painting
// so the grid and shell stay aligned (avoids an empty band below the grid
// while the debouncer catches up). Only skipped during Windows live-resize
// drag — macOS must sync every frame or the grid outruns the terminal.
func (s *uiState) syncResizeBeforePaint(fw, fh, tabBarH, padPx, newCols, newRows int, inLiveResize, needFinalSync bool) {
	if newCols != s.cols || newRows != s.rows {
		s.cols, s.rows = newCols, newRows
	}
	if runtime.GOOS == osWindows && inLiveResize && !needFinalSync {
		return
	}
	ox, oy := padPx, tabBarH+padPx
	cw, ch := fw-2*padPx, fh-tabBarH-2*padPx
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	s.contentOX, s.contentOY, s.contentW, s.contentH = ox, oy, cw, ch
	s.resizeAllPanes()
}

// wireFirstTab sets s.newTab (spawning s.cfg.ShellCommand() — read fresh
// each call, not the cfg parameter, so a hot-reloaded shell.command takes
// effect for tabs opened after the reload; already-running tabs keep
// whatever shell they were launched with) in the user's home directory at
// the current grid size, and opens the first tab, returning any error from
// that initial spawn.
func (s *uiState) wireFirstTab(gapp gpuApp) error {
	const op = "ui.wireFirstTab"
	homeDir, _ := os.UserHomeDir()
	s.newTab = func() error {
		dir := strings.TrimSpace(s.cfg.Shell.WorkingDir)
		if dir == "" {
			dir = homeDir
		}
		sess, err := s.mgr.New(session.Config{
			Command:          s.cfg.ShellCommand(),
			Env:              append([]string(nil), s.cfg.Shell.Env...),
			Dir:              dir,
			Cols:             s.cols,
			Rows:             s.rows,
			HistoryLimit:     s.cfg.Scrollback.Lines,
			Clipboard:        session.ParseClipboardPolicy(s.cfg.Clipboard.OSC52Write, s.cfg.Clipboard.OSC52Read, s.cfg.Clipboard.MaxSize),
			OnDirty:          gapp.RequestRedraw,
			ShellIntegration: s.cfg.Shell.Integration,
			Log:              slog.Default().With(slog.String("op", op)),
		})
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
		s.attachOSC52HostRead(sess)
		return nil
	}
	s.mgr.SetSpawn(func(cols, rows int) (*session.Session, error) {
		dir := strings.TrimSpace(s.cfg.Shell.WorkingDir)
		if dir == "" {
			dir = homeDir
		}
		if cols < 1 {
			cols = 1
		}
		if rows < 1 {
			rows = 1
		}
		sess, err := session.New(session.Config{
			Command:          s.cfg.ShellCommand(),
			Env:              append([]string(nil), s.cfg.Shell.Env...),
			Dir:              dir,
			Cols:             cols,
			Rows:             rows,
			HistoryLimit:     s.cfg.Scrollback.Lines,
			Clipboard:        session.ParseClipboardPolicy(s.cfg.Clipboard.OSC52Write, s.cfg.Clipboard.OSC52Read, s.cfg.Clipboard.MaxSize),
			OnDirty:          gapp.RequestRedraw,
			ShellIntegration: s.cfg.Shell.Integration,
			Log:              slog.Default().With(slog.String("op", op+".split")),
		})
		if err != nil {
			return nil, err
		}
		s.attachOSC52HostRead(sess)
		return sess, nil
	})
	return s.newTab()
}

// attachOSC52HostRead wires OSC 52 clipboard queries to the host pasteboard
// when clipboard.osc52_read = "allow".
func (s *uiState) attachOSC52HostRead(sess *session.Session) {
	if sess == nil {
		return
	}
	sess.SetHostClipboardRead(func() ([]byte, bool) {
		text, err := clipboardReadNative()
		if err != nil || text == "" {
			return nil, false
		}
		return []byte(text), true
	})
}

// wireConfigReload starts watching s.cfg's source file (see
// config.Config.Watch) when s.cfg.HotReload is set, handing each
// successfully reparsed Config to applyPendingConfig via the pendingCfg
// slot and requesting a redraw so onDraw picks it up promptly rather than
// waiting for the next unrelated repaint. A no-op (returning a no-op stop
// func) when HotReload is off or s.cfg has no source path (e.g. tests
// building a Config directly rather than through config.Load).
func (s *uiState) wireConfigReload(gapp gpuApp) (stop func()) {
	if !s.cfg.HotReload {
		return func() {}
	}
	return s.cfg.Watch(func(reloaded *config.Config, err error) {
		if err != nil {
			slog.Warn("reload config", slog.Any("error", err))
			return
		}
		s.pendingCfg.Store(reloaded)
		gapp.RequestRedraw()
	})
}

// applyPendingConfig drains pendingCfg (see wireConfigReload) and, if a
// reloaded Config is waiting, applies it. Called at the top of onDraw so
// every actual mutation below happens on the draw goroutine, not the
// background watch goroutine that detected the change.
//
// Deliberately not reloaded here: Window (resizing a live window on config
// change would be a surprising side effect of editing a text file, not a
// requested one), Plugins (unloading/reloading a running WASM host safely
// is a separate, larger feature), and already-running tabs' Shell.Command
// (wireFirstTab reads s.cfg fresh, so only tabs opened after this point see
// a changed shell).
func (s *uiState) applyPendingConfig() {
	reloaded := s.pendingCfg.Swap(nil)
	if reloaded == nil {
		return
	}
	s.applyConfig(reloaded)
}

// buildTheme parses [colors]/[ui] and applies [cursor].color over colors.cursor
// when set (Kitty-style: dedicated cursor key wins).
func buildTheme(cfg *config.Config) (theme.Theme, error) {
	return theme.New(cfg)
}

func (s *uiState) applyConfig(cfg *config.Config) {
	if thm, err := buildTheme(cfg); err != nil {
		slog.Warn("reload config: invalid colors, keeping previous", slog.Any("error", err))
	} else {
		s.thm = thm
		s.painter.Palette = thm.Palette
		s.painter.URLUnderline = thm.UI.URLUnderline
	}
	if keymap, err := NewKeymap(cfg.Keybindings); err != nil {
		slog.Warn("reload config: invalid keybindings, keeping previous", slog.Any("error", err))
	} else {
		s.keymap = keymap
	}

	s.cfg = cfg
	// Force ensureFonts to rebuild every face on the next call (this same
	// frame, right after applyPendingConfig returns) even if size/scale
	// happen to match its cache — Family/Bold/Italic may have changed, and
	// ensureFonts' cache check has no way to notice that on its own.
	s.painter.Fonts = fontBundle{}
}

// startBlinkLoop starts the cursor-blink ticker goroutine and returns a
// stop func that terminates it — mirrors loadPlugins' stopPlugins pattern
// so Run's shutdown sequence reads the same way for both.
func (s *uiState) startBlinkLoop(gapp gpuApp) (stop func()) {
	if !s.cfg.Cursor.Blink {
		return func() {}
	}
	interval := time.Duration(s.cfg.Cursor.IntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = 530 * time.Millisecond
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
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
	win.SetOnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		s.perWindowKeyReleased = true
		s.handleKeyRelease(key, mods)
	})
	win.SetOnTextInput(func(text string) {
		s.perWindowTextInputed = true
		s.handleTextInput(text)
	})
	win.SetOnPointer(s.handlePointerEvent)
	win.SetOnScroll(s.handleScrollEvent)
	win.SetOnClose(func() bool {
		if s.cfg != nil && s.cfg.Window.ConfirmClose && len(s.mgr.Tabs()) > 1 {
			s.confirmClose = true
			s.app.RequestRedraw()
			return false // cancel OS close; overlay asks first
		}
		return true
	})
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
	src.OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		if s.perWindowKeyReleased {
			s.perWindowKeyReleased = false
			return
		}
		s.handleKeyRelease(key, mods)
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

const keyEchoTTL = 80 * time.Millisecond

func (s *uiState) setKeyEcho(text string) {
	if text == "" {
		s.keyEcho = ""
		return
	}
	// Single printable ASCII used as OS echo of a handled shortcut (letters,
	// digits, and font-zoom punctuation). Escape sequences and multi-byte
	// input must not stick — they never arrive as matching TextInput.
	if len(text) == 1 {
		c := text[0]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '=' || c == '-' || c == '+' {
			s.keyEcho = text
			s.keyEchoAt = time.Now()
			return
		}
	}
	s.keyEcho = ""
}

// handleKeyPress matches a shortcut first, then falls through to Kitty/
// legacy encoding for the active session — the same precedence as the old
// key-press path, adapted to gpucontext's split key/text-input
// callbacks (see keyEcho's doc comment).
func (s *uiState) handleKeyPress(key gpucontext.Key, mods gpucontext.Modifiers) {
	s.keyEcho = ""
	s.lastMods = mods
	if s.handleConfirmCloseKey(key) {
		return
	}
	if s.handleHintsKey(key, mods) {
		return
	}
	if s.handleSearchKey(key, mods) {
		return
	}
	if action, ok := s.keymap.Match(key, mods); ok {
		s.dispatchAction(action)
		s.setKeyEcho(keyToChar[key])
		return
	}
	if s.tryScrollbackKey(key, mods) {
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
		s.setKeyEcho(string(b))
		return
	}
	if b, ok := EncodeKey(key, mods); ok {
		_, _ = active.Write(b)
		s.setKeyEcho(string(b))
	}
}

// tryScrollbackKey handles Shift+PageUp/PageDown (and plain PageUp/Down
// once already scrolled into history) as local scrollback navigation.
// Full-screen alt-buffer apps keep receiving PageUp/Down as CSI.
func (s *uiState) tryScrollbackKey(key gpucontext.Key, mods gpucontext.Modifiers) bool {
	if key != gpucontext.KeyPageUp && key != gpucontext.KeyPageDown {
		return false
	}
	active := s.mgr.Active()
	if active == nil {
		return false
	}
	active.Term.RLock()
	mode := active.Term.Mode()
	rows := active.Term.Size().R
	active.Term.RUnlock()
	if mode&emu.ModeAltScreen != 0 {
		return false
	}
	shift := mods.HasShift()
	scrolled := active.ScrollOffset() > 0
	if !shift && !scrolled {
		return false
	}
	page := rows - 1
	if page < 1 {
		page = 1
	}
	delta := page
	if key == gpucontext.KeyPageDown {
		delta = -page
	}
	active.ScrollBy(delta)
	s.scrollBarUntil = time.Now().Add(1200 * time.Millisecond)
	s.app.RequestRedraw()
	return true
}

func (s *uiState) handleKeyRelease(key gpucontext.Key, mods gpucontext.Modifiers) {
	s.lastMods = mods
	if s.searchActive() || s.hintsOverlayActive() {
		return
	}
	active := s.mgr.Active()
	if active == nil {
		return
	}
	active.Term.RLock()
	keyState := active.Term.KeyState()
	active.Term.RUnlock()
	// EncodeKitty returns ok=false unless Report event types is set, so
	// key-up never duplicates a press-shaped CSI-u under Disambiguate-only.
	if b, ok := EncodeKitty(keyState, key, mods, false); ok {
		_, _ = active.Write(b)
	}
}

func (s *uiState) handleTextInput(text string) {
	if echo := s.keyEcho; echo != "" {
		s.keyEcho = ""
		fresh := time.Since(s.keyEchoAt) <= keyEchoTTL
		if fresh && strings.EqualFold(echo, text) {
			return
		}
	}
	if text == "" {
		return
	}
	if s.handleSearchText(text) {
		return
	}
	if s.hintsOverlayActive() {
		return // hint labels are key-press only; don't duplicate via IME/text input
	}
	active := s.mgr.Active()
	if active == nil {
		return
	}
	_, _ = active.Write(EncodeText(text))
	active.ClearSelection()
}

// splitActivePane splits the focused pane, then reflows every leaf's PTY size.
func (s *uiState) splitActivePane(dir session.SplitDir) {
	active := s.mgr.Active()
	if active == nil {
		return
	}
	active.Term.RLock()
	sz := active.Term.Size()
	active.Term.RUnlock()
	cols, rows := sz.C, sz.R
	switch dir {
	case session.SplitVertical:
		cols /= 2
		if cols < 1 {
			cols = 1
		}
	default:
		rows /= 2
		if rows < 1 {
			rows = 1
		}
	}
	if !s.mgr.Split(dir, cols, rows) {
		return
	}
	s.resizeAllPanes()
	s.app.RequestRedraw()
}

const (
	fontZoomMinPt = 6.0
	fontZoomMaxPt = 72.0
)

// adjustFontZoom changes the live font size by deltaPt (0 resets to the
// configured size). Forces ensureFonts to rebuild on the next frame,
// resizes the window to keep the current cols×rows grid (Kitty-style), and
// re-clamps scrollback offsets so the viewport cannot drift into empty rows.
func (s *uiState) adjustFontZoom(deltaPt float64) {
	base := float64(s.cfg.Font.Size)
	if base <= 0 {
		base = 13
	}
	if deltaPt == 0 {
		s.fontZoomDelta = 0
	} else {
		next := base + s.fontZoomDelta + deltaPt
		if next < fontZoomMinPt {
			next = fontZoomMinPt
		}
		if next > fontZoomMaxPt {
			next = fontZoomMaxPt
		}
		s.fontZoomDelta = next - base
	}
	if s.cols > 0 && s.rows > 0 {
		s.fontZoomResizeCols = s.cols
		s.fontZoomResizeRows = s.rows
	}
	s.fontSizeCurrent = 0 // bust ensureFonts cache
	s.clampAllSessionScroll()
	s.app.RequestRedraw()
}

// windowSizeForGrid returns the logical (DIP) window size that fits cols×rows
// terminal cells with the current font metrics and chrome padding.
func windowSizeForGrid(cols, rows int, cellW, cellH int, scale float64, padDp int, tabBarVisible bool) (dipW, dipH int) {
	if scale <= 0 {
		scale = 1
	}
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if cellW < 1 {
		cellW = 1
	}
	if cellH < 1 {
		cellH = 1
	}
	padPx := dpToPx(padDp, scale)
	tabBarPx := 0
	if tabBarVisible {
		tabBarPx = dpToPx(TabBarHeightDp, scale)
	}
	fw := cols*cellW + 2*padPx
	fh := rows*cellH + tabBarPx + 2*padPx
	if fw < 1 {
		fw = 1
	}
	if fh < 1 {
		fh = 1
	}
	return int(float64(fw)/scale + 0.5), int(float64(fh)/scale + 0.5)
}

// syncFontZoomWindowSize stages a Kitty-style window resize for the next
// main-thread OnUpdate tick. SetSize must not run from onDraw — gogpu draws
// on the render thread and AppKit crashes if the window is resized there.
func (s *uiState) stageFontZoomWindowResize() {
	cols, rows := s.fontZoomResizeCols, s.fontZoomResizeRows
	if cols <= 0 || rows <= 0 || s.cellW <= 0 || s.cellH <= 0 {
		return
	}
	s.fontZoomResizeCols, s.fontZoomResizeRows = 0, 0
	tabBarVisible := s.tabBarShowTabs() || s.tabBarShowPlus()
	dipW, dipH := windowSizeForGrid(cols, rows, s.cellW, s.cellH, s.scale, s.contentPadDp(), tabBarVisible)
	s.fontZoomPendingDIPW, s.fontZoomPendingDIPH = dipW, dipH
}

// applyPendingFontZoomWindowResize runs on the main thread (gogpu OnUpdate)
// and applies any DIP size staged by the last onDraw after a font zoom.
func (s *uiState) applyPendingFontZoomWindowResize() {
	dipW, dipH := s.fontZoomPendingDIPW, s.fontZoomPendingDIPH
	if dipW <= 0 || dipH <= 0 {
		return
	}
	s.fontZoomPendingDIPW, s.fontZoomPendingDIPH = 0, 0
	win := s.app.PrimaryWindow()
	if win == nil {
		return
	}
	win.SetSize(dipW, dipH)
}

func (s *uiState) wireFontZoomResize(gapp gpuApp) {
	gapp.OnUpdate(func(_ float64) {
		s.applyPendingFontZoomWindowResize()
	})
}

func (s *uiState) clampAllSessionScroll() {
	if s.mgr == nil {
		return
	}
	for _, sess := range s.mgr.AllSessions() {
		if sess != nil {
			sess.ScrollBy(0)
		}
	}
}

// onDraw is the gogpu FrameEvent equivalent: measure fonts/cell metrics,
// lay out chrome + grid, paint into the persistent frame buffer, and
// present it as a texture.
func (s *uiState) onDraw(ctx *gogpulib.Context) {
	s.applyPendingConfig()

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

	tabBarH := s.tabBarHeightPx()
	padPx := dpToPx(s.contentPadDp(), scale)
	cellW, cellH := s.gridCellMetrics()
	newCols, newRows := gridSize(image.Pt(fw-2*padPx, fh-tabBarH-2*padPx), cellW, cellH)

	s.syncResizeBeforePaint(fw, fh, tabBarH, padPx, newCols, newRows, inLiveResize, needFinalSync)
	s.paintFrame(fw, fh, tabBarH, padPx)
	s.drainBells()
	s.triggerResizeIfNeeded(newCols, newRows, inLiveResize, needFinalSync)
	s.drainClipboardWrites()
	s.uploadAndPresent(ctx, fw, fh)
	s.stageFontZoomWindowResize()
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
	bg := s.thm.Palette.Background
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
// background color (or, for a single-pane dirty redraw, only the chrome
// strip), then paints the tab bar and terminal grid into it.
func (s *uiState) paintFrame(fw, fh, tabBarH, padPx int) {
	prevSame := len(s.frame) >= fw*fh*4 && s.frameW == fw && s.frameH == fh
	if needed := fw * fh * 4; needed > cap(s.frame) {
		s.frame = make([]byte, needed, needed+needed/4)
		prevSame = false
	} else {
		s.frame = s.frame[:needed]
	}
	s.frameW, s.frameH = fw, fh
	bg := toRGBA(s.thm.Palette.Background)

	tabs := s.mgr.Tabs()
	if !s.tabBarShowTabs() {
		tabs = nil
	}
	drag := chrome.DragVisual{
		Active:   s.tabDrag.dragging,
		From:     s.tabDrag.from,
		Over:     s.tabDrag.over,
		DX:       s.tabDrag.dx,
		ScrollX:  s.tabScrollX,
		TabID:    s.tabDrag.tabID,
		HoverIdx: s.hoverTabIdx,
	}

	ox, oy := padPx, tabBarH+padPx
	cw, ch := fw-2*padPx, fh-tabBarH-2*padPx
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	s.contentOX, s.contentOY, s.contentW, s.contentH = ox, oy, cw, ch
	leaves, focus, ok := s.mgr.ActiveLayout(ox, oy, cw, ch)
	s.activePaneRects = leaves

	var dirtyRows vt.DirtyRows
	useDirty := false
	gridMismatch := false
	gridGrowing := false
	if ok && len(leaves) == 1 && leaves[0].Session != nil && leaves[0].Session.ScrollOffset() == 0 {
		leaf := leaves[0]
		paintCols, paintRows := s.paneGridSize(leaf)
		sz := leaf.Session.Term.Size()
		gridMismatch = paintCols != sz.C || paintRows != sz.R
		gridGrowing = paintRows > s.lastPaintRows || fh > s.lastPaintFH
		lines, screenCh := leaf.Session.Term.TakePaintDirty()
		if prevSame && !screenCh && !gridMismatch && !gridGrowing &&
			!s.app.InSizeMove() && len(lines) > 0 {
			useDirty = true
			dirtyRows = lines
		}
	} else if ok {
		for _, leaf := range leaves {
			if leaf.Session != nil {
				_, _ = leaf.Session.Term.TakePaintDirty()
			}
		}
	}

	if useDirty {
		raster.FillRect(s.frame, fw, 0, 0, fw, tabBarH+padPx, bg)
	} else {
		raster.FillRect(s.frame, fw, 0, 0, fw, fh, bg)
	}

	s.tabBar.Layout(s.frame, fw, fh, tabBarH, s.thm, tabs, s.mgr.ActiveID(), pluginStatusText(s.pluginHost), drag, s.hoverPlus, s.tabBarShowPlus())

	if ok {
		if len(leaves) > 1 {
			raster.FillRect(s.frame, fw, ox, oy, ox+cw, oy+ch, toRGBA(s.thm.Palette.HoverTabBG))
		}
		for _, leaf := range leaves {
			if leaf.Session == nil {
				continue
			}
			paintCols, paintRows := s.ensurePaneGrid(leaf.Session, leaf)
			raster.FillRect(s.frame, fw, leaf.X, leaf.Y, leaf.X+leaf.W, leaf.Y+leaf.H, bg)
			rows := dirtyRows
			if !useDirty || leaf.Session != focus {
				rows = nil
			}
			s.painter.Paint(s.frame, fw, fh, leaf.X, leaf.Y, leaf.Session.Term, leaf.Session.ScrollOffset(), gridSelection(leaf.Session), gridPlacements(leaf.Session), s.blinkOn.Load(), rows, paintCols, paintRows)
			_, cellH := s.gridCellMetrics()
			gridBottom := leaf.Y + paintRows*cellH
			if gridBottom < leaf.Y+leaf.H {
				raster.FillRect(s.frame, fw, leaf.X, gridBottom, leaf.X+leaf.W, leaf.Y+leaf.H, bg)
			}
			if paintRows > s.lastPaintRows {
				s.lastPaintRows = paintRows
			}
			s.paintContentBrackets(fw, leaf.X, leaf.Y, leaf.W, leaf.H, leaf.Session)
			if leaf.Session == focus {
				s.paintScrollBarOverlay(fw, fh, leaf.Y, leaf.Session, time.Now().Before(s.scrollBarUntil))
				if len(leaves) > 1 {
					ring := toRGBA(s.thm.UI.PaneFocusBorder)
					const t = 2
					raster.BlendRect(s.frame, fw, leaf.X, leaf.Y, leaf.X+leaf.W, leaf.Y+t, ring)
					raster.BlendRect(s.frame, fw, leaf.X, leaf.Y+leaf.H-t, leaf.X+leaf.W, leaf.Y+leaf.H, ring)
					raster.BlendRect(s.frame, fw, leaf.X, leaf.Y, leaf.X+t, leaf.Y+leaf.H, ring)
					raster.BlendRect(s.frame, fw, leaf.X+leaf.W-t, leaf.Y, leaf.X+leaf.W, leaf.Y+leaf.H, ring)
				}
			}
		}
		if focus != nil {
			s.paintCommandBorder(fw, fh, focus)
		}
	}
	s.paintSearchOverlay(fw, fh, padPx, tabBarH)
	s.paintHintsOverlay(fw, fh)
	s.paintConfirmCloseOverlay(fw, fh, padPx)
	s.paintVisualBell(fw, fh)
	s.lastPaintFH = fh
}

func (s *uiState) drainBells() {
	for _, sess := range s.mgr.AllSessions() {
		if sess != nil && sess.Term.TakeBell() {
			s.visualBellUntil = time.Now().Add(120 * time.Millisecond)
			s.app.RequestRedraw()
			time.AfterFunc(130*time.Millisecond, func() { s.app.RequestRedraw() })
		}
	}
}

func (s *uiState) paintVisualBell(fw, fh int) {
	if time.Now().After(s.visualBellUntil) {
		return
	}
	c := toRGBA(s.thm.UI.VisualBell)
	t := dpToPx(4, s.scale)
	if t < 2 {
		t = 2
	}
	raster.BlendRect(s.frame, fw, 0, 0, fw, t, c)
	raster.BlendRect(s.frame, fw, 0, fh-t, fw, fh, c)
	raster.BlendRect(s.frame, fw, 0, 0, t, fh, c)
	raster.BlendRect(s.frame, fw, fw-t, 0, fw, fh, c)
}

func (s *uiState) handleConfirmCloseKey(key gpucontext.Key) bool {
	if !s.confirmClose {
		return false
	}
	switch key {
	case gpucontext.KeyEscape:
		s.confirmClose = false
		s.app.RequestRedraw()
		return true
	case gpucontext.KeyEnter:
		s.confirmClose = false
		s.app.Quit()
		return true
	}
	return true // swallow other keys while confirming
}

func (s *uiState) paintConfirmCloseOverlay(fw, fh, padPx int) {
	if !s.confirmClose {
		return
	}
	n := len(s.mgr.Tabs())
	msg := fmt.Sprintf("Close %d tabs?  Enter = yes, Esc = cancel", n)
	barH := s.cellH + 2*padPx
	if barH < 28 {
		barH = 28
	}
	y0 := (fh - barH) / 2
	if y0 < 0 {
		y0 = 0
	}
	bg := withAlpha(toRGBA(s.thm.Palette.InactiveTabBG), 0xf0)
	raster.BlendRect(s.frame, fw, padPx, y0, fw-padPx, y0+barH, bg)
	if s.tabBar != nil {
		s.tabBar.drawText(s.frame, fw, fh, msg, padPx*2, y0, fw-4*padPx, barH, toRGBA(s.thm.Palette.Foreground))
	}
}

// paintContentBrackets draws soft [ ] marks on each OSC 133 command-input
// line (the PromptStart that was followed by CommandExecuted — i.e. after
// the user pressed Enter). Idle prompts without a submitted command stay
// unmarked. Marks scroll with the viewport like selection/URL chrome.
func (s *uiState) paintContentBrackets(fw, ox, oy, cw, ch int, sess *session.Session) {
	c := toRGBA(s.thm.UI.ContentBrackets)
	if c.A == 0 || sess == nil || s.cellH <= 0 || cw < 8 {
		return
	}
	sess.Term.RLock()
	marks := sess.Term.PromptMarks()
	viewRows := sess.Term.Size().R
	sess.Term.RUnlock()
	if viewRows < 1 || len(marks) == 0 {
		return
	}
	top := sess.ViewportTopAbsLine()
	bottom := top + viewRows

	arm := dpToPx(7, s.scale)
	if arm < 4 {
		arm = 4
	}
	t := 1
	lx := ox - 2
	if lx < 0 {
		lx = 0
	}
	rx := ox + cw + 1
	if rx >= fw {
		rx = fw - 1
	}

	for i, m := range marks {
		if m.Type != emu.CommandExecuted {
			continue
		}
		// Brackets frame the prompt/command line the user submitted, not
		// the output that follows CommandExecuted.
		abs := m.AbsLine
		for j := i - 1; j >= 0; j-- {
			if marks[j].Type == emu.PromptStart {
				abs = marks[j].AbsLine
				break
			}
		}
		if abs < top || abs >= bottom {
			continue
		}
		viewRow := abs - top
		y0 := oy + viewRow*s.cellH
		y1 := y0 + s.cellH
		if y1 > oy+ch {
			y1 = oy + ch
		}
		if y0 >= oy+ch || y1 <= oy {
			continue
		}
		// [
		raster.BlendRect(s.frame, fw, lx, y0, lx+t, y1, c)
		raster.BlendRect(s.frame, fw, lx, y0, lx+arm, y0+t, c)
		raster.BlendRect(s.frame, fw, lx, y1-t, lx+arm, y1, c)
		// ]
		raster.BlendRect(s.frame, fw, rx, y0, rx+t, y1, c)
		raster.BlendRect(s.frame, fw, rx-arm+t, y0, rx+t, y0+t, c)
		raster.BlendRect(s.frame, fw, rx-arm+t, y1-t, rx+t, y1, c)
	}
}

// commandBorderThicknessDp is the active tab's OSC 133 status border's
// width — thick enough to read clearly as "this window has a highlight"
// at a glance without eating meaningfully into the content area, which
// tabBarShowTabs()-hidden single-tab windows (the common case) have no
// other indicator for at all (the tab-pill dot in tabbar.go's paintTab
// needs a visible tab strip, which a single tab doesn't show by default).
const commandBorderThicknessDp = 3

// paintCommandBorder outlines the whole window in commandIndicatorColor's
// color for sess — cyan while a command is running, a brief green/red
// flash after it finishes — so there's a highlight visible regardless of
// tab count or tab-bar visibility, not just the active tab's own pill.
func (s *uiState) paintCommandBorder(fw, fh int, sess *session.Session) {
	if !s.thm.UI.CommandBorderEnabled {
		return
	}
	c, ok := commandIndicatorColor(sess, s.thm)
	if !ok {
		return
	}
	t := dpToPx(commandBorderThicknessDp, s.scale)
	if t < 1 {
		t = 1
	}
	if 2*t >= fw || 2*t >= fh {
		return
	}
	raster.FillRect(s.frame, fw, 0, 0, fw, t, c)     // top
	raster.FillRect(s.frame, fw, 0, fh-t, fw, fh, c) // bottom
	raster.FillRect(s.frame, fw, 0, 0, t, fh, c)     // left
	raster.FillRect(s.frame, fw, fw-t, 0, fw, fh, c) // right
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
		data, shouldClear, ok := tb.Session.TakeClipboardWrite()
		if !ok {
			continue
		}
		if shouldClear {
			_ = clipboardWrite(s.app, "")
			continue
		}
		_ = clipboardWrite(s.app, string(data))
	}
}

// uploadAndPresent uploads s.frame as the window's GPU texture — recreating
// it if the framebuffer size changed since the last frame — and presents it.
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

	bg := toRGBA(s.thm.Palette.Background)
	ctx.Clear(float32(bg.R)/255, float32(bg.G)/255, float32(bg.B)/255, 1)
	if err := ctx.PresentTexture(s.tex); err != nil {
		s.app.RequestRedraw()
	}
}

const chromeContentPadDp = 8

func (s *uiState) contentPadDp() int {
	if s.cfg != nil && s.cfg.Window.Padding > 0 {
		return s.cfg.Window.Padding
	}
	return chromeContentPadDp
}

// ensureFonts (re)loads the grid + tab-bar font faces when the scale
// factor or configured font size changes.
func (s *uiState) ensureFonts(scale float64) {
	size := float64(s.cfg.Font.Size)
	if size <= 0 {
		size = 13
	}
	size += s.fontZoomDelta
	if size < fontZoomMinPt {
		size = fontZoomMinPt
	}
	if size > fontZoomMaxPt {
		size = fontZoomMaxPt
	}
	if s.painter.Fonts.Regular != nil && s.fontSizeCurrent == size && s.scale == scale {
		return
	}
	b := loadFontBundle(s.cfg.Font.Family, size, scale, roleMono)
	if !s.cfg.Font.Bold {
		b.Bold, b.BoldItalic = nil, nil
	}
	if !s.cfg.Font.Italic {
		b.Italic, b.BoldItalic = nil, nil
	}
	s.painter.Fonts = b
	s.painter.CellWidth = b.CellW
	s.painter.CellHeight = b.CellH
	s.painter.Ascent = b.Ascent
	s.cellW, s.cellH, s.asc = b.CellW, b.CellH, b.Ascent

	uiSize := s.cfg.UIFont.Size
	if uiSize <= 0 {
		uiSize = 12
	}
	tb := loadFontBundle(s.cfg.UIFont.Family, uiSize, scale, roleUI)
	s.tabBar.Face = tb.Regular
	s.tabBar.Ascent = tb.Ascent
	s.tabBar.Scale = scale

	s.fontSizeCurrent = size
	s.scale = scale

	s.painter.SetSymbolFallback(termview.LoadSymbolFallbackFace(size, scale))
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
	raster.FillRoundRect(s.frame, fw, x0, y0, x1, y1, (x1-x0)/2, toRGBA(s.thm.UI.ScrollbarTrack))

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
	raster.FillRoundRect(s.frame, fw, x0, thumbY, x1, thumbY+thumbH, (x1-x0)/2, toRGBA(s.thm.UI.ScrollbarThumb))
}

// gridSelection converts sess's absolute History()+Screen() selection
// into viewport cell rows for Painter.Paint. Returns inactive when the
// selection is fully scrolled off-screen (session selection is unchanged).
func gridSelection(sess *session.Session) Selection {
	start, end, ok := sess.Selection()
	if !ok {
		return Selection{}
	}
	top := sess.ViewportTopAbsLine()
	sess.Term.RLock()
	cols := sess.Term.Size().C
	rows := sess.Term.Size().R
	sess.Term.RUnlock()
	if rows <= 0 || cols <= 0 {
		return Selection{}
	}
	bottom := top + rows - 1
	if end.AbsLine < top || start.AbsLine > bottom {
		return Selection{}
	}

	viewStart := start
	viewEnd := end
	if viewStart.AbsLine < top {
		viewStart.AbsLine = top
		viewStart.Col = 0
	}
	if viewEnd.AbsLine > bottom {
		viewEnd.AbsLine = bottom
		viewEnd.Col = cols - 1
	}
	sel := Selection{Active: true, Rect: sess.SelectionRect()}
	sel.Start.Col, sel.Start.Row = viewStart.Col, viewStart.AbsLine-top
	sel.End.Col, sel.End.Row = viewEnd.Col, viewEnd.AbsLine-top
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
