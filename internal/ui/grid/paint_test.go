package grid

import (
	"bytes"
	"image"
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
	term := vt.New(10, 2, &bytes.Buffer{}, nil)
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
	// At maximum scroll, the viewport's first row must be the oldest
	// history line — you can't scroll back further than that.
	if lineText(got[0]) != lineText(history[0]) {
		t.Fatalf("viewport(max)[0] = %q, want oldest history line %q", lineText(got[0]), lineText(history[0]))
	}
	if top != 0 {
		t.Fatalf("top = %d, want 0 (oldest line is AbsLine 0)", top)
	}
}

func TestViewportSlidesContinuously(t *testing.T) {
	// Scrolling back by one line should shift the whole window by
	// exactly one line, not jump or skip.
	term := buildScrolledTerm(t, 6)
	history := term.History()
	if len(history) < 2 {
		t.Fatal("expected at least 2 lines of history")
	}

	atOffset1, top1 := viewport(term, 2, 1)
	atOffset2, top2 := viewport(term, 2, 2)

	// The top row at offset 2 should equal the top row at offset 1
	// shifted down by one — i.e. atOffset1's bottom-most-but-one
	// relationship: atOffset2[1] should equal atOffset1[0].
	if lineText(atOffset2[1]) != lineText(atOffset1[0]) {
		t.Fatalf("viewport did not slide continuously: offset2[1]=%q, offset1[0]=%q",
			lineText(atOffset2[1]), lineText(atOffset1[0]))
	}
	if top2 != top1-1 {
		t.Fatalf("top did not slide continuously: top1=%d top2=%d, want top2=top1-1", top1, top2)
	}
}

func TestViewportOffsetBeyondHistoryClampsAtTop(t *testing.T) {
	term := buildScrolledTerm(t, 5)
	maxOffset := len(term.History())

	// A caller passing an offset beyond history (shouldn't happen if
	// session.ScrollBy's clamping is used, but Paint takes a raw int)
	// must not panic or slice out of range.
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
	if !ok {
		t.Fatal("expected ok=true")
	}
	if colStart != 2 || colEnd != 6 {
		t.Fatalf("colStart=%d colEnd=%d, want 2,6 (inclusive end+1)", colStart, colEnd)
	}
}

func TestSelectionColRangeMultiRowMiddleIsFullWidth(t *testing.T) {
	sel := testSelection(5, 0, 3, 2)
	colStart, colEnd, ok := selectionColRange(sel, 1, 10)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if colStart != 0 || colEnd != 10 {
		t.Fatalf("middle row colStart=%d colEnd=%d, want full width 0,10", colStart, colEnd)
	}
}

func TestSelectionColRangeFirstAndLastRowClipped(t *testing.T) {
	sel := testSelection(5, 0, 3, 2)

	firstStart, firstEnd, ok := selectionColRange(sel, 0, 10)
	if !ok || firstStart != 5 || firstEnd != 10 {
		t.Fatalf("first row = %d,%d,%v, want 5,10,true", firstStart, firstEnd, ok)
	}

	lastStart, lastEnd, ok := selectionColRange(sel, 2, 10)
	if !ok || lastStart != 0 || lastEnd != 4 {
		t.Fatalf("last row = %d,%d,%v, want 0,4,true", lastStart, lastEnd, ok)
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
		t.Fatalf("cols,rows = %d,%d, want 5,2 (explicit request should win over pixel size)", cols, rows)
	}
}

func TestCellSpanComputesFromPixelSizeWhenNotRequested(t *testing.T) {
	pl := Placement{} // Cols=Rows=0: not specified
	cols, rows := cellSpan(pl, image.Pt(95, 41), 10, 20)
	// 95px / 10px-per-cell -> rounds up to 10 cells; 41px / 20px-per-line
	// -> rounds up to 3 lines.
	if cols != 10 || rows != 3 {
		t.Fatalf("cols,rows = %d,%d, want 10,3", cols, rows)
	}
}

func TestCellSpanNeverBelowOneCell(t *testing.T) {
	pl := Placement{}
	cols, rows := cellSpan(pl, image.Pt(0, 0), 10, 20)
	if cols != 1 || rows != 1 {
		t.Fatalf("cols,rows = %d,%d, want 1,1 (never zero, even for a degenerate image)", cols, rows)
	}
}

func TestSelectionColRangeZeroWidthOnSameRow(t *testing.T) {
	// A single-column selection (col N to col N) must still highlight
	// that one column, not disappear.
	sel := testSelection(3, 0, 3, 0)
	colStart, colEnd, ok := selectionColRange(sel, 0, 10)
	if !ok || colStart != 3 || colEnd != 4 {
		t.Fatalf("got %d,%d,%v, want 3,4,true", colStart, colEnd, ok)
	}
}
