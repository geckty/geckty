package gogpu

import (
	"image/color"
	"math"
)

// fillRoundRect fills a rounded rectangle with 1px analytic-AA corners —
// a CPU-side rounded-rect fill into the RGBA frame buffer, without any
// GPU clip primitive (bounds-checked pixel math instead).
//
// c.A is honored as real translucency (src-over blend against whatever is
// already in buf at that pixel — the tab-bar background or a tab sliding
// underneath), not just written through: a fully-opaque fill (A=255, the
// common case for non-dragged tabs) still hard-overwrites interior pixels
// exactly as before, but a translucent one (the dragged tab's frosted-
// glass look) actually lets the buffer content beneath it show through.
func fillRoundRect(buf []byte, frameW, x0, y0, x1, y1, radius int, c color.RGBA) {
	if frameW <= 0 || len(buf) == 0 || x0 >= x1 || y0 >= y1 {
		return
	}
	frameH := len(buf) / (frameW * 4)
	if radius <= 0 {
		blendRect(buf, frameW, x0, y0, x1, y1, c)
		return
	}
	cx0, cy0, cx1, cy1 := x0, y0, x1, y1
	if cx0 < 0 {
		cx0 = 0
	}
	if cy0 < 0 {
		cy0 = 0
	}
	if cx1 > frameW {
		cx1 = frameW
	}
	if cy1 > frameH {
		cy1 = frameH
	}
	if cx0 >= cx1 || cy0 >= cy1 {
		return
	}
	stride := frameW * 4
	srcA := float64(c.A) / 255
	for y := cy0; y < cy1; y++ {
		for x := cx0; x < cx1; x++ {
			cov := roundRectCoverage(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1, radius)
			if cov <= 0 {
				continue
			}
			// cov and srcA are both in [0,1], so alpha*255+0.5 is in
			// [0,255.5) — safe to truncate into uint8 without clamping.
			alpha := uint8(cov*srcA*255 + 0.5)
			if alpha == 0 {
				continue
			}
			blendPixel(buf, y*stride+x*4, c.R, c.G, c.B, alpha)
		}
	}
}

// blendRect is fillRect's translucency-aware counterpart — used by
// fillRoundRect when radius<=0 so a translucent caller still blends
// instead of hard-overwriting.
func blendRect(buf []byte, frameW, x0, y0, x1, y1 int, c color.RGBA) {
	if c.A == 255 {
		fillRect(buf, frameW, x0, y0, x1, y1, c)
		return
	}
	if frameW <= 0 || len(buf) == 0 {
		return
	}
	frameH := len(buf) / (frameW * 4)
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > frameW {
		x1 = frameW
	}
	if y1 > frameH {
		y1 = frameH
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	stride := frameW * 4
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			blendPixel(buf, y*stride+x*4, c.R, c.G, c.B, c.A)
		}
	}
}

// roundRectCoverage returns how much of the pixel centered at (px,py) falls
// inside the rounded rect [x0,y0,x1,y1) with the given corner radius —
// 0 (fully outside) to 1 (fully inside), with a 1px linear AA band at the
// corner arcs.
func roundRectCoverage(px, py float64, x0, y0, x1, y1, radius int) float64 {
	fx0, fy0, fx1, fy1 := float64(x0), float64(y0), float64(x1), float64(y1)
	if px < fx0 || px >= fx1 || py < fy0 || py >= fy1 {
		return 0
	}
	r := float64(radius)
	inCornerX := px < fx0+r || px > fx1-r
	inCornerY := py < fy0+r || py > fy1-r
	if !inCornerX || !inCornerY {
		return 1
	}
	cx := px
	switch {
	case cx < fx0+r:
		cx = fx0 + r
	case cx > fx1-r:
		cx = fx1 - r
	}
	cy := py
	switch {
	case cy < fy0+r:
		cy = fy0 + r
	case cy > fy1-r:
		cy = fy1 - r
	}
	dx, dy := px-cx, py-cy
	dist := math.Sqrt(dx*dx + dy*dy)
	switch {
	case dist <= r-0.5:
		return 1
	case dist >= r+0.5:
		return 0
	default:
		return r + 0.5 - dist
	}
}

// fillDiagonalCross draws an "X" (two crossing diagonal bars) centered at
// (cx,cy) with the given per-arm half-length and thickness, 1px AA edges.
// Used for the tab close button instead of a font glyph — a font's "✕"
// can sit off-center vertically depending on its own glyph metrics; a
// vector cross is centered by construction regardless of font.
func fillDiagonalCross(buf []byte, frameW, cx, cy, armLen, thickness int, c color.RGBA) {
	if frameW <= 0 || len(buf) == 0 || armLen <= 0 {
		return
	}
	frameH := len(buf) / (frameW * 4)
	x0, y0 := cx-armLen, cy-armLen
	x1, y1 := cx+armLen, cy+armLen
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > frameW {
		x1 = frameW
	}
	if y1 > frameH {
		y1 = frameH
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	half := float64(thickness) / 2
	stride := frameW * 4
	const sqrt2 = math.Sqrt2
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			dx, dy := float64(x-cx)+0.5, float64(y-cy)+0.5
			d1 := math.Abs(dx-dy) / sqrt2
			d2 := math.Abs(dx+dy) / sqrt2
			d := math.Min(d1, d2)
			cov := half + 0.5 - d
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			off := y*stride + x*4
			inv := 1 - cov
			buf[off] = uint8(float64(c.R)*cov + float64(buf[off])*inv)
			buf[off+1] = uint8(float64(c.G)*cov + float64(buf[off+1])*inv)
			buf[off+2] = uint8(float64(c.B)*cov + float64(buf[off+2])*inv)
			buf[off+3] = 255
		}
	}
}
