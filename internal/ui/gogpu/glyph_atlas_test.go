package gogpu

import (
	"testing"

	"golang.org/x/image/font/basicfont"
)

func TestGlyphAtlasValidNilAtlasIsInvalid(t *testing.T) {
	var a *glyphAtlas
	if a.valid(basicfont.Face7x13, 10) {
		t.Fatal("nil atlas must never be valid")
	}
}

func TestGlyphAtlasValidMatchesFaceAndAscent(t *testing.T) {
	a := newGlyphAtlas(basicfont.Face7x13, 10)
	if !a.valid(basicfont.Face7x13, 10) {
		t.Fatal("atlas should be valid for the exact face/ascent it was created with")
	}
}

func TestGlyphAtlasValidRejectsDifferentAscent(t *testing.T) {
	a := newGlyphAtlas(basicfont.Face7x13, 10)
	if a.valid(basicfont.Face7x13, 11) {
		t.Fatal("atlas should be invalid once the ascent changes (e.g. a font-size change)")
	}
}

func TestGlyphAtlasValidRejectsDifferentFace(t *testing.T) {
	a := newGlyphAtlas(basicfont.Face7x13, 10)
	if a.valid(nil, 10) {
		t.Fatal("atlas should be invalid for a different face value")
	}
}

func TestGlyphAtlasGetCachesEntry(t *testing.T) {
	a := newGlyphAtlas(basicfont.Face7x13, basicfont.Face7x13.Metrics().Ascent.Ceil())
	e1, ok := a.get('A')
	if !ok {
		t.Fatal("expected a rasterized entry for 'A'")
	}
	e2, ok := a.get('A')
	if !ok {
		t.Fatal("expected the second get('A') to hit the cache")
	}
	if e1.mask != e2.mask {
		t.Fatal("second get('A') should return the identical cached mask, not re-rasterize")
	}
}
