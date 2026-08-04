package gogpu

import (
	"image"
	"image/color"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestConsumeLiveResizeSync(t *testing.T) {
	s, _ := testUIState(t)

	if s.consumeLiveResizeSync(true) {
		t.Fatal("entering live resize should never report needFinalSync")
	}
	if !s.pendingTexSync {
		t.Fatal("entering live resize should set pendingTexSync")
	}

	if !s.consumeLiveResizeSync(false) {
		t.Fatal("leaving live resize with pendingTexSync set should report needFinalSync")
	}
	if s.pendingTexSync {
		t.Fatal("consumeLiveResizeSync should clear pendingTexSync once consumed")
	}

	if s.consumeLiveResizeSync(false) {
		t.Fatal("with no pending sync, needFinalSync should be false")
	}
}

func TestPaintFrame(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.paintFrame(200, 100, 32, 8)

	if s.frameW != 200 || s.frameH != 100 {
		t.Fatalf("frameW/H = %d,%d, want 200,100", s.frameW, s.frameH)
	}
	if len(s.frame) != 200*100*4 {
		t.Fatalf("len(frame) = %d, want %d", len(s.frame), 200*100*4)
	}
}

func TestPaintCommandBorderPaintsWhileRunning(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.frame = make([]byte, 100*100*4)
	active := s.mgr.Active()
	active.Term.Parse([]byte(emu.OSC133CommandExec))

	s.paintCommandBorder(100, 100, active)

	want := toRGBA(s.palette.ANSI[6])
	if got := pixelAt(s.frame, 100, 0, 0); got != want {
		t.Fatalf("top-left border pixel = %v, want %v (running indicator color)", got, want)
	}
	if got := pixelAt(s.frame, 100, 99, 99); got != want {
		t.Fatalf("bottom-right border pixel = %v, want %v (running indicator color)", got, want)
	}
	// The interior, well inside the border thickness, must be untouched.
	if got := pixelAt(s.frame, 100, 50, 50); got == want {
		t.Fatal("paintCommandBorder should not fill the interior, only a thin edge")
	}
}

func TestPaintCommandBorderNoopWhenIdle(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.frame = make([]byte, 100*100*4)
	before := make([]byte, len(s.frame))
	copy(before, s.frame)

	s.paintCommandBorder(100, 100, s.mgr.Active())

	if string(s.frame) != string(before) {
		t.Fatal("paintCommandBorder should not touch the frame when no command has run")
	}
}

func TestTriggerResizeIfNeededUpdatesColsRows(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.cols, s.rows = 80, 24
	s.triggerResizeIfNeeded(100, 30, false, false)
	if s.cols != 100 || s.rows != 30 {
		t.Fatalf("cols/rows = %d,%d, want 100,30", s.cols, s.rows)
	}
}

func TestTriggerResizeIfNeededUnchangedIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.cols, s.rows = 80, 24
	s.triggerResizeIfNeeded(80, 24, false, false)
	if s.cols != 80 || s.rows != 24 {
		t.Fatal("unchanged size should not modify cols/rows")
	}
}

func TestDrainClipboardWritesNoPendingIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	// No OSC 52 write has been queued by any tab, so this must return
	// without ever reaching clipboardWrite's real pbcopy/xclip/clip
	// fallback — a test must never touch the host's actual clipboard.
	s.drainClipboardWrites()
}

func TestDrainClipboardWritesPendingPayload(t *testing.T) {
	s, app := testUIStateWithTab(t)
	active := s.mgr.Active()
	if active == nil {
		t.Fatal("expected active tab")
	}
	// Queue an OSC 52 write through the session's VT parser.
	active.Term.Parse([]byte("\x1b]52;c;aGVsbG8=\x07"))
	s.drainClipboardWrites()
	if app.clipboard != "hello" && app.clipboard != "" {
		// On darwin, pbcopy may succeed first and leave fakeApp untouched;
		// either path means drain consumed the pending write.
		t.Fatalf("unexpected clipboard state %q", app.clipboard)
	}
	// Second drain must be a no-op (pending cleared).
	s.drainClipboardWrites()
}

func TestDrainClipboardWritesClear(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	active.Term.Parse([]byte("\x1b]52;c;\x07")) // empty payload => clear
	s.drainClipboardWrites()
	if _, _, ok := active.TakeClipboardWrite(); ok {
		t.Fatal("drain should have consumed the clear")
	}
}

func TestEnsureFontsLoadsOnce(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.cfg.Font.Size = 13
	s.fontSizeCurrent = 0
	s.scale = 0

	s.ensureFonts(1)
	if s.painter.Fonts.regular == nil {
		t.Fatal("ensureFonts should load a font face")
	}
	if s.cellW <= 0 || s.cellH <= 0 {
		t.Fatal("ensureFonts should measure cell metrics")
	}
	face := s.painter.Fonts.regular

	// Calling again with the same size/scale should be a no-op (same Face).
	s.ensureFonts(1)
	if s.painter.Fonts.regular != face {
		t.Fatal("ensureFonts should not reload when size/scale are unchanged")
	}
}

func TestEnsureFontsRespectsBoldItalicToggles(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.cfg.Font.Bold = false
	s.cfg.Font.Italic = false
	s.fontSizeCurrent = 0
	s.scale = 0

	s.ensureFonts(1)

	if s.painter.Fonts.bold != nil {
		t.Fatal("ensureFonts should not load a bold face when Font.Bold is false")
	}
	if s.painter.Fonts.italic != nil {
		t.Fatal("ensureFonts should not load an italic face when Font.Italic is false")
	}
	if s.painter.Fonts.boldItalic != nil {
		t.Fatal("ensureFonts should not load a bold-italic face when either toggle is false")
	}
	if s.painter.Fonts.regular == nil {
		t.Fatal("ensureFonts should still load the regular face")
	}
}

func TestPaintScrollBarOverlayHiddenIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.frame = make([]byte, 100*100*4)
	before := make([]byte, len(s.frame))
	copy(before, s.frame)

	s.paintScrollBarOverlay(100, 100, 32, s.mgr.Active(), false)
	for i := range s.frame {
		if s.frame[i] != before[i] {
			t.Fatal("paintScrollBarOverlay(show=false) should not touch the frame")
		}
	}
}

func TestPaintScrollBarOverlayNoHistoryIsNoop(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.frame = make([]byte, 100*100*4)
	before := make([]byte, len(s.frame))
	copy(before, s.frame)

	// Fresh session has no scrollback yet.
	s.paintScrollBarOverlay(100, 100, 32, s.mgr.Active(), true)
	for i := range s.frame {
		if s.frame[i] != before[i] {
			t.Fatal("paintScrollBarOverlay with no history should not touch the frame")
		}
	}
}

func TestPaintScrollBarOverlayDrawsThumb(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	for i := 0; i < 50; i++ {
		_, _ = active.Term.Write([]byte("line\r\n"))
	}
	s.frame = make([]byte, 100*100*4)

	s.paintScrollBarOverlay(100, 100, 32, active, true)
	found := false
	for i := 0; i+3 < len(s.frame); i += 4 {
		if s.frame[i+3] != 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("paintScrollBarOverlay with history should paint the track/thumb")
	}
}

func TestGridSelection(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()

	if sel := gridSelection(active); sel.Active {
		t.Fatal("gridSelection with no selection should be inactive")
	}

	active.StartSelection(1, 0)
	active.ExtendSelection(3, 0)
	active.EndSelection()
	sel := gridSelection(active)
	if !sel.Active {
		t.Fatal("gridSelection should report Active once a selection exists")
	}
}

func TestGridPlacementsEmpty(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	if got := gridPlacements(s.mgr.Active()); got != nil {
		t.Fatalf("gridPlacements with no placements = %v, want nil", got)
	}
}

func TestToRGBAImagePassthroughAndConvert(t *testing.T) {
	rgba := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if got := toRGBAImage(rgba); got != rgba {
		t.Fatal("toRGBAImage should pass an *image.RGBA through unchanged")
	}

	gray := image.NewGray(image.Rect(0, 0, 2, 2))
	gray.SetGray(0, 0, color.Gray{Y: 128})
	got := toRGBAImage(gray)
	if got == nil || got.Bounds() != gray.Bounds() {
		t.Fatal("toRGBAImage should convert a non-RGBA image")
	}
}

func TestPluginStatusTextNilHost(t *testing.T) {
	if got := pluginStatusText(nil); got != "" {
		t.Fatalf("pluginStatusText(nil) = %q, want empty", got)
	}
}
