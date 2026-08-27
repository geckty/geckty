package raster

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const glyphAtlasMaxEntries = 4096

// GlyphEntry holds a single cached glyph rasterization.
// DrRel is the destination rect produced by face.Glyph at the canonical dot
// (0, cAscent). At draw time, add image.Pt(originX, y0) to get the actual
// frame-buffer rect.
type GlyphEntry struct {
	Mask  *image.Alpha
	MaskP image.Point
	DrRel image.Rectangle
}

// GlyphAtlas caches face.Glyph() rasterizations keyed by rune.
type GlyphAtlas struct {
	face     font.Face
	cAscent  int
	embolden bool
	entries  map[rune]GlyphEntry
}

// NewGlyphAtlas builds an atlas for face at the given cell ascent.
func NewGlyphAtlas(face font.Face, cAscent int) *GlyphAtlas {
	return &GlyphAtlas{
		face:    face,
		cAscent: cAscent,
		entries: make(map[rune]GlyphEntry, 256),
	}
}

// NewEmboldenedGlyphAtlas is like NewGlyphAtlas but fattens Regular stems by
// 1px — other terminals often look "bold" by default; Regular faces
// (Menlo / IBM Plex Mono) read thin on HiDPI without this.
func NewEmboldenedGlyphAtlas(face font.Face, cAscent int) *GlyphAtlas {
	a := NewGlyphAtlas(face, cAscent)
	a.embolden = true
	return a
}

// Valid reports whether the atlas matches the current face and ascent.
func (a *GlyphAtlas) Valid(face font.Face, cAscent int) bool {
	return a != nil && a.face == face && a.cAscent == cAscent
}

// Get returns the cached entry for r, populating it on first access.
func (a *GlyphAtlas) Get(r rune) (GlyphEntry, bool) {
	if e, ok := a.entries[r]; ok {
		return e, true
	}
	if len(a.entries) >= glyphAtlasMaxEntries {
		a.entries = make(map[rune]GlyphEntry, 256)
	}
	if a.face == nil {
		return GlyphEntry{}, false
	}
	canonDot := fixed.P(0, a.cAscent)
	dr, mask, maskp, _, ok := a.face.Glyph(canonDot, r)
	if !ok {
		return GlyphEntry{}, false
	}
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
	e := GlyphEntry{Mask: cached, MaskP: maskp, DrRel: dr}
	if a.embolden {
		fattenAlphaMask(cached, 1)
	}
	a.entries[r] = e
	return e, true
}

func fattenAlphaMask(img *image.Alpha, px int) {
	if img == nil || px < 1 {
		return
	}
	b := img.Bounds()
	src := make([]uint8, len(img.Pix))
	copy(src, img.Pix)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := img.PixOffset(x, y)
			a := src[i]
			if x-px >= b.Min.X {
				if sa := src[img.PixOffset(x-px, y)]; sa > a {
					a = sa
				}
			}
			img.Pix[i] = a
		}
	}
}
