package emu

import (
	"bytes"
	"log"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu/geom"
)

func TestCursorVisible(t *testing.T) {
	term := New()
	if !term.CursorVisible() {
		t.Fatal("cursor should be visible by default")
	}
	_, _ = term.Write([]byte("\x1b[?25l")) // DECTCEM off
	if term.CursorVisible() {
		t.Fatal("cursor should be hidden after DECTCEM reset")
	}
	_, _ = term.Write([]byte("\x1b[?25h")) // DECTCEM on
	if !term.CursorVisible() {
		t.Fatal("cursor should be visible after DECTCEM set")
	}
}

func TestClearAltScrollbackOnAltExit(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 4}), WithHistoryLimit(100))
	tr := term.(*terminal)
	_, _ = term.Write([]byte(LineFeedMode))
	for range 5 {
		_, _ = term.Write([]byte("main\r\n"))
	}
	mainHist := len(tr.history)

	_, _ = term.Write([]byte(EnterAltScreen))
	_, _ = term.Write([]byte("altscreen-long-line-that-reflows\r\n"))
	term.Resize(geom.Vec2{C: 4, R: 4})

	if len(tr.history) == 0 && len(tr.altHistory) == 0 {
		t.Fatal("expected some history on alt screen before exit")
	}

	_, _ = term.Write([]byte(ExitAltScreen))

	if len(tr.altHistory) != 0 {
		t.Fatalf("altHistory = %d lines after alt exit, want 0", len(tr.altHistory))
	}
	if len(tr.history) < mainHist {
		t.Fatalf("main history shrunk from %d to %d after alt exit", mainHist, len(tr.history))
	}
	if term.IsAltMode() {
		t.Fatal("should be back on main screen")
	}
}
func TestIsAltModeToggle(t *testing.T) {
	term := New()
	if term.IsAltMode() {
		t.Fatal("should start on the main screen")
	}
	_, _ = term.Write([]byte(EnterAltScreen))
	if !term.IsAltMode() {
		t.Fatal("should be on the alt screen after EnterAltScreen")
	}
}

func TestMoveAbsToViaCUP(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 20, R: 10}))
	_, _ = term.Write([]byte("\x1b[5;10H")) // CUP row 5, col 10 (1-based)
	cur := term.Cursor()
	if cur.R != 4 || cur.C != 9 {
		t.Fatalf("Cursor = %+v, want R=4,C=9 (0-based)", cur)
	}
}

func TestScrollDownViaSD(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 3}))
	_, _ = term.Write([]byte(LineFeedMode))
	_, _ = term.Write([]byte("aaa\r\nbbb\r\nccc"))
	_, _ = term.Write([]byte("\x1b[2T")) // SD - scroll down 2 lines
	// After scrolling down, the top lines should now be blank.
	line0 := extractStr(term, 0, 2, 0)
	if line0 != "   " {
		t.Fatalf("row 0 after scroll-down = %q, want blank", line0)
	}
}

func TestInsertAndDeleteChars(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 2}))
	_, _ = term.Write([]byte("abcdef"))
	_, _ = term.Write([]byte("\x1b[3G")) // move to col 3 (1-based) => index 2
	_, _ = term.Write([]byte("\x1b[2@")) // ICH - insert 2 blanks at cursor
	got := extractStr(term, 0, 7, 0)
	if got != "ab  cdef" {
		t.Fatalf("after ICH: got %q, want \"ab  cdef\"", got)
	}

	term2 := New(WithSize(geom.Vec2{C: 10, R: 2}))
	_, _ = term2.Write([]byte("abcdef"))
	_, _ = term2.Write([]byte("\x1b[3G"))
	_, _ = term2.Write([]byte("\x1b[2P")) // DCH - delete 2 chars at cursor
	got2 := extractStr(term2, 0, 5, 0)
	if got2 != "abef  " {
		t.Fatalf("after DCH: got %q, want \"abef  \"", got2)
	}
}

func TestInsertAndDeleteLines(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 5, R: 4}))
	_, _ = term.Write([]byte(LineFeedMode))
	_, _ = term.Write([]byte("111\r\n222\r\n333\r\n444"))
	_, _ = term.Write([]byte("\x1b[2;1H")) // move to row 2
	_, _ = term.Write([]byte("\x1b[L"))    // IL - insert 1 blank line
	if got := extractStr(term, 0, 2, 1); got != "   " {
		t.Fatalf("row 1 after IL = %q, want blank", got)
	}
	if got := extractStr(term, 0, 2, 2); got != "222" {
		t.Fatalf("row 2 after IL = %q, want \"222\" (pushed down)", got)
	}

	term2 := New(WithSize(geom.Vec2{C: 5, R: 4}))
	_, _ = term2.Write([]byte(LineFeedMode))
	_, _ = term2.Write([]byte("111\r\n222\r\n333\r\n444"))
	_, _ = term2.Write([]byte("\x1b[2;1H")) // move to row 2
	_, _ = term2.Write([]byte("\x1b[M"))    // DL - delete 1 line
	if got := extractStr(term2, 0, 2, 1); got != "333" {
		t.Fatalf("row 1 after DL = %q, want \"333\" (pulled up)", got)
	}
}

func TestPutTabBoundaries(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 20, R: 2}))
	// Forward tab from column 0 lands on the next tab stop.
	_, _ = term.Write([]byte("\t"))
	if cur := term.Cursor(); cur.C == 0 {
		t.Fatal("forward tab should advance the cursor")
	}
	// Backward tab from column 0 is a no-op (can't go further left).
	_, _ = term.Write([]byte("\x1b[20G")) // move near right edge
	_, _ = term.Write([]byte("\x1b[Z"))   // CBT - back tab
	if cur := term.Cursor(); cur.C == 19 {
		t.Fatal("backward tab should move the cursor left")
	}
	// Forward tab pinned at the last column is a no-op.
	term2 := New(WithSize(geom.Vec2{C: 4, R: 2}))
	_, _ = term2.Write([]byte("\x1b[4G")) // move to last column
	_, _ = term2.Write([]byte("\t"))
	if cur := term2.Cursor(); cur.C != 3 {
		t.Fatalf("forward tab at the last column should not move, got C=%d", cur.C)
	}
}

func TestSetModePrivateBroadSweep(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 5}))
	seqs := []string{
		"\x1b[?1h", "\x1b[?1l", // DECCKM
		"\x1b[?5h", "\x1b[?5l", // DECSCNM reverse video
		"\x1b[?6h", "\x1b[?6l", // DECOM origin
		"\x1b[?7h", "\x1b[?7l", // DECAWM
		"\x1b[?0h",             // ignored error mode
		"\x1b[?9h", "\x1b[?9l", // X10 mouse
		"\x1b[?1000h", "\x1b[?1000l", // button press reporting
		"\x1b[?1002h", "\x1b[?1002l", // motion reporting
		"\x1b[?1003h", "\x1b[?1003l", // all motions
		"\x1b[?1004h", "\x1b[?1004l", // focus events
		"\x1b[?1006h", "\x1b[?1006l", // SGR mouse
		"\x1b[?1034h", "\x1b[?1034l", // 8-bit
		"\x1b[?1049h", "\x1b[?1049l", // alt screen + save/restore cursor
		"\x1b[?47h", "\x1b[?47l", // alt screen (legacy)
		"\x1b[?1047h", "\x1b[?1047l", // alt screen (legacy)
		"\x1b[?1048h", "\x1b[?1048l", // save/restore cursor only
		"\x1b[?1001h",                // mouse highlight (no-op)
		"\x1b[?1005h",                // utf8 mouse (no-op)
		"\x1b[?1015h",                // urxvt mouse (no-op)
		"\x1b[?2026h", "\x1b[?2026l", // sync update
		"\x1b[?9999h", // unknown private mode -> logf fallback
	}
	for _, s := range seqs {
		if _, err := term.Write([]byte(s)); err != nil {
			t.Fatalf("Write(%q) error: %v", s, err)
		}
	}
	// Bracketed paste already covered by TestBracketedPasteMode in vt_test.go.
}

func TestSetModeNonPrivateBroadSweep(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 5}))
	seqs := []string{
		"\x1b[0h",            // ignored error
		"\x1b[2h", "\x1b[2l", // KAM
		"\x1b[4h", "\x1b[4l", // IRM (logs "not implemented")
		"\x1b[12h", "\x1b[12l", // SRM
		"\x1b[20h", "\x1b[20l", // LNM
		"\x1b[34h",   // right-to-left (logs "not implemented")
		"\x1b[96h",   // right-to-left copy (logs "not implemented")
		"\x1b[9999h", // unknown -> logf fallback
	}
	for _, s := range seqs {
		if _, err := term.Write([]byte(s)); err != nil {
			t.Fatalf("Write(%q) error: %v", s, err)
		}
	}
}

func TestSetModeLogsWithDebugLogger(t *testing.T) {
	var buf bytes.Buffer
	term := New(WithSize(geom.Vec2{C: 10, R: 5}))
	term.(*terminal).DebugLogger = log.New(&buf, "", 0)

	_, _ = term.Write([]byte("\x1b[?9999h")) // unknown private mode -> logf
	_, _ = term.Write([]byte("\x1b[4h"))     // IRM -> logln

	if buf.Len() == 0 {
		t.Fatal("expected DebugLogger to receive log output from setMode's logf/logln paths")
	}
}

func TestSetAttrBroadSweep(t *testing.T) {
	cellAfter := func(seq string) Glyph {
		term := New(WithSize(geom.Vec2{C: 10, R: 2}))
		_, _ = term.Write([]byte(seq + "x"))
		return term.Cell(0, 0)
	}

	if g := cellAfter("\x1b[1m"); g.Mode&attrBold == 0 {
		t.Error("SGR 1 should set bold")
	}
	if g := cellAfter("\x1b[3m"); g.Mode&attrItalic == 0 {
		t.Error("SGR 3 should set italic")
	}
	if g := cellAfter("\x1b[4m"); g.Underline.Mode != UnderlineSingle {
		t.Errorf("SGR 4 should set single underline, got %v", g.Underline.Mode)
	}
	if g := cellAfter("\x1b[5m"); g.Mode&attrBlink == 0 {
		t.Error("SGR 5 should set blink")
	}
	if g := cellAfter("\x1b[7m"); g.Mode&attrReverse == 0 {
		t.Error("SGR 7 should set reverse")
	}
	if g := cellAfter("\x1b[2m"); g.Mode&attrDim == 0 {
		t.Error("SGR 2 should set dim")
	}
	if g := cellAfter("\x1b[8m"); g.Mode&attrInvisible == 0 {
		t.Error("SGR 8 should set invisible")
	}
	if g := cellAfter("\x1b[53m"); g.Mode&attrOverline == 0 {
		t.Error("SGR 53 should set overline")
	}
	if g := cellAfter("\x1b[2m\x1b[22m"); g.Mode&attrDim != 0 {
		t.Error("SGR 22 should clear dim")
	}
	if g := cellAfter("\x1b[8m\x1b[28m"); g.Mode&attrInvisible != 0 {
		t.Error("SGR 28 should clear invisible")
	}
	if g := cellAfter("\x1b[53m\x1b[55m"); g.Mode&attrOverline != 0 {
		t.Error("SGR 55 should clear overline")
	}
	if g := cellAfter("\x1b[9m"); g.Mode&attrStrikethrough == 0 {
		t.Error("SGR 9 should set strikethrough")
	}
	if g := cellAfter("\x1b[21m"); g.Underline.Mode != UnderlineDouble {
		t.Error("SGR 21 should set double underline")
	}
	if g := cellAfter("\x1b[1m\x1b[22m"); g.Mode&attrBold != 0 {
		t.Error("SGR 22 should clear bold")
	}
	if g := cellAfter("\x1b[3m\x1b[23m"); g.Mode&attrItalic != 0 {
		t.Error("SGR 23 should clear italic")
	}
	if g := cellAfter("\x1b[4m\x1b[24m"); g.Underline.Mode != UnderlineNone {
		t.Error("SGR 24 should clear underline")
	}
	if g := cellAfter("\x1b[5m\x1b[25m"); g.Mode&attrBlink != 0 {
		t.Error("SGR 25 should clear blink")
	}
	if g := cellAfter("\x1b[7m\x1b[27m"); g.Mode&attrReverse != 0 {
		t.Error("SGR 27 should clear reverse")
	}
	if g := cellAfter("\x1b[9m\x1b[29m"); g.Mode&attrStrikethrough != 0 {
		t.Error("SGR 29 should clear strikethrough")
	}
	if g := cellAfter("\x1b[38;5;200m"); g.FG != XTermColor(200) {
		t.Errorf("SGR 38;5;200 should set xterm FG 200, got %v", g.FG)
	}
	if g := cellAfter("\x1b[38;2;10;20;30m"); g.FG != RGBColor(10, 20, 30) {
		t.Errorf("SGR 38;2;10;20;30 should set RGB FG, got %v", g.FG)
	}
	if g := cellAfter("\x1b[38;5;200m\x1b[39m"); g.FG != DefaultFG {
		t.Error("SGR 39 should reset FG to default")
	}
	if g := cellAfter("\x1b[48;5;200m"); g.BG != XTermColor(200) {
		t.Errorf("SGR 48;5;200 should set xterm BG 200, got %v", g.BG)
	}
	if g := cellAfter("\x1b[48;2;10;20;30m"); g.BG != RGBColor(10, 20, 30) {
		t.Errorf("SGR 48;2;10;20;30 should set RGB BG, got %v", g.BG)
	}
	if g := cellAfter("\x1b[48;5;200m\x1b[49m"); g.BG != DefaultBG {
		t.Error("SGR 49 should reset BG to default")
	}
	if g := cellAfter("\x1b[58;5;200m"); g.Underline.Color != XTermColor(200) {
		t.Errorf("SGR 58;5;200 should set xterm underline color, got %v", g.Underline.Color)
	}
	if g := cellAfter("\x1b[58;2;10;20;30m"); g.Underline.Color != RGBColor(10, 20, 30) {
		t.Errorf("SGR 58;2;10;20;30 should set RGB underline color, got %v", g.Underline.Color)
	}
	if g := cellAfter("\x1b[58;5;200m\x1b[59m"); g.Underline.Color != DefaultFG {
		t.Error("SGR 59 should reset underline color to default")
	}
	if g := cellAfter("\x1b[31m"); g.FG != ANSIColor(1) {
		t.Errorf("SGR 31 should set ANSI FG red, got %v", g.FG)
	}
	if g := cellAfter("\x1b[44m"); g.BG != ANSIColor(4) {
		t.Errorf("SGR 44 should set ANSI BG blue, got %v", g.BG)
	}
	if g := cellAfter("\x1b[91m"); g.FG != ANSIColor(9) {
		t.Errorf("SGR 91 should set bright ANSI FG, got %v", g.FG)
	}
	if g := cellAfter("\x1b[101m"); g.BG != ANSIColor(9) {
		t.Errorf("SGR 101 should set bright ANSI BG, got %v", g.BG)
	}
	// Malformed/short 38, 48, 58 sequences and a fully unknown attr all fall
	// through to the "gfx attr unknown" logf branch rather than erroring.
	for _, seq := range []string{"\x1b[38m", "\x1b[48m", "\x1b[58m", "\x1b[200m"} {
		_ = cellAfter(seq) // must not panic
	}
	// Full reset (SGR 0) clears everything set above.
	if g := cellAfter("\x1b[1;7;38;5;200m\x1b[0m"); g.Mode&(attrBold|attrReverse) != 0 || g.FG != DefaultFG {
		t.Errorf("SGR 0 should reset all attributes, got %+v", g)
	}
}
