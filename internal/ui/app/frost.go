package app

import (
	"image/color"
	"math"

	"github.com/geckty/geckty/internal/ui/raster"
)

// frostGlassRoundRect paints liquid glass over whatever is already in buf:
// mild blur + light refraction of the underlay, then a translucent tint.
func frostGlassRoundRect(buf []byte, frameW, x0, y0, x1, y1, radius, blurR int, tint color.RGBA) {
	if frameW <= 0 || len(buf) == 0 || x0 >= x1 || y0 >= y1 {
		return
	}
	frameH := len(buf) / (frameW * 4)
	sx0, sy0, sx1, sy1, ok := frostSampleBounds(frameW, frameH, x0, y0, x1, y1, blurR)
	if !ok {
		return
	}
	sw, sh := sx1-sx0, sy1-sy0
	src := make([]byte, sw*sh*4)
	tmp := make([]byte, len(src))
	stride := frameW * 4
	for y := 0; y < sh; y++ {
		copy(src[y*sw*4:(y+1)*sw*4], buf[(sy0+y)*stride+sx0*4:(sy0+y)*stride+sx1*4])
	}
	// One box pass — two passes read as muddy fog.
	boxBlurRGBA(src, tmp, sw, sh, blurR)
	compositeFrostRoundRect(buf, frameW, frameH, src, sw, sh, sx0, sy0, x0, y0, x1, y1, radius, tint)
}

func frostSampleBounds(frameW, frameH, x0, y0, x1, y1, blurR int) (sx0, sy0, sx1, sy1 int, ok bool) {
	if blurR < 1 {
		blurR = 1
	}
	sx0, sy0 = x0-blurR-2, y0-blurR-2
	sx1, sy1 = x1+blurR+2, y1+blurR+2
	if sx0 < 0 {
		sx0 = 0
	}
	if sy0 < 0 {
		sy0 = 0
	}
	if sx1 > frameW {
		sx1 = frameW
	}
	if sy1 > frameH {
		sy1 = frameH
	}
	if sx0 >= sx1 || sy0 >= sy1 {
		return 0, 0, 0, 0, false
	}
	return sx0, sy0, sx1, sy1, true
}

func compositeFrostRoundRect(buf []byte, frameW, frameH int, blurred []byte, sw, sh, sx0, sy0, x0, y0, x1, y1, radius int, tint color.RGBA) {
	srcA := float64(tint.A) / 255
	cx := float64(x0+x1) / 2
	cy := float64(y0+y1) / 2
	hw := float64(x1-x0) / 2
	hh := float64(y1-y0) / 2
	if hw < 1 {
		hw = 1
	}
	if hh < 1 {
		hh = 1
	}
	stride := frameW * 4
	for y := y0; y < y1; y++ {
		if y < 0 || y >= frameH {
			continue
		}
		for x := x0; x < x1; x++ {
			if x < 0 || x >= frameW {
				continue
			}
			cov := raster.RoundRectCoverage(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1, radius)
			if cov <= 0 {
				continue
			}
			nx := (float64(x) + 0.5 - cx) / hw
			ny := (float64(y) + 0.5 - cy) / hh
			warp := 1.8 * (1 - (nx*nx+ny*ny)*0.35)
			if warp < 0.4 {
				warp = 0.4
			}
			sx := x + int(math.Round(nx*warp))
			sy := y + int(math.Round(ny*warp*0.65))
			lx, ly := sx-sx0, sy-sy0
			if lx < 0 {
				lx = 0
			}
			if ly < 0 {
				ly = 0
			}
			if lx >= sw {
				lx = sw - 1
			}
			if ly >= sh {
				ly = sh - 1
			}
			off := (ly*sw + lx) * 4
			br, bgc, bb := float64(blurred[off]), float64(blurred[off+1]), float64(blurred[off+2])
			tr := uint8(br*(1-srcA) + float64(tint.R)*srcA + 0.5)
			tg := uint8(bgc*(1-srcA) + float64(tint.G)*srcA + 0.5)
			tb := uint8(bb*(1-srcA) + float64(tint.B)*srcA + 0.5)
			alpha := uint8(cov*255 + 0.5)
			if alpha == 0 {
				continue
			}
			dst := y*stride + x*4
			if alpha == 255 {
				buf[dst], buf[dst+1], buf[dst+2], buf[dst+3] = tr, tg, tb, 255
				continue
			}
			inv := uint32(255 - alpha)
			buf[dst] = uint8((uint32(tr)*uint32(alpha) + uint32(buf[dst])*inv) / 255)
			buf[dst+1] = uint8((uint32(tg)*uint32(alpha) + uint32(buf[dst+1])*inv) / 255)
			buf[dst+2] = uint8((uint32(tb)*uint32(alpha) + uint32(buf[dst+2])*inv) / 255)
			buf[dst+3] = 255
		}
	}
}

// boxBlurRGBA runs a separable box blur of radius r on an opaque RGBA
// buffer. Result is written back into src; tmp is scratch of the same size.
func boxBlurRGBA(src, tmp []byte, w, h, r int) {
	if r < 1 || w < 1 || h < 1 {
		return
	}
	div := 2*r + 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb int
			for k := -r; k <= r; k++ {
				xx := x + k
				if xx < 0 {
					xx = 0
				} else if xx >= w {
					xx = w - 1
				}
				o := (y*w + xx) * 4
				sr += int(src[o])
				sg += int(src[o+1])
				sb += int(src[o+2])
			}
			o := (y*w + x) * 4
			tmp[o] = uint8(sr / div)
			tmp[o+1] = uint8(sg / div)
			tmp[o+2] = uint8(sb / div)
			tmp[o+3] = 255
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb int
			for k := -r; k <= r; k++ {
				yy := y + k
				if yy < 0 {
					yy = 0
				} else if yy >= h {
					yy = h - 1
				}
				o := (yy*w + x) * 4
				sr += int(tmp[o])
				sg += int(tmp[o+1])
				sb += int(tmp[o+2])
			}
			o := (y*w + x) * 4
			src[o] = uint8(sr / div)
			src[o+1] = uint8(sg / div)
			src[o+2] = uint8(sb / div)
			src[o+3] = 255
		}
	}
}
