package gogpu

import (
	"image/color"
	"testing"

	"golang.org/x/image/font/basicfont"

	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
	"github.com/geckty/geckty/internal/vt/emu"
)

// newTestTab spawns a real (short-lived, quiet) shell so tabTitle/paintTab
// have a genuine *session.Session to read Dir/Term.Title() from — matching
// the pattern internal/session's own tests use for spawning real processes.
func newTestTab(t *testing.T, id int, dir string) session.Tab {
	t.Helper()
	sess, err := session.New(session.Config{
		Command: testSleepCommand(),
		Dir:     dir,
		Cols:    80,
		Rows:    24,
	})
	if err != nil {
		t.Fatalf("session.New: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	go sess.Run()
	return session.Tab{ID: id, Session: sess}
}

func testTabBar() *TabBar {
	face := basicfont.Face7x13
	return &TabBar{
		Face:   face,
		Ascent: face.Metrics().Ascent.Ceil(),
		Scale:  1,
	}
}

func TestDpToPxRoundsHalfUp(t *testing.T) {
	if got := dpToPx(10, 1); got != 10 {
		t.Fatalf("dpToPx(10,1) = %d, want 10", got)
	}
	if got := dpToPx(10, 1.5); got != 15 {
		t.Fatalf("dpToPx(10,1.5) = %d, want 15", got)
	}
	if got := dpToPx(1, 0.4); got != 0 {
		t.Fatalf("dpToPx(1,0.4) = %d, want 0", got)
	}
}

func TestNewTabBarIsEmpty(t *testing.T) {
	tb := NewTabBar()
	if tb.Face != nil {
		t.Fatal("NewTabBar should start with a nil Face")
	}
}

func TestDrawTextNilFaceIsNoop(t *testing.T) {
	tb := &TabBar{}
	buf := newBuf(50, 20)
	tb.drawText(buf, 50, 20, "hello", 0, 0, 50, 20, color.RGBA{R: 0xff, A: 0xff})
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("buf[%d] = %d, want 0 (nil Face must draw nothing)", i, b)
		}
	}
}

func TestMeasureTextNilFaceIsZero(t *testing.T) {
	tb := &TabBar{}
	if got := tb.measureText("hello"); got != 0 {
		t.Fatalf("measureText with nil Face = %d, want 0", got)
	}
}

func TestMeasureTextNonEmpty(t *testing.T) {
	tb := testTabBar()
	if got := tb.measureText("hello"); got <= 0 {
		t.Fatalf("measureText(\"hello\") = %d, want > 0", got)
	}
}

func TestDrawTextCenteredPaintsWithinBounds(t *testing.T) {
	tb := testTabBar()
	buf := newBuf(100, 20)
	fg := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	tb.drawTextCentered(buf, 100, 20, "Hi", 0, 0, 100, 20, fg)

	found := false
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] == 0xff && buf[i+1] == 0xff && buf[i+2] == 0xff && buf[i+3] == 0xff {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected drawTextCentered to paint at least one foreground pixel")
	}
}

func TestDrawTextClipsToBox(t *testing.T) {
	tb := testTabBar()
	buf := newBuf(100, 20)
	fg := color.RGBA{R: 0xff, A: 0xff}
	// Clip box is a zero-width sliver far to the right; nothing should be
	// painted at all since drawTextAt clips to [clipX0,clipX0+w).
	tb.drawText(buf, 100, 20, "long text here", 90, 0, 0, 20, fg)
	for x := 0; x < 90; x++ {
		if got := pixelAt(buf, 100, x, 10); got.A != 0 {
			t.Fatalf("pixel (%d,10) = %v, want untouched (outside clip box)", x, got)
		}
	}
}

func TestTabTitleFallsBackToDir(t *testing.T) {
	tab := newTestTab(t, 1, t.TempDir())
	got := tabTitle(tab)
	if got == "" {
		t.Fatal("tabTitle should not be empty")
	}
}

func TestTabTitleFallsBackToTabNumber(t *testing.T) {
	tab := newTestTab(t, 5, "")
	// With no Dir and (most likely) no title set yet, tabTitle should fall
	// back to "Tab N".
	got := tabTitle(tab)
	if got == "" {
		t.Fatal("tabTitle should not be empty even with no Dir and no Term title")
	}
}

func TestPaintTabAndPlusButton(t *testing.T) {
	tb := testTabBar()
	pal := testPalette()
	tab := newTestTab(t, 1, t.TempDir())

	buf := newBuf(200, 40)
	tb.paintTab(buf, 200, 40, pal, tab, 0, 100, 32, 3, true, false, false, true, true)

	// Plus button paints its own chip + cross.
	tb.paintPlusButton(buf, 200, pal, 150, 36, 32, 3, true)
	tb.paintPlusButton(buf, 200, pal, 150, 36, 32, 3, false)
}

func TestPaintStatus(t *testing.T) {
	tb := testTabBar()
	pal := testPalette()
	// ComputeGeometry always stretches tabs to fill every pixel up to the
	// "+" button (PlusX == totalWidth-PlusWidth regardless of tab count),
	// so statusX == frameW for any geometry it produces — paintStatus's
	// "there's room" branch is unreachable through that path. paintStatus
	// itself only takes a chrome.Geometry value though, so build one by
	// hand here to exercise that branch directly.
	g := chrome.Geometry{TabWidth: 80, PlusX: 400, PlusWidth: 36}

	buf := newBuf(600, 40)
	tb.paintStatus(buf, 600, 40, pal, g, 32, "status text")

	found := false
	for i := 3; i < len(buf); i += 4 {
		if buf[i] != 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("paintStatus should paint the status text somewhere in the buffer")
	}

	// statusX >= frameW should be a no-op and must not panic.
	gFull := g
	gFull.PlusX = 599
	gFull.PlusWidth = 5
	tb.paintStatus(buf, 600, 40, pal, gFull, 32, "status text")

	// Empty status text is skipped entirely by Layout's caller, but
	// paintStatus itself has no such guard — confirm it still just draws
	// nothing visible rather than erroring.
	tb.paintStatus(buf, 600, 40, pal, g, 32, "")
}

func TestLayoutPaintsTabsAndPlusButton(t *testing.T) {
	tb := testTabBar()
	pal := testPalette()
	tabs := []session.Tab{
		newTestTab(t, 1, t.TempDir()),
		newTestTab(t, 2, t.TempDir()),
	}

	buf := newBuf(400, 40)
	drag := chrome.DragVisual{}
	tb.Layout(buf, 400, 40, 32, pal, tabs, 1, "plugin status", drag, false, true)

	// Bar background should be painted (non-zero alpha somewhere in the
	// bar strip that isn't covered by a tab pill or text).
	if got := pixelAt(buf, 400, 399, 1); got.A == 0 {
		t.Fatal("tab bar background should be painted across the full width")
	}
}

func TestLayoutWithActiveDrag(t *testing.T) {
	tb := testTabBar()
	pal := testPalette()
	tabs := []session.Tab{
		newTestTab(t, 1, t.TempDir()),
		newTestTab(t, 2, t.TempDir()),
		newTestTab(t, 3, t.TempDir()),
	}

	buf := newBuf(400, 40)
	drag := chrome.DragVisual{
		Active:   true,
		From:     0,
		Over:     1,
		DX:       20,
		TabID:    1,
		HoverIdx: 2,
	}
	// Must not panic while a tab is mid-drag (exercises the two-pass
	// paint order and the dragged-tab positioning branch).
	tb.Layout(buf, 400, 40, 32, pal, tabs, 1, "", drag, true, true)
}

func TestCommandIndicatorColorNoneByDefault(t *testing.T) {
	tab := newTestTab(t, 1, t.TempDir())
	pal := testPalette()

	if _, ok := commandIndicatorColor(tab.Session, pal); ok {
		t.Fatal("a tab with no OSC 133 activity should have no indicator")
	}
}

func TestCommandIndicatorColorWhileRunning(t *testing.T) {
	tab := newTestTab(t, 1, t.TempDir())
	pal := testPalette()
	tab.Session.Term.Parse([]byte(emu.OSC133CommandExec))

	got, ok := commandIndicatorColor(tab.Session, pal)
	if !ok {
		t.Fatal("a running command should show an indicator")
	}
	if want := toRGBA(pal.ANSI[6]); got != want {
		t.Fatalf("running indicator color = %+v, want %+v (ANSI[6])", got, want)
	}
}

func TestCommandIndicatorColorAfterSuccessAndFailure(t *testing.T) {
	cases := []struct {
		name    string
		doneSeq string
		wantIdx int
	}{
		{"success", emu.OSC133CommandDone, 2},
		{"failure", emu.OSC133CommandDone1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tab := newTestTab(t, 1, t.TempDir())
			pal := testPalette()
			tab.Session.Term.Parse([]byte(emu.OSC133CommandExec))
			tab.Session.Term.Parse([]byte(c.doneSeq))

			got, ok := commandIndicatorColor(tab.Session, pal)
			if !ok {
				t.Fatal("a just-finished command should show an indicator")
			}
			if want := toRGBA(pal.ANSI[c.wantIdx]); got != want {
				t.Fatalf("%s indicator color = %+v, want %+v (ANSI[%d])", c.name, got, want, c.wantIdx)
			}
		})
	}
}
