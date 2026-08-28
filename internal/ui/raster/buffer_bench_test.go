package raster

import (
	"image"
	"image/color"
	"testing"
)

func benchLegacyBlit(b *testing.B, maskW, maskH int) {
	buf := make([]byte, 640*480*4)
	mask := image.NewAlpha(image.Rect(0, 0, maskW, maskH))
	for i := range mask.Pix {
		if i%3 == 0 {
			mask.Pix[i] = 255
		} else if i%3 == 1 {
			mask.Pix[i] = 128
		}
	}
	dr := image.Rect(10, 10, 10+maskW, 10+maskH)
	fg := color.RGBA{R: 200, G: 220, B: 240, A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blitLegacyMaskClipped(buf, 640, 480, mask, image.Point{}, dr, fg, 0, 0, 640, 480)
	}
}

func BenchmarkBlitLegacyMask8x16(b *testing.B)  { benchLegacyBlit(b, 8, 16) }
func BenchmarkBlitLegacyMask12x20(b *testing.B) { benchLegacyBlit(b, 12, 20) }
