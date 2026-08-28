package app

import (
	"testing"

	"github.com/gogpu/gpucontext"
	uiapp "github.com/gogpu/ui/app"
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
}

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
