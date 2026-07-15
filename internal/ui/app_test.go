package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
)

func TestGridSize(t *testing.T) {
	cases := []struct {
		size                  image.Point
		cellWidth, lineHeight int
		wantCols, wantRows    int
	}{
		{image.Pt(800, 600), 10, 20, 80, 30},
		{image.Pt(805, 615), 10, 20, 80, 30}, // remainder pixels truncate
		{image.Pt(5, 5), 10, 20, 1, 1},       // smaller than one cell still yields 1x1
		{image.Pt(800, 600), 0, 20, 1, 1},    // unmeasured cell width doesn't divide by zero
		{image.Pt(800, 600), 10, 0, 1, 1},
	}
	for _, c := range cases {
		cols, rows := gridSize(c.size, c.cellWidth, c.lineHeight)
		if cols != c.wantCols || rows != c.wantRows {
			t.Errorf("gridSize(%v, %d, %d) = (%d, %d), want (%d, %d)",
				c.size, c.cellWidth, c.lineHeight, cols, rows, c.wantCols, c.wantRows)
		}
	}
}

func TestCellFromPosition(t *testing.T) {
	const cellWidth, lineHeight, chromeHeightPx = 10, 20, 32
	const cols, rows = 80, 24

	cases := []struct {
		name    string
		pos     f32.Point
		wantCol int
		wantRow int
	}{
		{"origin (just below the tab bar)", f32.Pt(0, 32), 0, 0},
		{"mid-grid", f32.Pt(45, 92), 4, 3},
		{"inside the tab bar clamps row to 0", f32.Pt(45, 10), 4, 0},
		{"negative X clamps col to 0", f32.Pt(-5, 92), 0, 3},
		{
			// A drag ending a few pixels past the last column must clamp
			// to the last valid column, not go out of range — an
			// unclamped out-of-range column silently dropped from
			// session.Session.SelectedText, which was a real, reported
			// bug ("cuts off characters" at the end of a selection).
			"past the right edge clamps to the last column",
			f32.Pt(float32(cols*cellWidth+50), 92),
			cols - 1, 3,
		},
		{
			"past the bottom edge clamps to the last row",
			f32.Pt(45, float32(chromeHeightPx+rows*lineHeight+100)),
			4, rows - 1,
		},
		{
			"past both edges clamps both",
			f32.Pt(float32(cols*cellWidth+999), float32(chromeHeightPx+rows*lineHeight+999)),
			cols - 1, rows - 1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			col, row := cellFromPosition(c.pos, cellWidth, lineHeight, chromeHeightPx, 0, 0, cols, rows)
			if col != c.wantCol || row != c.wantRow {
				t.Errorf("cellFromPosition(%v) = (%d, %d), want (%d, %d)", c.pos, col, row, c.wantCol, c.wantRow)
			}
		})
	}
}

func TestCellFromPositionRespectsPad(t *testing.T) {
	const cellWidth, lineHeight, chrome, pad = 10, 20, 30, 10
	col, row := cellFromPosition(f32.Pt(10, 40), cellWidth, lineHeight, chrome, pad, pad, 80, 24)
	if col != 0 || row != 0 {
		t.Fatalf("cell at pad origin = (%d, %d), want (0, 0)", col, row)
	}
	col, row = cellFromPosition(f32.Pt(25, 70), cellWidth, lineHeight, chrome, pad, pad, 80, 24)
	if col != 1 || row != 1 {
		t.Fatalf("cell with pad = (%d, %d), want (1, 1)", col, row)
	}
}

func TestCellFromPositionZeroSizeDoesNotClampAway(t *testing.T) {
	// cols/rows of 0 (not yet measured) must not make every position
	// clamp to -1 — the existing >= 0 clamps still apply, just no upper
	// bound is enforced when the size isn't known yet.
	col, row := cellFromPosition(f32.Pt(1000, 1000), 10, 20, 32, 0, 0, 0, 0)
	if col < 0 || row < 0 {
		t.Fatalf("cellFromPosition with cols=rows=0 = (%d, %d), want non-negative", col, row)
	}
}

func TestAccumulateScrollLinesExactMultiple(t *testing.T) {
	var accum float32
	if got := accumulateScrollLines(40, 20, &accum); got != 2 {
		t.Fatalf("accumulateScrollLines(40, 20) = %d, want 2", got)
	}
	if accum != 0 {
		t.Fatalf("remainder after exact multiple = %v, want 0", accum)
	}
}

func TestAccumulateScrollLinesCarriesFractionalRemainder(t *testing.T) {
	// A trackpad sends many small deltas per line — none of them should
	// be silently dropped just because a single event is smaller than
	// one line.
	var accum float32
	const lineHeight = 20

	total := 0
	for i := 0; i < 10; i++ {
		total += accumulateScrollLines(7, lineHeight, &accum)
	}
	// 10 events of 7px = 70px total = 3 whole lines (60px) with 10px
	// left over in the accumulator.
	if total != 3 {
		t.Fatalf("total lines over 10x7px events = %d, want 3", total)
	}
	if accum != 10 {
		t.Fatalf("remainder = %v, want 10", accum)
	}
}

func TestAccumulateScrollLinesNegativeDirection(t *testing.T) {
	var accum float32
	if got := accumulateScrollLines(-45, 20, &accum); got != -2 {
		t.Fatalf("accumulateScrollLines(-45, 20) = %d, want -2", got)
	}
}

func TestAccumulateScrollLinesZeroLineHeight(t *testing.T) {
	var accum float32
	if got := accumulateScrollLines(100, 0, &accum); got != 0 {
		t.Fatalf("accumulateScrollLines with lineHeight=0 = %d, want 0 (no divide-by-zero)", got)
	}
}

func TestTabStripScrollDelta(t *testing.T) {
	const tabW = 128
	cases := []struct {
		name string
		x, y float32
		want int
	}{
		{"horizontal preferred", 12, 40, 12},
		{"horizontal left", -9, 0, -9},
		{"wheel fallback amplify+", 0, 3, tabW / 2},
		{"wheel fallback amplify-", 0, -4, -(tabW / 2)},
		{"wheel large unchanged", 0, 80, 80},
		{"zero", 0, 0, 0},
	}
	for _, c := range cases {
		got := tabStripScrollDelta(c.x, c.y, tabW)
		if got != c.want {
			t.Errorf("%s: tabStripScrollDelta(%v, %v, %d) = %d, want %d",
				c.name, c.x, c.y, tabW, got, c.want)
		}
	}
}
