package raster

import (
	"image/color"
	"math"
)

// FillRoundRect fills a rounded rectangle with analytic AA on every edge
// (not only the corner arcs) via a signed-distance field.
//
// c.A is honored as real translucency (src-over blend against whatever is
// already in buf at that pixel).
func FillRoundRect(buf []byte, frameW, x0, y0, x1, y1, radius int, c color.RGBA) {
	if frameW <= 0 || len(buf) == 0 || x0 >= x1 || y0 >= y1 {
		return
	}
	frameH := len(buf) / (frameW * 4)
	if radius <= 0 {
		BlendRect(buf, frameW, x0, y0, x1, y1, c)
		return
	}
	cx0, cy0, cx1, cy1 := x0-1, y0-1, x1+1, y1+1
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
			cov := RoundRectCoverage(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1, radius)
			if cov <= 0 {
				continue
			}
			alpha := uint8(cov*srcA*255 + 0.5)
			if alpha == 0 {
				continue
			}
			BlendPixel(buf, y*stride+x*4, c.R, c.G, c.B, alpha)
		}
	}
}

func roundRectSDF(px, py float64, x0, y0, x1, y1, radius int) float64 {
	r := float64(radius)
	hw := float64(x1-x0) / 2
	hh := float64(y1-y0) / 2
	if hw < 0 {
		hw = 0
	}
	if hh < 0 {
		hh = 0
	}
	if r > hw {
		r = hw
	}
	if r > hh {
		r = hh
	}
	cx := float64(x0+x1) / 2
	cy := float64(y0+y1) / 2
	bx := hw - r
	by := hh - r
	dx := math.Abs(px-cx) - bx
	dy := math.Abs(py-cy) - by
	ax := math.Max(dx, 0)
	ay := math.Max(dy, 0)
	return math.Min(math.Max(dx, dy), 0) + math.Hypot(ax, ay) - r
}

// RoundRectCoverage returns how much of the pixel centered at (px,py) falls
// inside the rounded rect — smooth 1px AA on flats and corners alike.
func RoundRectCoverage(px, py float64, x0, y0, x1, y1, radius int) float64 {
	d := roundRectSDF(px, py, x0, y0, x1, y1, radius)
	cov := 0.5 - d
	if cov <= 0 {
		return 0
	}
	if cov >= 1 {
		return 1
	}
	return cov
}

// StrokeRoundRect draws a smooth ~1px AA outline centered on the rounded
// rect boundary (half inside / half outside).
func StrokeRoundRect(buf []byte, frameW, x0, y0, x1, y1, radius int, c color.RGBA) {
	if frameW <= 0 || len(buf) == 0 || x0 >= x1 || y0 >= y1 || c.A == 0 {
		return
	}
	frameH := len(buf) / (frameW * 4)
	cx0, cy0, cx1, cy1 := x0-2, y0-2, x1+2, y1+2
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
	const halfW = 0.6
	stride := frameW * 4
	srcA := float64(c.A) / 255
	for y := cy0; y < cy1; y++ {
		for x := cx0; x < cx1; x++ {
			d := math.Abs(roundRectSDF(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1, radius))
			ring := halfW + 0.5 - d
			if ring <= 0 {
				continue
			}
			if ring > 1 {
				ring = 1
			}
			alpha := uint8(ring*srcA*255 + 0.5)
			if alpha == 0 {
				continue
			}
			BlendPixel(buf, y*stride+x*4, c.R, c.G, c.B, alpha)
		}
	}
}

// FillDiagonalCross draws an "X" centered at (cx,cy) with 1px AA edges.
func FillDiagonalCross(buf []byte, frameW, cx, cy, armLen, thickness int, c color.RGBA) {
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
