package gogpu

import (
	"image/color"
	"testing"
)

func TestFillRoundRectOpaqueFillsInteriorMinusCorners(t *testing.T) {
	buf := newBuf(10, 10)
	red := color.RGBA{R: 0xff, A: 0xff}
	fillRoundRect(buf, 10, 0, 0, 10, 10, 3, red)

	// Dead center must be fully covered.
	if got := pixelAt(buf, 10, 5, 5); got != red {
		t.Fatalf("center pixel = %v, want %v", got, red)
	}
	// The extreme corner (0,0) sits outside a radius-3 rounded corner and
	// must be left untouched.
	if got := pixelAt(buf, 10, 0, 0); got != (color.RGBA{}) {
		t.Fatalf("corner pixel (0,0) = %v, want zero (outside the rounded corner)", got)
	}
}

func TestFillRoundRectZeroRadiusDegeneratesToPlainRect(t *testing.T) {
	buf := newBuf(6, 6)
	blue := color.RGBA{B: 0xff, A: 0xff}
	fillRoundRect(buf, 6, 1, 1, 5, 5, 0, blue)

	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			inside := x >= 1 && x < 5 && y >= 1 && y < 5
			got := pixelAt(buf, 6, x, y)
			if inside && got != blue {
				t.Fatalf("(%d,%d) = %v, want %v (radius=0 should fill the full rect like fillRect)", x, y, got, blue)
			}
			if !inside && got != (color.RGBA{}) {
				t.Fatalf("(%d,%d) = %v, want zero (outside the rect)", x, y, got)
			}
		}
	}
}

func TestFillRoundRectTranslucentBlendsWithExistingContent(t *testing.T) {
	buf := newBuf(4, 4)
	// Pre-fill with opaque green so a translucent overlay has something to
	// blend against — this is the drag "glass" effect's exact scenario.
	fillRect(buf, 4, 0, 0, 4, 4, color.RGBA{G: 0xff, A: 0xff})

	half := color.RGBA{R: 0xff, A: 0x80}
	fillRoundRect(buf, 4, 0, 0, 4, 4, 0, half)

	got := pixelAt(buf, 4, 1, 1)
	if got.A != 255 {
		t.Fatalf("blended pixel alpha = %d, want 255 (result is always opaque)", got.A)
	}
	if got.R == 0 || got.G == 0 {
		t.Fatalf("blended pixel = %v, want both R and G contributions (translucent red over opaque green)", got)
	}
}

func TestFillDiagonalCrossIsSymmetricAroundCenter(t *testing.T) {
	buf := newBuf(21, 21)
	fg := color.RGBA{R: 0xff, A: 0xff}
	fillDiagonalCross(buf, 21, 10, 10, 8, 2, fg)

	if got := pixelAt(buf, 21, 10, 10); got != fg {
		t.Fatalf("center pixel = %v, want %v (the two diagonals cross exactly at center)", got, fg)
	}
	// A point far from both diagonals should be untouched.
	if got := pixelAt(buf, 21, 10, 2); got != (color.RGBA{}) {
		t.Fatalf("(10,2) = %v, want zero (outside both diagonal bars)", got)
	}
}

func TestFillDiagonalCrossZeroArmLenIsNoop(t *testing.T) {
	buf := newBuf(6, 6)
	fillDiagonalCross(buf, 6, 3, 3, 0, 2, color.RGBA{R: 0xff, A: 0xff})
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("buf[%d] = %d, want 0 (armLen<=0 must draw nothing)", i, b)
		}
	}
}

func TestRoundRectCoverageFullyInsideIsOne(t *testing.T) {
	if got := roundRectCoverage(5, 5, 0, 0, 10, 10, 3); got != 1 {
		t.Fatalf("coverage at dead center = %v, want 1", got)
	}
}

func TestRoundRectCoverageOutsideBoundsIsZero(t *testing.T) {
	if got := roundRectCoverage(-1, 5, 0, 0, 10, 10, 3); got != 0 {
		t.Fatalf("coverage outside x bounds = %v, want 0", got)
	}
	if got := roundRectCoverage(5, 20, 0, 0, 10, 10, 3); got != 0 {
		t.Fatalf("coverage outside y bounds = %v, want 0", got)
	}
}

func TestRoundRectCoverageBeyondCornerRadiusIsZero(t *testing.T) {
	// (0.5,0.5) sits in the extreme corner, well outside a radius-3 arc
	// centered at (3,3).
	if got := roundRectCoverage(0.5, 0.5, 0, 0, 10, 10, 3); got != 0 {
		t.Fatalf("coverage at extreme corner = %v, want 0", got)
	}
}
