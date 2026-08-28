package app

import (
	"image"
	"testing"

	"github.com/gogpu/gpucontext"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

func TestUseUICompositor(t *testing.T) {
	t.Setenv("GECKTY_UI_COMPOSITOR", "")
	if useUICompositor() {
		t.Fatal("empty env should not enable compositor")
	}
	t.Setenv("GECKTY_UI_COMPOSITOR", "1")
	if !useUICompositor() {
		t.Fatal("GECKTY_UI_COMPOSITOR=1 should enable compositor")
	}
}

func TestPaintForCompositor(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.scale = 1
	s.cellW = 7
	s.cellH = 13

	img := s.paintForCompositor(1, 200, 100)
	if img == nil {
		t.Fatal("paintForCompositor returned nil")
	}
	if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 100 {
		t.Fatalf("bounds = %v, want 200x100", img.Bounds())
	}
	if len(s.frame) != 200*100*4 {
		t.Fatalf("len(frame) = %d, want %d", len(s.frame), 200*100*4)
	}
}

func TestCompositorFrameLayout(t *testing.T) {
	w := &compositorFrame{state: &uiState{}}
	ctx := widget.NewContext()
	size := w.Layout(ctx, geometry.Constraints{
		MinWidth: 100, MinHeight: 50,
		MaxWidth: 800, MaxHeight: 600,
	})
	if size.Width != 800 || size.Height != 600 {
		t.Fatalf("Layout = %v, want 800x600 (Biggest)", size)
	}
}

func TestCompositorAppRequestRedrawInvalidatesUI(t *testing.T) {
	wp := &compositorTestWindowProvider{width: 800, height: 600, scale: 1}
	pp := &compositorTestPlatformProvider{}
	uiApp := uiapp.New(
		uiapp.WithWindowProvider(wp),
		uiapp.WithPlatformProvider(pp),
	)
	bridge := &uiCompositorBridge{uiApp: uiApp}
	inner := newFakeApp()
	capp := &compositorApp{inner: inner, bridge: bridge}

	capp.RequestRedraw()
	if inner.redrawCount.Load() != 1 {
		t.Fatalf("inner redrawCount = %d, want 1", inner.redrawCount.Load())
	}
	if !uiApp.Window().Context().IsInvalidated() {
		t.Fatal("compositor RequestRedraw should invalidate ui context")
	}
}

func TestWireCompositorWindowRegistersCallbacks(t *testing.T) {
	s, app := testUIState(t)
	win := &fakeWindow{}
	s.app = app
	s.wireCompositorWindow(win)

	if win.onDraw != nil {
		t.Fatal("wireCompositorWindow must not register OnDraw (desktop.Run owns it)")
	}
	if win.onResize == nil || win.onKeyPress == nil || win.onTextInput == nil ||
		win.onPointer == nil || win.onScroll == nil || win.onClose == nil {
		t.Fatal("wireCompositorWindow should register input/resize callbacks")
	}

	win.onResize(100, 100)
	if app.redrawCount.Load() != 1 {
		t.Fatalf("resize should RequestRedraw, got %d", app.redrawCount.Load())
	}
	win.onKeyPress(gpucontext.KeyA, 0)
	win.onTextInput("x")
}

func TestWireCompositorWindowConfirmClose(t *testing.T) {
	s, app := testUIStateWithTab(t)
	s.cfg.Window.ConfirmClose = true
	if err := s.newTab(); err != nil {
		t.Fatalf("newTab: %v", err)
	}
	win := &fakeWindow{}
	s.app = app
	s.wireCompositorWindow(win)
	if win.onClose() {
		t.Fatal("multi-tab confirm close should block window close")
	}
	if !s.confirmClose {
		t.Fatal("expected confirmClose overlay")
	}
}

func TestWireLifecycleCallbacksCompositor(t *testing.T) {
	s, app := testUIStateWithTab(t)
	s.wireLifecycleCallbacksCompositor(app)
	if app.onFocusFn == nil || app.onCloseFn == nil || app.onSurfaceAvailableFn == nil {
		t.Fatal("wireLifecycleCallbacksCompositor should register callbacks")
	}
	app.onFocusFn(true)
	app.onCloseFn()
	app.onSurfaceAvailableFn()
}

func TestCompositorAppDelegates(t *testing.T) {
	inner := newFakeApp()
	inner.clipboard = "clip"
	capp := &compositorApp{inner: inner, bridge: nil}

	if err := capp.Run(); err != nil {
		t.Fatal(err)
	}
	capp.Quit()
	if !inner.quit.Load() {
		t.Fatal("Quit should delegate")
	}
	inner.inSizeMove.Store(true)
	if !capp.InSizeMove() {
		t.Fatal("InSizeMove should delegate")
	}
	capp.OnFocus(nil)
	capp.OnClose(nil)
	capp.OnSurfaceAvailable(nil)
	if capp.PrimaryWindow() != nil {
		t.Fatal("fake PrimaryWindow is nil")
	}
	if capp.EventSource() != inner.events {
		t.Fatal("EventSource should delegate")
	}
	if err := capp.ClipboardWrite("x"); err != nil || inner.clipboard != "x" {
		t.Fatalf("ClipboardWrite = %q, err=%v", inner.clipboard, err)
	}
	if got, err := capp.ClipboardRead(); err != nil || got != "x" {
		t.Fatalf("ClipboardRead = %q, err=%v", got, err)
	}
}

func TestUICompositorBridgeInvalidateNilSafe(_ *testing.T) {
	var b *uiCompositorBridge
	b.invalidate()
	b = &uiCompositorBridge{}
	b.invalidate()
}

func TestFrameRGBAView(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	if s.frameRGBAView(0, 0) != nil {
		t.Fatal("zero size should return nil")
	}
	s.frame = make([]byte, 40) // too small for 10x10
	if s.frameRGBAView(10, 10) != nil {
		t.Fatal("undersized frame should return nil")
	}
	s.paintForCompositor(1, 10, 10)
	img := s.frameRGBAView(10, 10)
	if img == nil || img.Bounds().Dx() != 10 {
		t.Fatalf("frameRGBAView = %v", img)
	}
	img2 := s.frameRGBAView(20, 5)
	if img2 == nil || img2.Bounds().Dx() != 20 || img2.Bounds().Dy() != 5 {
		t.Fatalf("realloc frameRGBAView = %v", img2)
	}
}

func TestPaintForCompositorInvalidSize(t *testing.T) {
	s, app := testUIStateWithTab(t)
	if s.paintForCompositor(1, 0, 100) != nil {
		t.Fatal("zero width should return nil")
	}
	if app.redrawCount.Load() == 0 {
		t.Fatal("invalid size should RequestRedraw")
	}
}

func TestCompositorFrameDraw(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.scale = 1
	s.cellW = 7
	s.cellH = 13
	w := &compositorFrame{state: s}
	w.SetBounds(geometry.NewRect(0, 0, 100, 50))
	ctx := widget.NewContext()
	canvas := &drawImageCanvas{}
	w.Draw(ctx, canvas)
	if !canvas.drew || canvas.at != geometry.Pt(0, 0) {
		t.Fatal("Draw should blit painted frame via DrawImage")
	}
}

func TestCompositorFrameEvent(t *testing.T) {
	var f compositorFrame
	ctx := widget.NewContext()
	if f.Event(ctx, &event.KeyEvent{}) {
		t.Fatal("compositorFrame should not consume events")
	}
}

// drawImageCanvas is a minimal widget.Canvas that records DrawImage calls.
type drawImageCanvas struct {
	drew bool
	at   geometry.Point
}

func (drawImageCanvas) Clear(widget.Color)                                            {}
func (drawImageCanvas) DrawRect(geometry.Rect, widget.Color)                          {}
func (drawImageCanvas) FillRectDirect(geometry.Rect, widget.Color)                    {}
func (drawImageCanvas) StrokeRect(geometry.Rect, widget.Color, float32)               {}
func (drawImageCanvas) DrawRoundRect(geometry.Rect, widget.Color, float32)            {}
func (drawImageCanvas) StrokeRoundRect(geometry.Rect, widget.Color, float32, float32) {}
func (drawImageCanvas) DrawCircle(geometry.Point, float32, widget.Color)              {}
func (drawImageCanvas) StrokeCircle(geometry.Point, float32, widget.Color, float32)   {}
func (drawImageCanvas) StrokeArc(geometry.Point, float32, float64, float64, widget.Color, float32) {
}
func (drawImageCanvas) DrawLine(geometry.Point, geometry.Point, widget.Color, float32) {}
func (drawImageCanvas) DrawText(string, geometry.Rect, float32, widget.Color, bool, widget.TextAlign) {
}
func (drawImageCanvas) MeasureText(string, float32, bool) float32 { return 0 }
func (c *drawImageCanvas) DrawImage(_ image.Image, at geometry.Point) {
	c.drew = true
	c.at = at
}
func (drawImageCanvas) PushClip(geometry.Rect)                   {}
func (drawImageCanvas) PushClipRoundRect(geometry.Rect, float32) {}
func (drawImageCanvas) PopClip()                                 {}
func (drawImageCanvas) PushTransform(geometry.Point)             {}
func (drawImageCanvas) PopTransform()                            {}
func (drawImageCanvas) TransformOffset() geometry.Point          { return geometry.Point{} }
func (drawImageCanvas) ScreenOriginBase() geometry.Point         { return geometry.Point{} }
func (drawImageCanvas) ClipBounds() geometry.Rect                { return geometry.NewRect(0, 0, 10000, 10000) }
func (drawImageCanvas) ReplayScene(widget.SceneCache)            {}

var _ widget.Canvas = (*drawImageCanvas)(nil)

type compositorTestWindowProvider struct {
	width, height int
	scale         float64
}

func (m *compositorTestWindowProvider) Size() (int, int) { return m.width, m.height }
func (m *compositorTestWindowProvider) ScaleFactor() float64 {
	return m.scale
}
func (m *compositorTestWindowProvider) RequestRedraw() {}

type compositorTestPlatformProvider struct{}

func (m *compositorTestPlatformProvider) ClipboardRead() (string, error)   { return "", nil }
func (m *compositorTestPlatformProvider) ClipboardWrite(string) error      { return nil }
func (m *compositorTestPlatformProvider) SetCursor(gpucontext.CursorShape) {}
func (m *compositorTestPlatformProvider) DarkMode() bool                   { return false }
func (m *compositorTestPlatformProvider) ReduceMotion() bool               { return false }
func (m *compositorTestPlatformProvider) HighContrast() bool               { return false }
func (m *compositorTestPlatformProvider) FontScale() float32               { return 1 }
func (m *compositorTestPlatformProvider) SubpixelLayout() gpucontext.SubpixelLayout {
	return gpucontext.SubpixelNone
}
func (m *compositorTestPlatformProvider) FontSmoothing() gpucontext.FontSmoothing {
	return gpucontext.FontSmoothingGrayscale
}
