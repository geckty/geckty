package chrome

import (
	"image/color"
	"testing"
)

func TestGlassFillExtremes(t *testing.T) {
	bg := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	if got := GlassFill(bg, 0); got != bg {
		t.Fatalf("GlassFill(black, 0) = %v, want unchanged", got)
	}
	wantWhite := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if got := GlassFill(bg, 1); got != wantWhite {
		t.Fatalf("GlassFill(black, 1) = %v, want white", got)
	}
}

func TestGlassFillClamps(t *testing.T) {
	bg := color.NRGBA{R: 10, G: 20, B: 30, A: 200}
	got := GlassFill(bg, -1)
	want := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	if got != want {
		t.Fatalf("GlassFill(factor=-1) = %v, want %v", got, want)
	}
	got = GlassFill(bg, 2)
	if got.R != 255 || got.G != 255 || got.B != 255 || got.A != 255 {
		t.Fatalf("GlassFill(factor=2) = %v, want opaque white", got)
	}
}

func TestGlassStyle(t *testing.T) {
	if got := GlassStyle(false, false, false); got != GlassInactive {
		t.Fatalf("inactive = %v, want %v", got, GlassInactive)
	}
	if got := GlassStyle(false, true, false); got != GlassHover {
		t.Fatalf("hover = %v, want %v", got, GlassHover)
	}
	if got := GlassStyle(true, false, false); got != GlassActive {
		t.Fatalf("active = %v, want %v", got, GlassActive)
	}
	if got := GlassStyle(false, false, true); got != GlassDrag {
		t.Fatalf("dragging = %v, want %v", got, GlassDrag)
	}
}

func TestDimFG(t *testing.T) {
	fg := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	bg := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	got := DimFG(fg, bg, 0.5)
	if got.R != 127 && got.R != 128 {
		t.Fatalf("DimFG mid = %v, want ~127 gray", got)
	}
}
