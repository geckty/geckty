package gogpu

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/geckty/geckty/internal/vt"
	"github.com/geckty/geckty/internal/vt/emu"
)

func lineText(l emu.Line) string {
	var b strings.Builder
	for _, g := range l {
		c := g.Char
		if c == 0 {
			c = ' '
		}
		b.WriteRune(c)
	}
	return strings.TrimRight(b.String(), " ")
}

// buildScrolledTerm writes n lines (each "line<i>\r\n") to a term with a
// 2-row viewport, forcing lines to scroll into history.
func buildScrolledTerm(t *testing.T, n int) *vt.Terminal {
	t.Helper()
	term := vt.New(10, 2, &bytes.Buffer{}, nil, 0)
	for i := 0; i < n; i++ {
		term.Parse([]byte("line" + string(rune('0'+i)) + "\r\n"))
	}
	return term
}

func TestViewportZeroOffsetMatchesLiveScreen(t *testing.T) {
	term := buildScrolledTerm(t, 5)
	got, top := viewport(term, 2, 0)
	want := term.Screen()
	if len(got) != len(want) {
		t.Fatalf("len(viewport) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if lineText(got[i]) != lineText(want[i]) {
			t.Fatalf("row %d: viewport(0) = %q, Screen() = %q", i, lineText(got[i]), lineText(want[i]))
		}
	}
	if want := len(term.History()); top != want {
		t.Fatalf("top = %d, want len(History())=%d", top, want)
	}
}

func TestViewportMaxScrollShowsOldestHistory(t *testing.T) {
	term := buildScrolledTerm(t, 5)
	history := term.History()
	maxOffset := len(history)
	if maxOffset == 0 {
		t.Fatal("expected some history to have accumulated")
	}

	got, top := viewport(term, 2, maxOffset)
	if len(got) != 2 {
		t.Fatalf("len(viewport) = %d, want 2", len(got))
	}
	if lineText(got[0]) != lineText(history[0]) {
		t.Fatalf("viewport(max)[0] = %q, want oldest history line %q", lineText(got[0]), lineText(history[0]))
	}
	if top != 0 {
		t.Fatalf("top = %d, want 0 (oldest line is AbsLine 0)", top)
	}
}

func TestViewportOffsetBeyondHistoryClampsAtTop(t *testing.T) {
	term := buildScrolledTerm(t, 5)
	maxOffset := len(term.History())

	got, top := viewport(term, 2, maxOffset+50)
	if len(got) != 2 {
		t.Fatalf("len(viewport) = %d, want 2", len(got))
	}
	if top != 0 {
		t.Fatalf("top = %d, want 0 (clamped)", top)
	}
}

func testSelection(startCol, startRow, endCol, endRow int) Selection {
	sel := Selection{Active: true}
	sel.Start.Col, sel.Start.Row = startCol, startRow
	sel.End.Col, sel.End.Row = endCol, endRow
	return sel
}

func TestSelectionColRangeSingleRow(t *testing.T) {
	sel := testSelection(2, 0, 5, 0)
	colStart, colEnd, ok := selectionColRange(sel, 0, 10)
	if !ok || colStart != 2 || colEnd != 6 {
		t.Fatalf("got %d,%d,%v, want 2,6,true", colStart, colEnd, ok)
	}
}

func TestSelectionColRangeMultiRowMiddleIsFullWidth(t *testing.T) {
	sel := testSelection(5, 0, 3, 2)
	colStart, colEnd, ok := selectionColRange(sel, 1, 10)
	if !ok || colStart != 0 || colEnd != 10 {
		t.Fatalf("got %d,%d,%v, want 0,10,true", colStart, colEnd, ok)
	}
}

func TestSelectionColRangeOutsideRowRange(t *testing.T) {
	sel := testSelection(0, 1, 5, 3)
	if _, _, ok := selectionColRange(sel, 0, 10); ok {
		t.Fatal("expected ok=false for a row above the selection")
	}
	if _, _, ok := selectionColRange(sel, 4, 10); ok {
		t.Fatal("expected ok=false for a row below the selection")
	}
}

func TestCellSpanUsesRequestedColsRows(t *testing.T) {
	pl := Placement{Cols: 5, Rows: 2}
	cols, rows := cellSpan(pl, image.Pt(999, 999), 10, 20)
	if cols != 5 || rows != 2 {
		t.Fatalf("cols,rows = %d,%d, want 5,2", cols, rows)
	}
}

func TestCellSpanComputesFromPixelSizeWhenNotRequested(t *testing.T) {
	pl := Placement{}
	cols, rows := cellSpan(pl, image.Pt(95, 41), 10, 20)
	if cols != 10 || rows != 3 {
		t.Fatalf("cols,rows = %d,%d, want 10,3", cols, rows)
	}
}

func TestCellSpanNeverBelowOneCell(t *testing.T) {
	pl := Placement{}
	cols, rows := cellSpan(pl, image.Pt(0, 0), 10, 20)
	if cols != 1 || rows != 1 {
		t.Fatalf("cols,rows = %d,%d, want 1,1", cols, rows)
	}
}

// ── Pixel-buffer assertion tests (following termizard's painter_test.go
// precedent — coverage for the software painter's fill helpers). ───────

func newBuf(w, h int) []byte { return make([]byte, w*h*4) }

func pixelAt(buf []byte, frameW, x, y int) color.RGBA {
	off := (y*frameW + x) * 4
	return color.RGBA{R: buf[off], G: buf[off+1], B: buf[off+2], A: buf[off+3]}
}

func TestFillRectFillsCorrectRegion(t *testing.T) {
	buf := newBuf(4, 4)
	red := color.RGBA{R: 0xff, A: 0xff}
	fillRect(buf, 4, 1, 1, 3, 3, red)

	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			inside := x >= 1 && x < 3 && y >= 1 && y < 3
			got := pixelAt(buf, 4, x, y)
			if inside && got != red {
				t.Fatalf("(%d,%d) = %v, want %v (inside fill rect)", x, y, got, red)
			}
			if !inside && got != (color.RGBA{}) {
				t.Fatalf("(%d,%d) = %v, want zero (outside fill rect)", x, y, got)
			}
		}
	}
}

func TestFillRectClampsToBufferBounds(t *testing.T) {
	buf := newBuf(2, 2)
	// Should not panic even though the rect extends far past the buffer.
	fillRect(buf, 2, -5, -5, 100, 100, color.RGBA{G: 0xff, A: 0xff})
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			if got := pixelAt(buf, 2, x, y); got.G != 0xff {
				t.Fatalf("(%d,%d).G = %d, want 0xff (clamped fill should still cover the whole buffer)", x, y, got.G)
			}
		}
	}
}

func TestBlitGlyphClippedRespectsClipBounds(t *testing.T) {
	buf := newBuf(6, 3)
	mask := image.NewAlpha(image.Rect(0, 0, 6, 3))
	for i := range mask.Pix {
		mask.Pix[i] = 255 // fully opaque glyph covering the whole mask
	}
	fg := color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}

	// Glyph draw rect spans the whole buffer, but the clip window only
	// allows columns [2,4) — mirrors how a cell's glyph bearing can extend
	// slightly beyond its own cell but must not paint into the neighbor.
	dr := image.Rect(0, 0, 6, 3)
	blitGlyphClipped(buf, 6, 3, dr, mask, image.Point{}, fg, 2, 0, 4, 3)

	for y := 0; y < 3; y++ {
		for x := 0; x < 6; x++ {
			got := pixelAt(buf, 6, x, y)
			inClip := x >= 2 && x < 4
			if inClip && got != fg {
				t.Fatalf("(%d,%d) = %v, want %v (inside clip window)", x, y, got, fg)
			}
			if !inClip && got != (color.RGBA{}) {
				t.Fatalf("(%d,%d) = %v, want zero (outside clip window — must not bleed into neighbor cell)", x, y, got)
			}
		}
	}
}

func TestBlitGlyphClippedSkipsFullyTransparentPixels(t *testing.T) {
	buf := newBuf(2, 1)
	mask := image.NewAlpha(image.Rect(0, 0, 2, 1)) // all zero alpha
	fg := color.RGBA{R: 0xff, A: 0xff}
	dr := image.Rect(0, 0, 2, 1)
	blitGlyphClipped(buf, 2, 1, dr, mask, image.Point{}, fg, 0, 0, 2, 1)

	if got := pixelAt(buf, 2, 0, 0); got != (color.RGBA{}) {
		t.Fatalf("fully transparent glyph pixel must not be written, got %v", got)
	}
}
