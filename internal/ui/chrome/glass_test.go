package chrome

import (
	"image"
	"image/color"
	"testing"
)

func TestGlassFillExtremes(t *testing.T) {
	bg := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	if got := glassFill(bg, 0); got != bg {
		t.Fatalf("glassFill(black, 0) = %v, want unchanged", got)
	}
	wantWhite := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if got := glassFill(bg, 1); got != wantWhite {
		t.Fatalf("glassFill(black, 1) = %v, want white", got)
	}
}

func TestGlassFillClamps(t *testing.T) {
	bg := color.NRGBA{R: 10, G: 20, B: 30, A: 200}
	got := glassFill(bg, -1)
	want := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	if got != want {
		t.Fatalf("glassFill(factor=-1) = %v, want %v", got, want)
	}
	got = glassFill(bg, 2)
	if got.R != 255 || got.G != 255 || got.B != 255 || got.A != 255 {
		t.Fatalf("glassFill(factor=2) = %v, want opaque white", got)
	}
}

func TestGlassStyle(t *testing.T) {
	if got := glassStyle(false, false, false); got != glassInactive {
		t.Fatalf("inactive = %v, want %v", got, glassInactive)
	}
	if got := glassStyle(false, true, false); got != glassHover {
		t.Fatalf("hover = %v, want %v", got, glassHover)
	}
	if got := glassStyle(true, false, false); got != glassActive {
		t.Fatalf("active = %v, want %v", got, glassActive)
	}
	if got := glassStyle(false, false, true); got != glassDrag {
		t.Fatalf("dragging = %v, want %v", got, glassDrag)
	}
}

func TestPaintGlassCapsuleSkipsZeroFactor(t *testing.T) {
	// Must not panic — inactive factor paints nothing.
	paintGlassCapsule(nil, color.NRGBA{A: 255}, image.Rect(0, 0, 40, 20), 8, 0, 0xff)
	paintGlassCapsule(nil, color.NRGBA{A: 255}, image.Rectangle{}, 4, glassActive, 0xff)
}

func TestDimFG(t *testing.T) {
	fg := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	bg := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	got := dimFG(fg, bg, 0.5)
	if got.R != 127 && got.R != 128 {
		t.Fatalf("dimFG mid = %v, want ~127 gray", got)
	}
}
