package gogpu

import (
	"image"
	"image/color"
)

// fillRect fills [x0,y0)-[x1,y1) in buf (RGBA8, stride frameW*4) with c,
// clamped to the buffer's bounds. This is a plain bounds-checked pixel
// write — no GPU clip primitive involved, unlike the gio renderer this
// package replaces.
func fillRect(buf []byte, frameW, x0, y0, x1, y1 int, c color.RGBA) {
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
	firstOff := y0*stride + x0*4
	for x := x0; x < x1; x++ {
		off := y0*stride + x*4
		buf[off] = c.R
		buf[off+1] = c.G
		buf[off+2] = c.B
		buf[off+3] = c.A
	}
	rowBytes := (x1 - x0) * 4
	src := buf[firstOff : firstOff+rowBytes]
	for y := y0 + 1; y < y1; y++ {
		dst := buf[y*stride+x0*4:]
		copy(dst[:rowBytes], src)
	}
}

// blendPixel writes r,g,b into buf at byte offset off (RGBA8), src-over
// blended against whatever is already there by alpha (0-255). This is the
// shared blend math behind blitGlyphClipped, blitImageScaled, and
// roundrect.go's fillRoundRect/blendRect — factored out since all four were
// doing the same per-channel (src*a + dst*(255-a))/255 arithmetic. alpha==0
// is a no-op the caller should skip rather than pay the write for.
func blendPixel(buf []byte, off int, r, g, b, alpha uint8) {
	if alpha == 255 {
		buf[off], buf[off+1], buf[off+2], buf[off+3] = r, g, b, 255
		return
	}
	inv := uint32(255 - alpha)
	buf[off] = uint8((uint32(r)*uint32(alpha) + uint32(buf[off])*inv) / 255)     //nolint:gosec // G115: 0-255 product/255 fits uint8
	buf[off+1] = uint8((uint32(g)*uint32(alpha) + uint32(buf[off+1])*inv) / 255) //nolint:gosec
	buf[off+2] = uint8((uint32(b)*uint32(alpha) + uint32(buf[off+2])*inv) / 255) //nolint:gosec
	buf[off+3] = 255
}

// blitGlyphClipped composites a glyph mask onto buf, discarding ink outside
// [cx0,cx1)x[cy0,cy1) (keeps side bearings from painting into a neighboring
// cell or past the frame edge). This replaces gio's per-cell clip.Rect
// push/pop — the same visual effect via bounds-checked pixel math instead
// of a GPU clip stack, which is what avoids the D3D11 corruption bug.
func blitGlyphClipped(buf []byte, frameW, frameH int, dr image.Rectangle, mask *image.Alpha, maskp image.Point, fg color.RGBA, cx0, cy0, cx1, cy1 int) {
	if frameW <= 0 || len(buf) == 0 {
		return
	}
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
	for py := dr.Min.Y; py < dr.Max.Y; py++ {
		if py < cy0 || py >= cy1 {
			continue
		}
		for px := dr.Min.X; px < dr.Max.X; px++ {
			if px < cx0 || px >= cx1 {
				continue
			}
			mx := px - dr.Min.X + maskp.X
			my := py - dr.Min.Y + maskp.Y
			a := mask.AlphaAt(mx, my).A
			if a == 0 {
				continue
			}
			blendPixel(buf, py*stride+px*4, fg.R, fg.G, fg.B, a)
		}
	}
}

// blitImageScaled composites src (Kitty-graphics placement image, may carry
// real alpha) into buf at [x0,y0)-[x0+w,y0+h), nearest-neighbor scaled from
// src's own bounds. New code — termizard has no Kitty-graphics prior art to
// port from.
func blitImageScaled(buf []byte, frameW, frameH int, src *image.RGBA, x0, y0, w, h int) {
	if frameW <= 0 || len(buf) == 0 || w <= 0 || h <= 0 {
		return
	}
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 {
		return
	}
	x1, y1 := x0+w, y0+h
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
	for py := cy0; py < cy1; py++ {
		sy := sb.Min.Y + (py-y0)*sh/h
		for px := cx0; px < cx1; px++ {
			sx := sb.Min.X + (px-x0)*sw/w
			sOff := src.PixOffset(sx, sy)
			sr, sg, sb2, sa := src.Pix[sOff], src.Pix[sOff+1], src.Pix[sOff+2], src.Pix[sOff+3]
			if sa == 0 {
				continue
			}
			blendPixel(buf, py*stride+px*4, sr, sg, sb2, sa)
		}
	}
}
