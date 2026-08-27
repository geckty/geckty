package raster

import (
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
