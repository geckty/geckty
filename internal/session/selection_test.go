package session

import (
	"strings"
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
		t.Fatalf("start=%+v end=%+v, want both Col=3 AbsLine=1", start, end)
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
	if got := s.RegisterClick(3, 1); got != 1 {
		t.Fatalf("first click count = %d, want 1", got)
	}
	if got := s.RegisterClick(3, 1); got != 2 {
		t.Fatalf("second click count = %d, want 2", got)
	}
}

func TestRegisterClickDifferentCellIsNotADouble(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.RegisterClick(3, 1)
	if got := s.RegisterClick(4, 1); got != 1 {
		t.Fatalf("click in a different cell count = %d, want 1", got)
	}
}

func TestRegisterClickTooSlowIsNotADouble(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	s.selMu.Lock()
	s.lastClick = clickRecord{at: time.Now().Add(-doubleClickWindow * 2), pos: cellPos{3, 1}, count: 1}
	s.selMu.Unlock()

	if got := s.RegisterClick(3, 1); got != 1 {
		t.Fatalf("click after doubleClickWindow count = %d, want 1", got)
	}
}

func TestRegisterClickTripleThenResets(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	if got := s.RegisterClick(3, 1); got != 1 {
		t.Fatalf("click 1 = %d, want 1", got)
	}
	if got := s.RegisterClick(3, 1); got != 2 {
		t.Fatalf("click 2 = %d, want 2", got)
	}
	if got := s.RegisterClick(3, 1); got != 3 {
		t.Fatalf("click 3 = %d, want 3", got)
	}
	if got := s.RegisterClick(3, 1); got != 1 {
		t.Fatalf("click 4 should restart at 1, got %d", got)
	}
}

func TestSelectLine(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "hello")
	s.SelectLine(0)
	start, end, ok := s.Selection()
	if !ok {
		t.Fatal("expected selection after SelectLine")
	}
	if start.Col != 0 || end.Col != 9 || start.AbsLine != 0 || end.AbsLine != 0 {
		t.Fatalf("SelectLine bounds = %+v-%+v, want cols 0-9 on AbsLine 0", start, end)
	}
	text, ok := s.SelectedText()
	if !ok || text != "hello" {
		t.Fatalf("SelectedText = %q, %v, want \"hello\"", text, ok)
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

func TestSelectedTextFromScrollbackHistory(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 32)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	for i := 0; i < 5; i++ {
		writeAndWaitDirty(t, p, dirty, "line"+string(rune('0'+i))+"\r\n")
	}
	if len(s.Term.History()) == 0 {
		t.Fatal("expected history after overflowing the screen")
	}

	// AbsLine 0 is the oldest retained history line ("line0").
	s.StartSelection(0, 0)
	s.ExtendSelection(4, 0)
	s.EndSelection()
	text, ok := s.SelectedText()
	if !ok {
		t.Fatal("expected SelectedText ok=true for a history selection")
	}
	if text != "line0" {
		t.Fatalf("SelectedText = %q, want %q", text, "line0")
	}
}

func TestViewToAbsLineRespectsScrollOffset(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 32)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	for i := 0; i < 5; i++ {
		writeAndWaitDirty(t, p, dirty, "line"+string(rune('0'+i))+"\r\n")
	}
	histLen := len(s.Term.History())
	if histLen == 0 {
		t.Fatal("expected history")
	}
	if got := s.ViewToAbsLine(0); got != histLen {
		t.Fatalf("live ViewToAbsLine(0) = %d, want histLen %d", got, histLen)
	}
	s.ScrollBy(histLen)
	if got := s.ViewToAbsLine(0); got != 0 {
		t.Fatalf("fully scrolled ViewToAbsLine(0) = %d, want 0", got)
	}
}

func TestSyncSelectionHistoryOffsetShiftsOnPrune(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 64)
	s := newWithPTY(p, 10, 2, Config{
		OnDirty:      func() { dirty <- struct{}{} },
		HistoryLimit: 3,
		Clipboard:    ClipboardPolicy{WriteAllow: true, MaxSize: defaultMaxOSC52},
	})
	go s.Run()
	defer func() { _ = s.Close() }()

	// Grow history to the limit without pruning past the selection yet.
	for i := 0; i < 5; i++ {
		writeAndWaitDirty(t, p, dirty, "line"+string(rune('0'+i))+"\r\n")
	}
	// Select whatever is currently AbsLine 0 (oldest retained).
	s.StartSelection(0, 0)
	s.ExtendSelection(4, 0)
	s.EndSelection()
	before, ok := s.SelectedText()
	if !ok || before == "" {
		t.Fatalf("SelectedText before prune = %q, %v", before, ok)
	}

	// More output forces prune; syncSelectionHistoryOffset runs in Run.
	for i := 0; i < 8; i++ {
		writeAndWaitDirty(t, p, dirty, "more"+string(rune('a'+i))+"\r\n")
	}

	start, end, still := s.Selection()
	if !still {
		// Fully pruned selection is allowed to disappear.
		return
	}
	if start.AbsLine < 0 || end.AbsLine < 0 {
		t.Fatalf("selection AbsLines went negative: start=%+v end=%+v", start, end)
	}
	text, ok := s.SelectedText()
	if !ok {
		t.Fatal("expected SelectedText ok while selection still present")
	}
	if text == "" {
		t.Fatal("expected non-empty SelectedText after prune adjust")
	}
}

func TestViewportTopHelpers(t *testing.T) {
	if got := viewportTop(5, 2, 2, 0); got != 5 {
		t.Fatalf("live top = %d, want 5", got)
	}
	if got := viewportTop(5, 2, 2, 5); got != 0 {
		t.Fatalf("max scroll top = %d, want 0", got)
	}
	if got := viewToAbsLine(5, 2, 2, 3, 1); got != 3 {
		// top = 5+2-2-3 = 2; + viewRow 1 = 3
		t.Fatalf("viewToAbsLine = %d, want 3", got)
	}
}

func TestSelectionRectAndDragging(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 4, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "abcdef\r\nghijkl\r\n")

	s.StartSelectionMode(1, 0, true)
	if !s.SelectionDragging() {
		t.Fatal("expected dragging after StartSelectionMode")
	}
	if !s.SelectionRect() {
		t.Fatal("expected rectangular selection")
	}
	s.ExtendSelection(3, 1)
	start, end, ok := s.Selection()
	if !ok || start.Col != 1 || end.Col != 3 || start.AbsLine != 0 || end.AbsLine != 1 {
		t.Fatalf("rect bounds = %+v %+v ok=%v", start, end, ok)
	}
	text, ok := s.SelectedText()
	if !ok {
		t.Fatal("SelectedText should succeed for rect")
	}
	// Columns 1..3 of each line: bcd / hij
	if !strings.Contains(text, "bcd") || !strings.Contains(text, "hij") {
		t.Fatalf("SelectedText = %q, want columns bcd/hij", text)
	}
	s.EndSelection()
	if s.SelectionDragging() {
		t.Fatal("dragging should clear after EndSelection")
	}
	if !s.SelectionRect() {
		t.Fatal("rect mode should remain after EndSelection")
	}
}

