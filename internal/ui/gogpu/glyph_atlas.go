package gogpu

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const glyphAtlasMaxEntries = 4096

// glyphEntry holds a single cached glyph rasterization.
// drRel is the destination rect produced by face.Glyph at the canonical dot
// (0, cAscent). At draw time, add image.Pt(originX, y0) to get the actual
// frame-buffer rect.
type glyphEntry struct {
	mask  *image.Alpha
	maskp image.Point
	drRel image.Rectangle
}

// glyphAtlas caches face.Glyph() rasterizations keyed by rune. Owned by the
// window's render state; recreated when the face or ascent changes.
type glyphAtlas struct {
	face    font.Face
	cAscent int
	entries map[rune]glyphEntry
}

func newGlyphAtlas(face font.Face, cAscent int) *glyphAtlas {
	return &glyphAtlas{
		face:    face,
		cAscent: cAscent,
		entries: make(map[rune]glyphEntry, 256),
	}
}

// valid reports whether the atlas matches the current face and ascent.
func (a *glyphAtlas) valid(face font.Face, cAscent int) bool {
	return a != nil && a.face == face && a.cAscent == cAscent
}

// get returns the cached entry for r, populating it on first access.
func (a *glyphAtlas) get(r rune) (glyphEntry, bool) {
	if e, ok := a.entries[r]; ok {
		return e, true
	}
	if len(a.entries) >= glyphAtlasMaxEntries {
		// Clear on overflow — overflow is rare in practice (<4k distinct runes).
		a.entries = make(map[rune]glyphEntry, 256)
	}
	if a.face == nil {
		return glyphEntry{}, false
	}
	// Rasterize at canonical (0, cAscent) so drRel is relative to (originX=0, y0=0).
	// Caller adds (originX, y0) at draw time: actualDr = drRel.Add(image.Pt(originX, y0)).
	canonDot := fixed.P(0, a.cAscent)
	dr, mask, maskp, _, ok := a.face.Glyph(canonDot, r)
	if !ok {
		return glyphEntry{}, false
	}
	// Copy mask: many face implementations reuse the same backing buffer.
	bounds := mask.Bounds()
	cached := image.NewAlpha(bounds)
	if alphaImg, isA := mask.(*image.Alpha); isA {
		copy(cached.Pix, alphaImg.Pix)
	} else {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, ca := mask.At(x, y).RGBA()
				cached.SetAlpha(x, y, color.Alpha{A: uint8(ca >> 8)}) //nolint:gosec // G115
			}
		}
	}
	e := glyphEntry{mask: cached, maskp: maskp, drRel: dr}
	a.entries[r] = e
	return e, true
}
