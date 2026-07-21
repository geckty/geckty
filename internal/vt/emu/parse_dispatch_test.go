package emu

import (
	"log"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu/geom"
)

func TestCsiDispatchMaxargMovement(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 20, R: 10}))
	_, _ = term.Write([]byte("\x1b[5;5H")) // CUP to 4,4 (0-based)
	_, _ = term.Write([]byte("\x1b[2A"))   // CUU up 2
	if cur := term.Cursor(); cur.R != 2 {
		t.Fatalf("after CUU 2: R=%d, want 2", cur.R)
	}
	_, _ = term.Write([]byte("\x1b[3B")) // CUD down 3
	if cur := term.Cursor(); cur.R != 5 {
		t.Fatalf("after CUD 3: R=%d, want 5", cur.R)
	}
	_, _ = term.Write([]byte("\x1b[2C")) // CUF forward 2
	if cur := term.Cursor(); cur.C != 6 {
		t.Fatalf("after CUF 2: C=%d, want 6", cur.C)
	}
	_, _ = term.Write([]byte("\x1b[3D")) // CUB backward 3
	if cur := term.Cursor(); cur.C != 3 {
		t.Fatalf("after CUB 3: C=%d, want 3", cur.C)
	}
}

func TestCsiDispatchCursorNextPrevLine(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 20, R: 10}))
	_, _ = term.Write([]byte("\x1b[5;5H"))
	_, _ = term.Write([]byte("\x1b[2E")) // CNL down 2, col 0 (from R=4 -> R=6)
	if cur := term.Cursor(); cur.R != 6 || cur.C != 0 {
		t.Fatalf("after CNL 2: %+v, want R=6,C=0", cur)
	}
	_, _ = term.Write([]byte("\x1b[5;5H"))
	_, _ = term.Write([]byte("\x1b[2F")) // CPL up 2, col 0
	if cur := term.Cursor(); cur.R != 2 || cur.C != 0 {
		t.Fatalf("after CPL 2: %+v, want R=2,C=0", cur)
	}
}

func TestCsiDispatchTabClear(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 20, R: 5}))
	st := term.(*terminal)
	_, _ = term.Write([]byte("\x1b[3G"))
	_, _ = term.Write([]byte("\x1b[0g")) // TBC clear current tab stop
	if st.tabs[st.cur.C] {
		t.Fatal("CSI 0g should clear the tab stop at the cursor")
	}
	_, _ = term.Write([]byte("\x1b[3g")) // TBC clear all tab stops
	for i, set := range st.tabs {
		if set {
			t.Fatalf("CSI 3g should clear all tab stops, tabs[%d] still set", i)
		}
	}
}

func TestCsiDispatchEraseDisplayAndLine(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 5, R: 3}))
	_, _ = term.Write([]byte(LineFeedMode))
	_, _ = term.Write([]byte("aaaaa\r\nbbbbb\r\nccccc"))
	_, _ = term.Write([]byte("\x1b[2;3H")) // row 1 (0-based), col 2
	_, _ = term.Write([]byte("\x1b[0J"))   // ED below
	if got := extractStr(term, 0, 4, 2); got != "     " {
		t.Fatalf("row 2 after ED-below = %q, want blank", got)
	}

	term2 := New(WithSize(geom.Vec2{C: 5, R: 4}))
	_, _ = term2.Write([]byte(LineFeedMode))
	_, _ = term2.Write([]byte("aaaaa\r\nbbbbb\r\nccccc\r\nddddd"))
	// ED-above's "clear everything above the cursor's row" branch only
	// runs when cur.R > 1 (0-based) — row 3 (1-based) puts the cursor at
	// R=2, satisfying that.
	_, _ = term2.Write([]byte("\x1b[3;3H"))
	_, _ = term2.Write([]byte("\x1b[1J")) // ED above
	if got := extractStr(term2, 0, 4, 0); got != "     " {
		t.Fatalf("row 0 after ED-above = %q, want blank", got)
	}

	term3 := New(WithSize(geom.Vec2{C: 5, R: 3}))
	_, _ = term3.Write([]byte(LineFeedMode))
	_, _ = term3.Write([]byte("aaaaa\r\nbbbbb\r\nccccc"))
	_, _ = term3.Write([]byte("\x1b[2J")) // ED all
	if got := extractStr(term3, 0, 4, 1); got != "     " {
		t.Fatalf("row 1 after ED-all = %q, want blank", got)
	}

	term4 := New(WithSize(geom.Vec2{C: 5, R: 1}))
	_, _ = term4.Write([]byte("aaaaa"))
	_, _ = term4.Write([]byte("\x1b[3G"))
	_, _ = term4.Write([]byte("\x1b[1K")) // EL left
	if got := extractStr(term4, 0, 4, 0); got != "   aa" {
		t.Fatalf("row 0 after EL-left = %q, want \"   aa\"", got)
	}
}

func TestCsiDispatchDeviceAttributesAndReports(t *testing.T) {
	var out bytesBuf
	term := New(WithWriter(&out), WithSize(geom.Vec2{C: 10, R: 5}))
	_, _ = term.Write([]byte("\x1b[c")) // DA
	if !bytesContains(out.data, "\033[?6c") {
		t.Errorf("DA should write device attributes response, got %q", out.data)
	}
	out.data = nil
	_, _ = term.Write([]byte("\x1b[5n")) // DSR
	if !bytesContains(out.data, "\033[0n") {
		t.Errorf("DSR should write status response, got %q", out.data)
	}
	out.data = nil
	_, _ = term.Write([]byte("\x1b[6n")) // CPR
	if !bytesContains(out.data, "\033[") {
		t.Errorf("CPR should write cursor position response, got %q", out.data)
	}
}

// bytesBuf is a minimal io.Writer collecting written bytes, avoiding a
// bytes.Buffer import collision with other test files in this package.
type bytesBuf struct{ data []byte }

func (b *bytesBuf) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func bytesContains(data []byte, sub string) bool {
	s := string(data)
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCsiDispatchScrollRegionAndCursorSave(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 10}))
	_, _ = term.Write([]byte("\x1b[3;7r")) // DECSTBM set scroll region
	st := term.(*terminal)
	if st.top != 2 || st.bottom != 6 {
		t.Fatalf("scroll region = [%d,%d], want [2,6]", st.top, st.bottom)
	}

	_, _ = term.Write([]byte("\x1b[5;5H"))
	_, _ = term.Write([]byte("\x1b[s")) // DECSC save cursor (CSI form)
	_, _ = term.Write([]byte("\x1b[1;1H"))
	_, _ = term.Write([]byte("\x1b[u")) // DECRC restore cursor (CSI form)
	if cur := term.Cursor(); cur.R != 4 || cur.C != 4 {
		t.Fatalf("after CSI u restore: %+v, want R=4,C=4", cur)
	}
}

func TestCsiDispatchCursorStyle(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 5}))
	_, _ = term.Write([]byte("\x1b[3 q")) // DECSCUSR blinking underline
	if cur := term.Cursor(); cur.Style != CursorStyleBlinkUnderline {
		t.Fatalf("cursor style = %v, want CursorStyleBlinkUnderline", cur.Style)
	}
}

func TestCsiDispatchUnknownFinalByteDoesNotPanic(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 5}))
	// '~' is not a recognized CSI final byte -> hits the `unknown:` label.
	if _, err := term.Write([]byte("\x1b[5~")); err != nil {
		t.Fatalf("unknown CSI sequence should not error: %v", err)
	}
}

func TestEscDispatchBroadSweep(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 5}))
	seqs := []string{
		"\x1bD",  // IND
		"\x1bE",  // NEL
		"\x1bH",  // HTS
		"\x1bM",  // RI
		"\x1bZ",  // DECID (no-op)
		"\x1b=",  // DECPAM
		"\x1b>",  // DECPNM
		"\x1b7",  // DECSC
		"\x1b8",  // DECRC
		"\x1b\\", // ST (no-op)
		"\x1b0",  // line drawing set
		"\x1bB",  // USASCII
		"\x1bA",  // UK (ignored)
		"\x1bc",  // RIS - reset to initial state
	}
	for _, s := range seqs {
		if _, err := term.Write([]byte(s)); err != nil {
			t.Fatalf("Write(%q) error: %v", s, err)
		}
	}
}

func TestEscDispatchUnknownDoesNotPanic(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 5}))
	if _, err := term.Write([]byte("\x1b\x01")); err != nil {
		t.Fatalf("unknown ESC sequence should not error: %v", err)
	}
}

func TestOscDispatchTitleAndDirectory(t *testing.T) {
	term := New()
	_, _ = term.Write([]byte("\x1b]2;my title\x07"))
	if got := term.Title(); got != "my title" {
		t.Fatalf("Title() = %q, want \"my title\"", got)
	}
	_, _ = term.Write([]byte("\x1b]7;/tmp\x07"))
	if got := term.Directory(); got != "/tmp" {
		t.Fatalf("Directory() = %q, want \"/tmp\"", got)
	}
}

func TestOscDispatchColorSetAndQuery(t *testing.T) {
	var out bytesBuf
	term := New(WithWriter(&out))
	_, _ = term.Write([]byte("\x1b]10;#ff0000\x07")) // set FG color
	out.data = nil
	_, _ = term.Write([]byte("\x1b]10;?\x07")) // query FG color
	if len(out.data) == 0 {
		t.Fatal("OSC 10 query should write a color response")
	}

	out.data = nil
	_, _ = term.Write([]byte("\x1b]11;#00ff00\x07")) // set BG color
	out.data = nil
	_, _ = term.Write([]byte("\x1b]11;?\x07")) // query BG color
	if len(out.data) == 0 {
		t.Fatal("OSC 11 query should write a color response")
	}

	out.data = nil
	_, _ = term.Write([]byte("\x1b]4;1;#0000ff\x07")) // set palette color 1
	out.data = nil
	_, _ = term.Write([]byte("\x1b]4;1;?\x07")) // query palette color 1
	if len(out.data) == 0 {
		t.Fatal("OSC 4 query should write a color response")
	}
}

func TestOscDispatchInvalidColorLogs(t *testing.T) {
	var buf bytesBuf
	term := New(WithWriter(&buf))
	st := term.(*terminal)
	var logBuf bytesBuf
	st.DebugLogger = log.New(&logBuf, "", 0)
	_, _ = term.Write([]byte("\x1b]10;not-a-color\x07"))
	if logBuf.data == nil {
		t.Fatal("an invalid OSC 10 color spec should log via logf")
	}
}

func TestOscDispatchUnknownCommand(t *testing.T) {
	term := New()
	if _, err := term.Write([]byte("\x1b]9999;x\x07")); err != nil {
		t.Fatalf("unknown OSC command should not error: %v", err)
	}
}

func TestWriteSyncReportsSyncMode(t *testing.T) {
	term := New()
	_, syncing, err := term.WriteSync([]byte("hello"))
	if err != nil {
		t.Fatalf("WriteSync error: %v", err)
	}
	if syncing {
		t.Fatal("syncing should be false before enabling ModeSyncUpdate")
	}

	_, _, _ = term.WriteSync([]byte("\x1b[?2026h")) // enable sync update mode
	_, syncing, _ = term.WriteSync([]byte("x"))
	if !syncing {
		t.Fatal("syncing should be true once ModeSyncUpdate is enabled")
	}
}

func TestFlowResultCoord(t *testing.T) {
	term := New(WithSize(geom.Vec2{C: 10, R: 2}))
	_, _ = term.Write([]byte(LineFeedMode))
	_, _ = term.Write([]byte("hello"))

	result := term.Flow(geom.Vec2{C: 10, R: 2}, term.Root())
	if !result.OK {
		t.Fatal("Flow should succeed")
	}

	_, ok := result.Coord(geom.Vec2{R: -1, C: 0})
	if ok {
		t.Fatal("Coord with negative row should not be ok")
	}
	_, ok = result.Coord(geom.Vec2{R: 0, C: 99999})
	if ok {
		t.Fatal("Coord with out-of-range column should not be ok")
	}
	if len(result.Lines) > 0 && len(result.Lines[0].Chars) > 0 {
		got, ok := result.Coord(geom.Vec2{R: 0, C: 0})
		if !ok {
			t.Fatal("Coord within bounds should be ok")
		}
		if got.R != result.Lines[0].R {
			t.Fatalf("Coord row = %d, want %d", got.R, result.Lines[0].R)
		}
	}
}

func TestScreenLineRoot(t *testing.T) {
	l := ScreenLine{R: 5, C0: 3, C1: 8}
	got := l.Root()
	want := geom.Vec2{R: 5, C: 3}
	if got != want {
		t.Fatalf("ScreenLine.Root() = %v, want %v", got, want)
	}
}

func TestDirtyScreenChanged(t *testing.T) {
	d := &Dirty{}
	if d.ScreenChanged() {
		t.Fatal("fresh Dirty should not report ScreenChanged")
	}
	d.Flag |= ChangedScreen
	if !d.ScreenChanged() {
		t.Fatal("Dirty with ChangedScreen flag should report ScreenChanged")
	}
}

func TestKeyProtocolString(t *testing.T) {
	if got := KeyLegacy.String(); got != "legacy" {
		t.Fatalf("KeyLegacy.String() = %q, want \"legacy\"", got)
	}
	if got := KeyReportAll.String(); got == "legacy" || got == "" {
		t.Fatalf("KeyReportAll.String() = %q, want a descriptive kitty string", got)
	}
}

func TestKeyProtocolStateIsEnabled(t *testing.T) {
	st := NewKeyProtocolState()
	if st.IsEnabled() {
		t.Fatal("a fresh KeyProtocolState should not be enabled")
	}
	st.Push(KeyDisambiguateEscape)
	if !st.IsEnabled() {
		t.Fatal("KeyProtocolState should be enabled once flags are pushed")
	}
}
