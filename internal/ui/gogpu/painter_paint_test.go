package gogpu

import (
	"image"
	"image/color"
	"io"
	"testing"

	"golang.org/x/image/font/basicfont"

	"github.com/geckty/geckty/internal/ui/theme"
	"github.com/geckty/geckty/internal/vt"
	"github.com/geckty/geckty/internal/vt/emu"
)

func testPalette() theme.Palette {
	pal := theme.Palette{
		Foreground: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Background: color.NRGBA{R: 0, G: 0, B: 0, A: 0xff},
		Selection:  color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff},
	}
	for i := range pal.ANSI {
		pal.ANSI[i] = color.NRGBA{R: uint8(i * 10), A: 0xff}
	}
	return pal
}

func testPainter() *Painter {
	face := basicfont.Face7x13
	return &Painter{
		Palette:    testPalette(),
		Fonts:      fontBundle{regular: face},
		CellWidth:  7,
		CellHeight: 13,
		Ascent:     face.Metrics().Ascent.Ceil(),
	}
}

func TestPainterPaintRendersText(t *testing.T) {
	p := testPainter()
	term := vt.New(10, 3, io.Discard, nil, 0)
	_, _ = term.Write([]byte("Hi"))

	buf := newBuf(100, 60)
	ok := p.Paint(buf, 100, 60, 0, 0, term, 0, Selection{}, nil, true)
	if !ok {
		t.Fatal("Paint should return true when it has valid cell metrics")
	}

	// Background should be filled with the palette background within the
	// grid but away from the "Hi" text (grid is 10 cols x 3 rows = 70x39px;
	// "Hi" only occupies the first two cells of row 0).
	if got := pixelAt(buf, 100, 65, 35); got != toRGBA(p.Palette.Background) {
		t.Fatalf("in-grid empty pixel = %v, want background %v", got, toRGBA(p.Palette.Background))
	}

	// Somewhere in the first cell's glyph region should be non-background
	// (the letter 'H' painted in foreground).
	found := false
	for y := 0; y < 13; y++ {
		for x := 0; x < 7; x++ {
			if pixelAt(buf, 100, x, y) == toRGBA(p.Palette.Foreground) {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected at least one foreground-colored pixel where 'H' was painted")
	}
}

func TestPainterPaintZeroCellMetricsIsNoop(t *testing.T) {
	p := &Painter{Palette: testPalette()}
	term := vt.New(10, 3, io.Discard, nil, 0)
	buf := newBuf(50, 50)
	if p.Paint(buf, 50, 50, 0, 0, term, 0, Selection{}, nil, true) {
		t.Fatal("Paint with CellWidth/CellHeight<=0 should return false")
	}
}

func TestPainterPaintSelectionHighlight(t *testing.T) {
	p := testPainter()
	term := vt.New(10, 3, io.Discard, nil, 0)
	_, _ = term.Write([]byte("abcdefghij"))

	buf := newBuf(100, 60)
	sel := Selection{Active: true}
	sel.Start.Col, sel.Start.Row = 2, 0
	sel.End.Col, sel.End.Row = 4, 0
	p.Paint(buf, 100, 60, 0, 0, term, 0, sel, nil, false)

	// A pixel within the selected column range (col 3, safely inside the
	// selection, away from any glyph ink) should show the selection color
	// blended in, not the plain background.
	x := 3*p.CellWidth + 1
	y := 1
	got := pixelAt(buf, 100, x, y)
	bg := toRGBA(p.Palette.Background)
	if got == bg {
		t.Fatalf("pixel inside selection = %v, want selection highlight (not plain background %v)", got, bg)
	}
}

func TestPainterPaintCursorStyles(t *testing.T) {
	p := testPainter()
	fg := toRGBA(p.Palette.Foreground)

	blockTerm := vt.New(10, 3, io.Discard, nil, 0)
	buf := newBuf(100, 60)
	p.Paint(buf, 100, 60, 0, 0, blockTerm, 0, Selection{}, nil, true)
	if got := pixelAt(buf, 100, 3, 6); got != fg {
		t.Fatalf("block cursor center pixel = %v, want FG %v", got, fg)
	}

	underlineTerm := vt.New(10, 3, io.Discard, nil, 0)
	_, _ = underlineTerm.Write([]byte("\x1b[4 q")) // DECSCUSR: steady underline
	buf2 := newBuf(100, 60)
	p.Paint(buf2, 100, 60, 0, 0, underlineTerm, 0, Selection{}, nil, true)
	if got := pixelAt(buf2, 100, 3, 0); got == fg {
		t.Fatal("underline cursor should not paint the top of the cell")
	}
	if got := pixelAt(buf2, 100, 3, 12); got != fg {
		t.Fatalf("underline cursor should paint the bottom row, got %v", got)
	}

	barTerm := vt.New(10, 3, io.Discard, nil, 0)
	_, _ = barTerm.Write([]byte("\x1b[6 q")) // DECSCUSR: steady bar
	buf3 := newBuf(100, 60)
	p.Paint(buf3, 100, 60, 0, 0, barTerm, 0, Selection{}, nil, true)
	if got := pixelAt(buf3, 100, 0, 6); got != fg {
		t.Fatalf("bar cursor should paint the left edge of the cell, got %v", got)
	}
	if got := pixelAt(buf3, 100, 5, 6); got == fg {
		t.Fatal("bar cursor should not paint the right side of the cell")
	}
}

func TestPainterPaintScrollOffsetSkipsCursorAndSelection(t *testing.T) {
	p := testPainter()
	term := vt.New(10, 2, io.Discard, nil, 0)
	buf := newBuf(100, 60)
	// scrollOffset != 0 must skip cursor/selection painting without error.
	if !p.Paint(buf, 100, 60, 0, 0, term, 1, Selection{Active: true}, nil, true) {
		t.Fatal("Paint should still return true with a nonzero scrollOffset")
	}
}

func TestPainterPaintPlacementsSkipsOutOfViewport(t *testing.T) {
	p := testPainter()
	term := vt.New(10, 3, io.Discard, nil, 0)
	buf := newBuf(100, 60)

	img := image.NewRGBA(image.Rect(0, 0, 14, 13))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	placements := []Placement{
		{AbsLine: 999, Image: img, Col: 0, Cols: 2, Rows: 1}, // far out of viewport
		{AbsLine: 0, Image: nil, Col: 0},                     // nil image, must be skipped
		{AbsLine: 0, Image: img, Col: 0, Cols: 2, Rows: 1},   // valid, in viewport
	}
	// Must not panic on the nil-image / out-of-range entries, and should
	// paint the valid one.
	p.Paint(buf, 100, 60, 0, 0, term, 0, Selection{}, placements, false)
	if got := pixelAt(buf, 100, 1, 1); got.R != 0xff {
		t.Fatalf("placement pixel = %v, want opaque white from the placed image", got)
	}
}

func TestStyleOfReverseSwapsColors(t *testing.T) {
	fg := emu.ANSIColor(1)
	bg := emu.ANSIColor(2)

	plain := emu.Glyph{FG: fg, BG: bg}
	st := styleOf(plain)
	if st.fg != fg || st.bg != bg {
		t.Fatalf("non-reverse glyph: fg=%v bg=%v, want unchanged", st.fg, st.bg)
	}

	reversed := emu.Glyph{FG: fg, BG: bg, Mode: emu.AttrReverse}
	st = styleOf(reversed)
	if st.fg != bg || st.bg != fg {
		t.Fatalf("reverse glyph: fg=%v bg=%v, want swapped (fg=%v,bg=%v)", st.fg, st.bg, bg, fg)
	}
}

func TestStyleOfAttrs(t *testing.T) {
	g := emu.Glyph{Mode: emu.AttrBold | emu.AttrItalic}
	st := styleOf(g)
	if !st.bold || !st.italic {
		t.Fatalf("styleOf(bold|italic) = %+v, want both true", st)
	}
	if st.underline {
		t.Fatal("plain glyph should not report underline")
	}
}
