package app

import (
	"image"
	"image/color"
	"testing"
)

func TestGridSizeDividesPixelsByCellMetrics(t *testing.T) {
	cols, rows := gridSize(image.Pt(801, 481), 10, 20)
	if cols != 80 || rows != 24 {
		t.Fatalf("gridSize = %d,%d, want 80,24", cols, rows)
	}
}

func TestGridSizeNeverBelowOneByOne(t *testing.T) {
	if cols, rows := gridSize(image.Pt(0, 0), 10, 20); cols != 1 || rows != 1 {
		t.Fatalf("gridSize(0,0) = %d,%d, want 1,1", cols, rows)
	}
	if cols, rows := gridSize(image.Pt(5, 5), 10, 20); cols != 1 || rows != 1 {
		t.Fatalf("gridSize(smaller than one cell) = %d,%d, want 1,1", cols, rows)
	}
}

func TestGridSizeZeroCellMetricsClampsToOneByOne(t *testing.T) {
	if cols, rows := gridSize(image.Pt(800, 480), 0, 20); cols != 1 || rows != 1 {
		t.Fatalf("gridSize with cellWidth=0 = %d,%d, want 1,1", cols, rows)
	}
	if cols, rows := gridSize(image.Pt(800, 480), 10, 0); cols != 1 || rows != 1 {
		t.Fatalf("gridSize with cellHeight=0 = %d,%d, want 1,1", cols, rows)
	}
}

func TestAbsInt(t *testing.T) {
	cases := map[int]int{5: 5, -5: 5, 0: 0, -1: 1}
	for in, want := range cases {
		if got := absInt(in); got != want {
			t.Errorf("absInt(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestAccumulateScrollLinesCarriesFractionalRemainder(t *testing.T) {
	var accum float64
	lineHeight := 20

	// Three small deltas that individually round to 0 lines should
	// accumulate into a whole line once their sum crosses lineHeight.
	if lines := accumulateScrollLines(8, lineHeight, &accum); lines != 0 {
		t.Fatalf("first call = %d lines, want 0", lines)
	}
	if lines := accumulateScrollLines(8, lineHeight, &accum); lines != 0 {
		t.Fatalf("second call = %d lines, want 0", lines)
	}
	lines := accumulateScrollLines(8, lineHeight, &accum)
	if lines != 1 {
		t.Fatalf("third call = %d lines, want 1 (8+8+8=24 >= 20)", lines)
	}
	if accum != 4 {
		t.Fatalf("accum after carry = %v, want 4 (24-20 remainder)", accum)
	}
}

func TestAccumulateScrollLinesNegativeDelta(t *testing.T) {
	var accum float64
	lines := accumulateScrollLines(-45, 20, &accum)
	if lines != -2 {
		t.Fatalf("lines = %d, want -2 (-45/20 truncated toward zero)", lines)
	}
	if accum != -5 {
		t.Fatalf("accum = %v, want -5 (-45 - (-2*20))", accum)
	}
}

func TestAccumulateScrollLinesZeroLineHeightIsNoop(t *testing.T) {
	var accum float64
	if lines := accumulateScrollLines(100, 0, &accum); lines != 0 {
		t.Fatalf("lines = %d, want 0 when lineHeight<=0", lines)
	}
	if accum != 0 {
		t.Fatalf("accum = %v, want unchanged at 0", accum)
	}
}

func TestColor32(t *testing.T) {
	got := color32(0x11, 0x22, 0x33, 0x44)
	want := color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x44}
	if got != want {
		t.Fatalf("color32 = %v, want %v", got, want)
	}
}
