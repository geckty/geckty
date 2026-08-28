package raster

import (
	"image"
	"image/color"
	"testing"
)

func newBuf(w, h int) []byte { return make([]byte, w*h*4) }

func pixelAt(buf []byte, frameW, x, y int) color.RGBA {
	off := (y*frameW + x) * 4
	return color.RGBA{R: buf[off], G: buf[off+1], B: buf[off+2], A: buf[off+3]}
}

func solidRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestBlitImageScaledFillsDestRegion(t *testing.T) {
	buf := newBuf(8, 8)
	src := solidRGBA(2, 2, color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff})
	BlitImageScaled(buf, 8, 8, src, 2, 2, 4, 4)

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			inside := x >= 2 && x < 6 && y >= 2 && y < 6
			got := pixelAt(buf, 8, x, y)
			if inside && got != (color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff}) {
				t.Fatalf("(%d,%d) = %v, want opaque src color inside dest rect", x, y, got)
			}
			if !inside && got != (color.RGBA{}) {
				t.Fatalf("(%d,%d) = %v, want zero outside dest rect", x, y, got)
			}
		}
	}
}

func TestBlitImageScaledSkipsFullyTransparentSourcePixels(t *testing.T) {
	buf := newBuf(2, 2)
	src := solidRGBA(2, 2, color.RGBA{}) // fully transparent
	BlitImageScaled(buf, 2, 2, src, 0, 0, 2, 2)

	if got := pixelAt(buf, 2, 0, 0); got != (color.RGBA{}) {
		t.Fatalf("fully transparent source pixel must not be written, got %v", got)
	}
}

func TestBlitImageScaledClipsToFrameBounds(t *testing.T) {
	buf := newBuf(4, 4)
	src := solidRGBA(1, 1, color.RGBA{R: 0xff, A: 0xff})
	// Dest rect extends far past the 4x4 frame — must not panic or write
	// out of bounds.
	BlitImageScaled(buf, 4, 4, src, 2, 2, 100, 100)

	if got := pixelAt(buf, 4, 3, 3); got != (color.RGBA{R: 0xff, A: 0xff}) {
		t.Fatalf("(3,3) = %v, want opaque red (last in-bounds pixel of the clipped dest rect)", got)
	}
}

func TestBlitImageScaledZeroSizeIsNoop(t *testing.T) {
	buf := newBuf(2, 2)
	src := solidRGBA(1, 1, color.RGBA{R: 0xff, A: 0xff})
	BlitImageScaled(buf, 2, 2, src, 0, 0, 0, 0)
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("buf[%d] = %d, want 0 (w=h=0 must draw nothing)", i, b)
		}
	}
}

func TestFillRectFillsRegion(t *testing.T) {
	buf := newBuf(4, 4)
	c := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	FillRect(buf, 4, 1, 1, 3, 3, c)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			inside := x >= 1 && x < 3 && y >= 1 && y < 3
			got := pixelAt(buf, 4, x, y)
			if inside && got != c {
				t.Fatalf("(%d,%d) = %v, want %v", x, y, got, c)
			}
			if !inside && got != (color.RGBA{}) {
				t.Fatalf("(%d,%d) = %v, want zero outside rect", x, y, got)
			}
		}
	}
}

func TestBlendRectTranslucent(t *testing.T) {
	buf := newBuf(2, 2)
	FillRect(buf, 2, 0, 0, 2, 2, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	BlendRect(buf, 2, 0, 0, 1, 1, color.RGBA{R: 255, G: 0, B: 0, A: 128})
	got := pixelAt(buf, 2, 0, 0)
	if got.R == 0 || got.R == 255 {
		t.Fatalf("expected blended red, got %v", got)
	}
}

func TestBlitGlyphClippedDrawsForeground(t *testing.T) {
	buf := newBuf(4, 4)
	mask := image.NewAlpha(image.Rect(0, 0, 2, 2))
	mask.SetAlpha(0, 0, color.Alpha{A: 255})
	dr := image.Rect(1, 1, 3, 3)
	fg := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	BlitGlyphClipped(buf, 4, 4, dr, mask, image.Point{}, fg, 0, 0, 4, 4)
	got := pixelAt(buf, 4, 1, 1)
	if got.R != 10 || got.G != 20 || got.B != 30 {
		t.Fatalf("glyph pixel = %v", got)
	}
}
