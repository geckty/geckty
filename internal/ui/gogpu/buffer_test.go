package gogpu

import (
	"image"
	"image/color"
	"testing"
)

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
	blitImageScaled(buf, 8, 8, src, 2, 2, 4, 4)

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
	blitImageScaled(buf, 2, 2, src, 0, 0, 2, 2)

	if got := pixelAt(buf, 2, 0, 0); got != (color.RGBA{}) {
		t.Fatalf("fully transparent source pixel must not be written, got %v", got)
	}
}

func TestBlitImageScaledClipsToFrameBounds(t *testing.T) {
	buf := newBuf(4, 4)
	src := solidRGBA(1, 1, color.RGBA{R: 0xff, A: 0xff})
	// Dest rect extends far past the 4x4 frame — must not panic or write
	// out of bounds.
	blitImageScaled(buf, 4, 4, src, 2, 2, 100, 100)

	if got := pixelAt(buf, 4, 3, 3); got != (color.RGBA{R: 0xff, A: 0xff}) {
		t.Fatalf("(3,3) = %v, want opaque red (last in-bounds pixel of the clipped dest rect)", got)
	}
}

func TestBlitImageScaledZeroSizeIsNoop(t *testing.T) {
	buf := newBuf(2, 2)
	src := solidRGBA(1, 1, color.RGBA{R: 0xff, A: 0xff})
	blitImageScaled(buf, 2, 2, src, 0, 0, 0, 0)
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("buf[%d] = %d, want 0 (w=h=0 must draw nothing)", i, b)
		}
	}
}
