package app

import (
	"image/color"
	"testing"
)

func TestFrostGlassRoundRectBlursSharpUnderlay(t *testing.T) {
	const w, h = 40, 24
	buf := newBuf(w, h)
	// Checkerboard underlay — sharp contrast that blur must soften.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{A: 0xff}
			if (x/2+y/2)%2 == 0 {
				c.R, c.G, c.B = 0xff, 0xff, 0xff
			}
			off := (y*w + x) * 4
			buf[off], buf[off+1], buf[off+2], buf[off+3] = c.R, c.G, c.B, c.A
		}
	}
	before := pixelAt(buf, w, 20, 12)
	frostGlassRoundRect(buf, w, 8, 4, 32, 20, 6, 3, color.RGBA{R: 0x40, G: 0x40, B: 0x40, A: 0x40})
	after := pixelAt(buf, w, 20, 12)
	if after == before {
		t.Fatalf("center pixel unchanged after frost: %v", after)
	}
	// Blur + tint should pull pure white toward mid-grey, not leave 255.
	if after.R == 0xff && after.G == 0xff && after.B == 0xff {
		t.Fatalf("frost left a sharp white pixel: %v", after)
	}
	// Outside the pill stays sharp black (or white) from the checkerboard.
	out := pixelAt(buf, w, 1, 1)
	if out.R != 0 && out.R != 0xff {
		t.Fatalf("outside pill should stay sharp checkerboard, got %v", out)
	}
}

func TestBoxBlurRGBASoftensNeighbors(t *testing.T) {
	const w, h = 5, 1
	src := make([]byte, w*h*4)
	tmp := make([]byte, len(src))
	// Single white pixel in the middle of black.
	src[2*4], src[2*4+1], src[2*4+2], src[2*4+3] = 255, 255, 255, 255
	boxBlurRGBA(src, tmp, w, h, 1)
	mid := src[2*4]
	side := src[1*4]
	if mid == 0 || side == 0 {
		t.Fatalf("blur should spread energy: mid=%d side=%d", mid, side)
	}
	if mid < side {
		t.Fatalf("peak should be >= neighbor: mid=%d side=%d", mid, side)
	}
}

func TestFrostGlassRoundRectInvalidGeometryIsNoop(_ *testing.T) {
	buf := newBuf(8, 8)
	frostGlassRoundRect(nil, 8, 0, 0, 4, 4, 2, 1, color.RGBA{A: 0xff})
	frostGlassRoundRect(buf, 0, 0, 0, 4, 4, 2, 1, color.RGBA{A: 0xff})
	frostGlassRoundRect(buf, 8, 4, 0, 4, 4, 2, 1, color.RGBA{A: 0xff})
}

func TestFrostGlassRoundRectTranslucentTint(t *testing.T) {
	buf := newBuf(20, 20)
	for i := range buf {
		buf[i] = 0xff
	}
	frostGlassRoundRect(buf, 20, 4, 4, 16, 16, 4, 2, color.RGBA{R: 0x80, G: 0x40, B: 0x20, A: 0x80})
	if pixelAt(buf, 20, 10, 10) == (color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Fatal("translucent frost should tint the underlay")
	}
}

func TestBoxBlurZeroRadiusIsNoop(t *testing.T) {
	src := []byte{1, 2, 3, 4}
	tmp := make([]byte, len(src))
	boxBlurRGBA(src, tmp, 1, 1, 0)
	if src[0] != 1 {
		t.Fatal("r<1 should leave src untouched")
	}
}
