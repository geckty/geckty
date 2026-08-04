package session

import (
	"testing"
	"time"
)

func TestSelectionNoneByDefault(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	if _, _, ok := s.Selection(); ok {
		t.Fatal("expected no selection by default")
	}
	if _, ok := s.SelectedText(); ok {
		t.Fatal("expected SelectedText ok=false with no selection")
	}
}

func TestSelectionStartGivesZeroWidthSelection(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.StartSelection(3, 1)
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected a selection after StartSelection")
	}
	if start != (CellPos{3, 1}) || end != (CellPos{3, 1}) {
		t.Fatalf("start=%+v end=%+v, want both (3,1)", start, end)
	}
}

func TestSelectionExtendForward(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.StartSelection(1, 0)
	s.ExtendSelection(5, 0)
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected a selection")
	}
	if start != (CellPos{1, 0}) || end != (CellPos{5, 0}) {
		t.Fatalf("start=%+v end=%+v, want (1,0)-(5,0)", start, end)
	}
}

func TestSelectionExtendBackwardNormalizes(t *testing.T) {
	// Dragging up/left of the anchor must still yield start <= end in
	// reading order, not a "negative width" selection.
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.StartSelection(5, 1)
	s.ExtendSelection(1, 0)
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected a selection")
	}
	if start != (CellPos{1, 0}) || end != (CellPos{5, 1}) {
		t.Fatalf("start=%+v end=%+v, want (1,0)-(5,1) after backward drag", start, end)
	}
}

func TestExtendSelectionWithoutStartIsNoOp(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.ExtendSelection(5, 5)
	if _, _, ok := s.Selection(); ok {
		t.Fatal("expected ExtendSelection without a prior StartSelection to be a no-op")
	}
}

func TestClearSelection(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.StartSelection(0, 0)
	s.ClearSelection()
	if _, _, ok := s.Selection(); ok {
		t.Fatal("expected no selection after ClearSelection")
	}
}

func TestStartSelectionReplacesPrevious(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.StartSelection(0, 0)
	s.ExtendSelection(5, 0)
	s.StartSelection(2, 1) // a new click elsewhere replaces the old selection
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected a selection")
	}
	if start != (CellPos{2, 1}) || end != (CellPos{2, 1}) {
		t.Fatalf("start=%+v end=%+v, want fresh zero-width selection at (2,1)", start, end)
	}
}

func TestEndSelectionDropsAPlainClick(t *testing.T) {
	// A press+release with no drag in between (ExtendSelection never
	// called) is not a meaningful selection — leaving one behind was a
	// real, reported bug (every plain click left a lingering 1-character
	// highlight).
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.StartSelection(3, 0)
	s.EndSelection()
	if _, _, ok := s.Selection(); ok {
		t.Fatal("expected no selection after a plain click (press+release, no drag)")
	}
}

func TestEndSelectionKeepsARealDrag(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.StartSelection(1, 0)
	s.ExtendSelection(5, 0)
	s.EndSelection()
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected the selection to survive EndSelection after a real drag")
	}
	if start != (CellPos{1, 0}) || end != (CellPos{5, 0}) {
		t.Fatalf("start=%+v end=%+v, want (1,0)-(5,0)", start, end)
	}
}

func TestEndSelectionKeepsADragThatReturnsToTheAnchorCell(t *testing.T) {
	// A drag that moves and comes back to the exact starting cell is
	// still a real drag gesture (ExtendSelection was called), not a
	// plain click — anchor == head here must not be confused with "never
	// dragged".
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.StartSelection(3, 0)
	s.ExtendSelection(7, 0)
	s.ExtendSelection(3, 0) // back to the anchor cell
	s.EndSelection()
	if _, _, ok := s.Selection(); !ok {
		t.Fatal("expected the selection to survive EndSelection after a real (if round-trip) drag")
	}
}

func TestSelectWordBasic(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "hello world")

	s.SelectWord(2, 0, "") // click inside "hello"
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected a selection after SelectWord")
	}
	if start != (CellPos{0, 0}) || end != (CellPos{4, 0}) {
		t.Fatalf("start=%+v end=%+v, want (0,0)-(4,0) (the word \"hello\")", start, end)
	}
	text, ok := s.SelectedText()
	if !ok || text != "hello" {
		t.Fatalf("SelectedText = %q, %v, want \"hello\", true", text, ok)
	}
}

func TestSelectWordSecondWord(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "hello world")

	s.SelectWord(8, 0, "") // click inside "world" (cols 6-10)
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected a selection after SelectWord")
	}
	if start != (CellPos{6, 0}) || end != (CellPos{10, 0}) {
		t.Fatalf("start=%+v end=%+v, want (6,0)-(10,0) (the word \"world\")", start, end)
	}
}

func TestSelectWordOnWhitespaceSelectsJustThatCell(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "hello world")

	s.SelectWord(5, 0, "") // the space between the two words
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected a selection after SelectWord, even on whitespace")
	}
	if start != (CellPos{5, 0}) || end != (CellPos{5, 0}) {
		t.Fatalf("start=%+v end=%+v, want a single-cell selection at (5,0)", start, end)
	}
}

func TestSelectWordCustomWordChars(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "my-file.txt end")

	// Without "-." as word characters, only "file" (between the hyphen
	// and the dot) is selected.
	s.SelectWord(4, 0, "")
	text, _ := s.SelectedText()
	if text != "file" {
		t.Fatalf("SelectedText (default word chars) = %q, want %q", text, "file")
	}

	// With "-." included, the whole "my-file.txt" is one word.
	s.SelectWord(4, 0, "-.")
	text, ok := s.SelectedText()
	if !ok || text != "my-file.txt" {
		t.Fatalf("SelectedText (with -. as word chars) = %q, %v, want \"my-file.txt\", true", text, ok)
	}
}

func TestEndSelectionKeepsASingleCharacterWordSelection(t *testing.T) {
	// A double-click on a single-character word (or on whitespace, per
	// SelectWord's own doc comment) produces anchor == head — the same
	// shape as a never-dragged plain click. EndSelection must still keep
	// it: complete is set directly by SelectWord, not inferred from
	// anchor/head equality.
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "a b")

	s.SelectWord(0, 0, "") // the lone "a"
	s.EndSelection()       // simulates the button-release after the double-click
	if _, _, ok := s.Selection(); !ok {
		t.Fatal("expected a single-character word selection to survive EndSelection")
	}
}

func TestRegisterClickDetectsDoubleClick(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	if s.RegisterClick(3, 1) {
		t.Fatal("first click must never register as a double-click")
	}
	if !s.RegisterClick(3, 1) {
		t.Fatal("expected a second click in the same cell, soon after, to register as a double-click")
	}
}

func TestRegisterClickDifferentCellIsNotADouble(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.RegisterClick(3, 1)
	if s.RegisterClick(4, 1) {
		t.Fatal("a click in a different cell must not register as a double-click")
	}
}

func TestRegisterClickTooSlowIsNotADouble(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.selMu.Lock()
	s.lastClick = clickRecord{at: time.Now().Add(-doubleClickWindow * 2), pos: cellPos{3, 1}}
	s.selMu.Unlock()

	if s.RegisterClick(3, 1) {
		t.Fatal("a click after doubleClickWindow has elapsed must not register as a double-click")
	}
}

func TestRegisterClickResetsAfterADouble(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.RegisterClick(3, 1)
	if !s.RegisterClick(3, 1) {
		t.Fatal("expected the second click to register as a double-click")
	}
	// A third rapid click in the same cell starts a fresh count, not
	// another double (matches "double-click" semantics, not "every
	// click after the first is a double").
	if s.RegisterClick(3, 1) {
		t.Fatal("expected the third click to NOT register as a double-click (count resets after a detected double)")
	}
}

func TestSelectedTextSingleLine(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 3, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "hello world")

	s.StartSelection(0, 0)
	s.ExtendSelection(4, 0) // "hello" = cols 0-4
	text, ok := s.SelectedText()
	if !ok {
		t.Fatal("expected SelectedText ok=true")
	}
	if text != "hello" {
		t.Fatalf("SelectedText = %q, want %q", text, "hello")
	}
}

func TestSelectedTextMultiLine(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 3, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "abc\r\ndef")

	s.StartSelection(0, 0)
	s.ExtendSelection(2, 1) // full first line, "def" through col 2 on the second
	text, ok := s.SelectedText()
	if !ok {
		t.Fatal("expected SelectedText ok=true")
	}
	if text != "abc\ndef" {
		t.Fatalf("SelectedText = %q, want %q", text, "abc\ndef")
	}
}

func TestSelectedTextTrimsTrailingBlanks(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "hi")

	s.StartSelection(0, 0)
	s.ExtendSelection(19, 0) // select the whole row, including blank cells
	text, ok := s.SelectedText()
	if !ok {
		t.Fatal("expected SelectedText ok=true")
	}
	if text != "hi" {
		t.Fatalf("SelectedText = %q, want %q (trailing blanks trimmed)", text, "hi")
	}
}
