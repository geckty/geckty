package theme

import (
	"image/color"
	"testing"

	"github.com/geckty/geckty/internal/config"
)

func TestNewBuildsFullTheme(t *testing.T) {
	cfg := config.Default()
	thm, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if thm.Palette.Foreground.A == 0 || thm.Palette.Background.A == 0 {
		t.Fatal("palette should be populated")
	}
	if thm.UI.VisualBell.A == 0 {
		t.Fatal("ui tokens should be populated")
	}
	if thm.Glass.Drag == 0 {
		t.Fatal("glass params should be populated")
	}
}

func TestNewRejectsInvalidUIColor(t *testing.T) {
	cfg := config.Default()
	cfg.UI.VisualBell = "not-a-color"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected error for invalid ui.visual_bell")
	}
}

func TestTabFillGlassDraggingUsesGlassDrag(t *testing.T) {
	p := testPalette(t)
	g := DefaultGlass()
	g.Drag = 0.5
	got := p.TabFillGlass(g, false, false, true)
	want := glassFill(p.Background, 0.5)
	if got != want {
		t.Fatalf("TabFillGlass(drag) = %v, want %v", got, want)
	}
}

func TestParseHexAlphaEightDigit(t *testing.T) {
	got, err := parseHexAlpha("#11223344")
	if err != nil {
		t.Fatal(err)
	}
	want := color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x44}
	if got != want {
		t.Fatalf("parseHexAlpha = %v, want %v", got, want)
	}
}

func TestGlassFromConfigOverrides(t *testing.T) {
	v := 0.42
	cfg := config.GlassConfig{Drag: &v}
	got := glassFromConfig(cfg)
	if got.Drag != 0.42 {
		t.Fatalf("Drag = %v", got.Drag)
	}
}
