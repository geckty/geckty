package gogpu

import "testing"

func TestCellFromPositionSubtractsChromeAndPadding(t *testing.T) {
	// col 2 needs x = padX + 2*cellWidth; row 1 needs
	// y = chromeHeightPx + padY + 1*cellHeight.
	col, row := cellFromPosition(3+20, 40+5+20, 10, 20, 40, 3, 5, 80, 24)
	if col != 2 || row != 1 {
		t.Fatalf("cellFromPosition = %d,%d, want 2,1", col, row)
	}
}

func TestCellFromPositionClampsNegativeToZero(t *testing.T) {
	col, row := cellFromPosition(0, 0, 10, 20, 40, 5, 5, 80, 24)
	if col != 0 || row != 0 {
		t.Fatalf("cellFromPosition(above/left of grid) = %d,%d, want 0,0", col, row)
	}
}

func TestCellFromPositionClampsToLastCell(t *testing.T) {
	col, row := cellFromPosition(100000, 100000, 10, 20, 40, 5, 5, 80, 24)
	if col != 79 || row != 23 {
		t.Fatalf("cellFromPosition(far past grid) = %d,%d, want 79,23 (cols-1,rows-1)", col, row)
	}
}

func TestCellFromPositionZeroColsRowsSkipsUpperClamp(t *testing.T) {
	// cols/rows<=0 means "unknown grid size yet" — cellFromPosition should
	// still return a non-negative cell rather than clamping against a
	// nonsensical -1 upper bound.
	col, row := cellFromPosition(1000, 1000, 10, 20, 40, 5, 5, 0, 0)
	if col < 0 || row < 0 {
		t.Fatalf("cellFromPosition with cols=rows=0 = %d,%d, want non-negative", col, row)
	}
}

func TestTabStripScrollDeltaPrefersHorizontal(t *testing.T) {
	if got := tabStripScrollDelta(7, 99, 100); got != 7 {
		t.Fatalf("tabStripScrollDelta = %d, want 7 (horizontal wins)", got)
	}
}

func TestTabStripScrollDeltaFallsBackToVertical(t *testing.T) {
	if got := tabStripScrollDelta(0, 60, 100); got != 60 {
		t.Fatalf("tabStripScrollDelta = %d, want 60 (deltaY unchanged, >= half tab width)", got)
	}
}

func TestTabStripScrollDeltaAmplifiesSmallWheelNotches(t *testing.T) {
	if got := tabStripScrollDelta(0, 3, 100); got != 50 {
		t.Fatalf("tabStripScrollDelta(small positive) = %d, want 50 (half of tabW)", got)
	}
	if got := tabStripScrollDelta(0, -3, 100); got != -50 {
		t.Fatalf("tabStripScrollDelta(small negative) = %d, want -50", got)
	}
}

func TestTabStripScrollDeltaZeroTabWidthReturnsRawDelta(t *testing.T) {
	if got := tabStripScrollDelta(0, 3, 0); got != 3 {
		t.Fatalf("tabStripScrollDelta(tabW=0) = %d, want 3 (raw deltaY, no amplification)", got)
	}
}
