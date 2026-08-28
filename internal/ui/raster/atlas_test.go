package raster

import (
	"image"
	"testing"

	"golang.org/x/image/font/basicfont"
)

func TestGlyphAtlasValidNilAtlasIsInvalid(t *testing.T) {
	var a *GlyphAtlas
	if a.Valid(basicfont.Face7x13, 10) {
		t.Fatal("nil atlas must never be valid")
	}
}

func TestGlyphAtlasValidMatchesFaceAndAscent(t *testing.T) {
	a := NewGlyphAtlas(basicfont.Face7x13, 10)
	if !a.Valid(basicfont.Face7x13, 10) {
		t.Fatal("atlas should be valid for the exact face/ascent it was created with")
	}
}

func TestGlyphAtlasValidRejectsDifferentAscent(t *testing.T) {
	a := NewGlyphAtlas(basicfont.Face7x13, 10)
	if a.Valid(basicfont.Face7x13, 11) {
		t.Fatal("atlas should be invalid once the ascent changes (e.g. a font-size change)")
	}
}

func TestGlyphAtlasValidRejectsDifferentFace(t *testing.T) {
	a := NewGlyphAtlas(basicfont.Face7x13, 10)
	if a.Valid(nil, 10) {
		t.Fatal("atlas should be invalid for a different face value")
	}
}

func TestGlyphAtlasGetCachesEntry(t *testing.T) {
	a := NewGlyphAtlas(basicfont.Face7x13, basicfont.Face7x13.Metrics().Ascent.Ceil())
	e1, ok := a.Get('A')
	if !ok {
		t.Fatal("expected a rasterized entry for 'A'")
	}
	e2, ok := a.Get('A')
	if !ok {
		t.Fatal("expected the second Get('A') to hit the cache")
	}
	if e1.Mask != e2.Mask {
		t.Fatal("second Get('A') should return the identical cached mask, not re-rasterize")
	}
}

func TestGlyphAtlasLRUEvictsOldest(t *testing.T) {
	a := NewGlyphAtlas(basicfont.Face7x13, basicfont.Face7x13.Metrics().Ascent.Ceil())
	for i := 1; i <= glyphAtlasMaxEntries; i++ {
		r := rune(i)
		a.entries[r] = GlyphEntry{Mask: image.NewAlpha(image.Rect(0, 0, 1, 1))}
		a.lru = append(a.lru, r)
	}
	// Inserting a new rune evicts the oldest (rune(1)); glyph need not rasterize.
	_, _ = a.Get(rune(glyphAtlasMaxEntries + 1))
	if _, ok := a.entries[1]; ok {
		t.Fatal("oldest entry rune(1) should have been evicted")
	}
}

func TestEmboldenedGlyphAtlasFattensMask(t *testing.T) {
	ascent := basicfont.Face7x13.Metrics().Ascent.Ceil()
	plain := NewGlyphAtlas(basicfont.Face7x13, ascent)
	bold := NewEmboldenedGlyphAtlas(basicfont.Face7x13, ascent)
	ePlain, ok := plain.Get('M')
	if !ok {
		t.Fatal("plain atlas Get('M')")
	}
	eBold, ok := bold.Get('M')
	if !ok {
		t.Fatal("emboldened atlas Get('M')")
	}
	if ePlain.Mask == eBold.Mask {
		t.Fatal("emboldened atlas should produce a different mask than plain")
	}
}
