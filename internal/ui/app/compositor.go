package app

import (
	"fmt"
	"image"
	"math"
	"os"

	gogpulib "github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/desktop"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/protocol/focus"
)

// useUICompositor reports whether geckty should use gogpu/ui desktop.Run
// (ADR-007 compositor) instead of the legacy PresentTexture path.
// Opt-in: GECKTY_UI_COMPOSITOR=1
func useUICompositor() bool {
	return os.Getenv("GECKTY_UI_COMPOSITOR") == "1"
}

// uiCompositorBridge holds the gogpu/ui app used in compositor mode.
type uiCompositorBridge struct {
	uiApp *uiapp.App
}

func (b *uiCompositorBridge) invalidate() {
	if b == nil || b.uiApp == nil {
		return
	}
	b.uiApp.Window().Context().Invalidate()
}

// compositorApp wraps gpuApp so every RequestRedraw also invalidates the
// ui widget tree (required for desktop.Run frame-skip / dirty tracking).
type compositorApp struct {
	inner  gpuApp
	bridge *uiCompositorBridge
}

func (c *compositorApp) Run() error       { return c.inner.Run() }
func (c *compositorApp) Quit()            { c.inner.Quit() }
func (c *compositorApp) InSizeMove() bool { return c.inner.InSizeMove() }
func (c *compositorApp) OnFocus(fn func(bool)) *gogpulib.App {
	return c.inner.OnFocus(fn)
}
func (c *compositorApp) OnClose(fn func()) *gogpulib.App { return c.inner.OnClose(fn) }
func (c *compositorApp) OnSurfaceAvailable(fn func()) *gogpulib.App {
	return c.inner.OnSurfaceAvailable(fn)
}
func (c *compositorApp) PrimaryWindow() *gogpulib.Window { return c.inner.PrimaryWindow() }
func (c *compositorApp) EventSource() gpucontext.EventSource {
	return c.inner.EventSource()
}
func (c *compositorApp) ClipboardWrite(text string) error { return c.inner.ClipboardWrite(text) }
func (c *compositorApp) ClipboardRead() (string, error)   { return c.inner.ClipboardRead() }

func (c *compositorApp) RequestRedraw() {
	c.inner.RequestRedraw()
	if c.bridge != nil {
		c.bridge.invalidate()
	}
}

// compositorFrame is the root leaf widget: it runs geckty's existing paintFrame
// and presents the result via canvas.DrawImage inside a RepaintBoundary.
type compositorFrame struct {
	widget.WidgetBase
	state *uiState
}

func (w *compositorFrame) Layout(_ widget.Context, c geometry.Constraints) geometry.Size {
	return c.Biggest()
}

func (w *compositorFrame) Draw(ctx widget.Context, canvas widget.Canvas) {
	fw := int(math.Ceil(float64(w.Bounds().Width())))
	fh := int(math.Ceil(float64(w.Bounds().Height())))
	if img := w.state.paintForCompositor(float64(ctx.Scale()), fw, fh); img != nil {
		canvas.DrawImage(img, w.Bounds().Min)
	}
}

func (*compositorFrame) Event(widget.Context, event.Event) bool { return false }

var (
	_ widget.Widget = (*compositorFrame)(nil)
	_ gpuApp        = (*compositorApp)(nil)
)

// paintForCompositor runs the same paint pipeline as onDraw but returns an
// RGBA view of s.frame for the ui compositor (no PresentTexture).
func (s *uiState) paintForCompositor(scale float64, fw, fh int) *image.RGBA {
	s.applyPendingConfig()
	s.ensureFonts(scale)
	if fw <= 0 || fh <= 0 || s.cellW <= 0 || s.cellH <= 0 {
		s.app.RequestRedraw()
		return nil
	}

	tabBarH := s.tabBarHeightPx()
	padPx := dpToPx(s.contentPadDp(), scale)
	newCols, newRows := gridSize(image.Pt(fw-2*padPx, fh-tabBarH-2*padPx), s.cellW, s.cellH)

	s.paintFrame(fw, fh, tabBarH, padPx)
	s.drainBells()
	inLiveResize := s.app.InSizeMove()
	needFinalSync := s.consumeLiveResizeSync(inLiveResize)
	s.triggerResizeIfNeeded(newCols, newRows, inLiveResize, needFinalSync)
	s.drainClipboardWrites()

	return s.frameRGBAView(fw, fh)
}

func (s *uiState) frameRGBAView(fw, fh int) *image.RGBA {
	need := fw * fh * 4
	if need <= 0 || len(s.frame) < need {
		return nil
	}
	if s.frameRGBA == nil || s.frameRGBA.Bounds().Dx() != fw || s.frameRGBA.Bounds().Dy() != fh {
		s.frameRGBA = image.NewRGBA(image.Rect(0, 0, fw, fh))
	}
	copy(s.frameRGBA.Pix, s.frame[:need])
	return s.frameRGBA
}

func runWithUICompositor(cfg *config.Config) error {
	thm, err := buildTheme(cfg)
	if err != nil {
		return err
	}
	keymap, err := NewKeymap(cfg.Keybindings)
	if err != nil {
		return err
	}

	s, gapp := buildUIState(cfg, thm, keymap)
	gpu, ok := gapp.(*gogpulib.App)
	if !ok {
		return fmt.Errorf("compositor mode requires *gogpulib.App")
	}

	uiApp := uiapp.New(
		uiapp.WithWindowProvider(gpu),
		uiapp.WithPlatformProvider(gpu),
		uiapp.WithEventSource(gpu.EventSource()),
	)
	frame := &compositorFrame{state: s}
	root := primitives.NewRepaintBoundary(frame, primitives.WithDebugLabel("geckty-root"))
	uiApp.SetRoot(root)

	bridge := &uiCompositorBridge{uiApp: uiApp}
	s.compositor = bridge
	s.app = &compositorApp{inner: gpu, bridge: bridge}

	s.wireSessionManager(s.app)
	defer s.resizeDebouncer.Stop()

	if err := s.wireFirstTab(s.app); err != nil {
		return err
	}

	s.pluginHost, s.stopPlugins = loadPlugins(cfg, s.app)
	defer s.stopPlugins()

	stopBlink := s.startBlinkLoop(s.app)
	defer stopBlink()

	stopConfigReload := s.wireConfigReload(s.app)
	defer stopConfigReload()

	s.wireLifecycleCallbacksCompositor(s.app)
	s.wireEventSource(s.app)

	stopRC := s.wireRemoteControl()
	defer stopRC()

	return desktop.Run(gpu, uiApp)
}

func (s *uiState) wireLifecycleCallbacksCompositor(gapp gpuApp) {
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
		s.wireCompositorWindow(win)
	})
}

func (s *uiState) wireCompositorWindow(win gpuWindow) {
	s.win = win
	// desktop.Run owns OnDraw — do not register s.onDraw.
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
			return false
		}
		return true
	})
}
