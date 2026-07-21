package emu

import (
	"testing"

	"github.com/geckty/geckty/internal/vt/emu/geom"
)

func TestGlyphTransparent(t *testing.T) {
	g := Glyph{Mode: attrTransparent}
	if !g.Transparent() {
		t.Fatal("glyph with attrTransparent set should be Transparent")
	}
	if (Glyph{}).Transparent() {
		t.Fatal("plain glyph should not be Transparent")
	}
}

func TestGlyphEqual(t *testing.T) {
	a := Glyph{Char: 'x', FG: Red, BG: Blue}
	b := Glyph{Char: 'x', FG: Red, BG: Blue}
	c := Glyph{Char: 'y', FG: Red, BG: Blue}
	if !a.Equal(b) {
		t.Fatal("identical glyphs should be Equal")
	}
	if a.Equal(c) {
		t.Fatal("glyphs with different chars should not be Equal")
	}
}

func TestGlyphSameAttrs(t *testing.T) {
	a := Glyph{Char: 'x', FG: Red, BG: Blue}
	b := Glyph{Char: 'y', FG: Red, BG: Blue}
	c := Glyph{Char: 'x', FG: Green, BG: Blue}
	if !a.SameAttrs(b) {
		t.Fatal("glyphs differing only by Char should have SameAttrs")
	}
	if a.SameAttrs(c) {
		t.Fatal("glyphs with different FG should not have SameAttrs")
	}
}

func TestLineOccupancy(t *testing.T) {
	line := LineFromString("a b")
	occ := line.Occupancy()
	want := []bool{true, false, true}
	if len(occ) != len(want) {
		t.Fatalf("Occupancy len = %d, want %d", len(occ), len(want))
	}
	for i, w := range want {
		if occ[i] != w {
			t.Errorf("Occupancy[%d] = %v, want %v", i, occ[i], w)
		}
	}
}

func TestLineIsEmpty(t *testing.T) {
	empty := make(Line, 5)
	for i := range empty {
		empty[i] = EmptyGlyph()
	}
	if !empty.IsEmpty() {
		t.Fatal("a line of only empty glyphs should be IsEmpty")
	}

	nonEmpty := LineFromString("hi")
	if nonEmpty.IsEmpty() {
		t.Fatal("a line with visible chars should not be IsEmpty")
	}
}

func TestLineWhitespace(t *testing.T) {
	line := make(Line, 5)
	for i := range line {
		line[i] = EmptyGlyph()
	}
	line[1].Char = 'a'
	line[3].Char = 'b'

	first, last := line.Whitespace()
	if first != 1 || last != 3 {
		t.Fatalf("Whitespace() = %d,%d, want 1,3", first, last)
	}
}

func TestWithSize(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 40, R: 10}))
	size := term.Size()
	if size.C != 40 || size.R != 10 {
		t.Fatalf("Size() = %v, want {C:40 R:10}", size)
	}
}

func TestWithHistoryLimit(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 2}), WithHistoryLimit(3))
	_, _ = term.Write([]byte(LineFeedMode))
	for i := 0; i < 20; i++ {
		_, _ = term.Write([]byte("line\r\n"))
	}
	st := term.(*terminal)
	if got := len(st.history); got > 3 {
		t.Fatalf("history length = %d, want at most 3 (WithHistoryLimit)", got)
	}
}
