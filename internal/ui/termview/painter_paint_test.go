package termview

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
	return testTheme().Palette
}

func testTheme() theme.Theme {
	pal := theme.Palette{
		Foreground:    color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Background:    color.NRGBA{R: 0, G: 0, B: 0, A: 0xff},
		Selection:     color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff},
		SelectionFG:   color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Cursor:        color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		TabBarBG:      color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xff},
		ActiveTabFG:   color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		ActiveTabBG:   color.NRGBA{R: 0x57, G: 0x57, B: 0x57, A: 0xff},
		InactiveTabFG: color.NRGBA{R: 0xad, G: 0xad, B: 0xad, A: 0xff},
		InactiveTabBG: color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xff},
		HoverTabBG:    color.NRGBA{R: 0x24, G: 0x24, B: 0x24, A: 0xff},
		PlusButtonBG:  color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff},
	}
	for i := range pal.ANSI {
		pal.ANSI[i] = color.NRGBA{R: uint8(i * 10), A: 0xff}
	}
	return theme.Theme{
		Palette: pal,
		Glass:   theme.DefaultGlass(),
		UI: theme.UITokens{
			VisualBell:           color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55},
			ScrollbarTrack:       color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x28},
			ScrollbarThumb:       color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x70},
			URLUnderline:         pal.ANSI[12],
			SearchMatch:          color.NRGBA{R: pal.ANSI[11].R, G: pal.ANSI[11].G, B: pal.ANSI[11].B, A: 0x90},
			HintLabelBG:          pal.ANSI[11],
			HintLabelFG:          pal.Background,
			PaneFocusBorder:      color.NRGBA{R: pal.ANSI[12].R, G: pal.ANSI[12].G, B: pal.ANSI[12].B, A: 0xaa},
			CommandRunning:       pal.ANSI[6],
			CommandSuccess:       pal.ANSI[2],
			CommandFailed:        pal.ANSI[1],
			CommandBorderEnabled: true, // tests exercise the border painter
		},
	}
}

func testPainter() *Painter {
	face := basicfont.Face7x13
	return &Painter{
		Palette:    testPalette(),
		Fonts:      FontBundle{Regular: face},
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
	ok := p.Paint(buf, 100, 60, 0, 0, term, 0, Selection{}, nil, true, nil)
	if !ok {
		t.Fatal("Paint should return true when it has valid cell metrics")
	}

	// Background should be filled with the palette background within the
	// grid but away from the "Hi" text (grid is 10 cols x 3 rows = 70x39px;
	// "Hi" only occupies the first two cells of row 0).
	if got := pixelAt(buf, 100, 65, 35); got != ToRGBA(p.Palette.Background) {
		t.Fatalf("in-grid empty pixel = %v, want background %v", got, ToRGBA(p.Palette.Background))
	}

	// Somewhere in the first cell's glyph region should be non-background
	// (the letter 'H' painted in foreground).
	found := false
	for y := 0; y < 13; y++ {
		for x := 0; x < 7; x++ {
			if pixelAt(buf, 100, x, y) == ToRGBA(p.Palette.Foreground) {
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
	if p.Paint(buf, 50, 50, 0, 0, term, 0, Selection{}, nil, true, nil) {
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
	p.Paint(buf, 100, 60, 0, 0, term, 0, sel, nil, false, nil)

	// A pixel within the selected column range (col 3, safely inside the
	// selection, away from any glyph ink) should show the selection color
	// blended in, not the plain background.
	x := 3*p.CellWidth + 1
	y := 1
	got := pixelAt(buf, 100, x, y)
	bg := ToRGBA(p.Palette.Background)
	if got == bg {
		t.Fatalf("pixel inside selection = %v, want selection highlight (not plain background %v)", got, bg)
	}
}

func TestPainterPaintCursorStyles(t *testing.T) {
	p := testPainter()
	fg := ToRGBA(p.Palette.Foreground)

	blockTerm := vt.New(10, 3, io.Discard, nil, 0)
	buf := newBuf(100, 60)
	p.Paint(buf, 100, 60, 0, 0, blockTerm, 0, Selection{}, nil, true, nil)
	if got := pixelAt(buf, 100, 3, 6); got != fg {
		t.Fatalf("block cursor center pixel = %v, want FG %v", got, fg)
	}

	underlineTerm := vt.New(10, 3, io.Discard, nil, 0)
	_, _ = underlineTerm.Write([]byte("\x1b[4 q")) // DECSCUSR: steady underline
	buf2 := newBuf(100, 60)
	p.Paint(buf2, 100, 60, 0, 0, underlineTerm, 0, Selection{}, nil, true, nil)
	if got := pixelAt(buf2, 100, 3, 0); got == fg {
		t.Fatal("underline cursor should not paint the top of the cell")
	}
	if got := pixelAt(buf2, 100, 3, 12); got != fg {
		t.Fatalf("underline cursor should paint the bottom row, got %v", got)
	}

	barTerm := vt.New(10, 3, io.Discard, nil, 0)
	_, _ = barTerm.Write([]byte("\x1b[6 q")) // DECSCUSR: steady bar
	buf3 := newBuf(100, 60)
	p.Paint(buf3, 100, 60, 0, 0, barTerm, 0, Selection{}, nil, true, nil)
	if got := pixelAt(buf3, 100, 0, 6); got != fg {
		t.Fatalf("bar cursor should paint the left edge of the cell, got %v", got)
	}
	if got := pixelAt(buf3, 100, 5, 6); got == fg {
		t.Fatal("bar cursor should not paint the right side of the cell")
	}
}

func TestPainterPaintScrollOffsetSkipsCursorButPaintsSelection(t *testing.T) {
	p := testPainter()
	term := vt.New(10, 2, io.Discard, nil, 0)
	_, _ = term.Write([]byte("abcdefghij"))
	buf := newBuf(100, 60)
	sel := Selection{Active: true}
	sel.Start.Col, sel.Start.Row = 2, 0
	sel.End.Col, sel.End.Row = 4, 0
	// scrollOffset != 0 must skip the cursor but still paint selection.
	if !p.Paint(buf, 100, 60, 0, 0, term, 1, sel, nil, true, nil) {
		t.Fatal("Paint should still return true with a nonzero scrollOffset")
	}
	x := 3*p.CellWidth + 1
	y := 1
	got := pixelAt(buf, 100, x, y)
	bg := ToRGBA(p.Palette.Background)
	if got == bg {
		t.Fatalf("pixel inside selection with scrollOffset!=0 = %v, want selection highlight", got)
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
	p.Paint(buf, 100, 60, 0, 0, term, 0, Selection{}, placements, false, nil)
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

func TestStyleOfDimInvisibleStrikethrough(t *testing.T) {
	g := emu.Glyph{
		Mode:      emu.AttrDim | emu.AttrInvisible | emu.AttrStrikethrough,
		Underline: emu.UnderlineStyle{Mode: emu.UnderlineSingle},
	}
	st := styleOf(g)
	if !st.dim || !st.invisible || !st.strikethrough || !st.underline {
		t.Fatalf("styleOf = %+v, want dim/invisible/strikethrough/underline", st)
	}
}

func TestDimRGBAAveragesTowardBackground(t *testing.T) {
	fg := color.RGBA{R: 200, G: 100, B: 0, A: 255}
	bg := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	got := dimRGBA(fg, bg)
	if got.R != 100 || got.G != 50 || got.B != 0 || got.A != 255 {
		t.Fatalf("dimRGBA = %v, want average toward bg", got)
	}
}

func TestPaintUnderlineModes(t *testing.T) {
	fg := color.RGBA{R: 0xff, A: 0xff}
	for _, mode := range []emu.UnderlineMode{
		emu.UnderlineSingle,
		emu.UnderlineDouble,
		emu.UnderlineDotted,
		emu.UnderlineDashed,
		emu.UnderlineCurly,
	} {
		buf := newBuf(20, 10)
		paintUnderline(buf, 20, 0, 16, 10, mode, fg)
		found := false
		for y := 0; y < 10 && !found; y++ {
			for x := 0; x < 16; x++ {
				if pixelAt(buf, 20, x, y) == fg {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("paintUnderline(%v) left no foreground pixels", mode)
		}
	}
}

func TestPainterPaintDimInvisibleStrikethrough(t *testing.T) {
	p := testPainter()
	term := vt.New(10, 3, io.Discard, nil, 0)
	// Dim + strikethrough on 'A', then invisible on 'B'.
	_, _ = term.Write([]byte("\x1b[2;9mA\x1b[0;8mB"))

	buf := newBuf(100, 60)
	if !p.Paint(buf, 100, 60, 0, 0, term, 0, Selection{}, nil, false, nil) {
		t.Fatal("Paint should succeed")
	}

	// Strikethrough paints a mid-cell horizontal line in cell 0.
	midY := p.CellHeight / 2
	foundStrike := false
	for x := 0; x < p.CellWidth; x++ {
		if pixelAt(buf, 100, x, midY) != ToRGBA(p.Palette.Background) {
			foundStrike = true
			break
		}
	}
	if !foundStrike {
		t.Fatal("expected strikethrough ink in first cell")
	}

	// Invisible 'B' in cell 1 should leave that cell without glyph FG ink
	// (background only), aside from any residual bleed — check far from
	// cell 0.
	x := p.CellWidth + p.CellWidth/2
	y := 2
	if got := pixelAt(buf, 100, x, y); got != ToRGBA(p.Palette.Background) {
		// Invisible may still leave bg fill only; non-bg means glyph leaked.
		t.Fatalf("invisible cell pixel = %v, want background", got)
	}
}
