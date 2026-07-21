package mouse

import (
	"bytes"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestTrackingEnabled(t *testing.T) {
	cases := []struct {
		mode emu.ModeFlag
		want bool
	}{
		{0, false},
		{emu.ModeAltScreen, false},
		{emu.ModeMouseButton, true},
		{emu.ModeMouseMotion, true},
		{emu.ModeMouseX10, true},
		{emu.ModeMouseMany, true},
		{emu.ModeMouseSgr, false}, // SGR alone isn't a tracking mode, just a coordinate format
	}
	for _, c := range cases {
		if got := TrackingEnabled(c.mode); got != c.want {
			t.Errorf("TrackingEnabled(%v) = %v, want %v", c.mode, got, c.want)
		}
	}
}

func TestEncodeWheelNoTrackingReturnsNotOK(t *testing.T) {
	if _, ok := EncodeWheel(0, Up, 5, 5, 0); ok {
		t.Fatal("expected ok=false with no tracking mode active")
	}
}

func TestEncodeWheelSGR(t *testing.T) {
	mode := emu.ModeMouseButton | emu.ModeMouseSgr
	up, ok := EncodeWheel(mode, Up, 10, 20, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[<64;10;20M"); !bytes.Equal(up, want) {
		t.Fatalf("wheel up SGR = %q, want %q", up, want)
	}

	down, _ := EncodeWheel(mode, Down, 10, 20, 0)
	if want := []byte("\x1b[<65;10;20M"); !bytes.Equal(down, want) {
		t.Fatalf("wheel down SGR = %q, want %q", down, want)
	}
}

func TestEncodeWheelSGRModifiers(t *testing.T) {
	mode := emu.ModeMouseButton | emu.ModeMouseSgr
	got, _ := EncodeWheel(mode, Up, 1, 1, ModShift|ModCtrl)
	// button 64 (wheel up) | 4 (shift) | 16 (ctrl) = 84
	want := []byte("\x1b[<84;1;1M")
	if !bytes.Equal(got, want) {
		t.Fatalf("wheel up with modifiers = %q, want %q", got, want)
	}
}

func TestEncodeWheelLegacy(t *testing.T) {
	// Button-tracking mode without SGR falls back to the legacy X10
	// byte encoding.
	mode := emu.ModeMouseButton
	up, ok := EncodeWheel(mode, Up, 5, 6, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := []byte{0x1b, '[', 'M', 0x60 + 32, byte(5 + 32), byte(6 + 32)}
	if !bytes.Equal(up, want) {
		t.Fatalf("wheel up legacy = %v, want %v", up, want)
	}

	down, _ := EncodeWheel(mode, Down, 5, 6, 0)
	if down[3] != up[3]+1 {
		t.Fatalf("legacy wheel down button byte = %#x, want up+1 = %#x", down[3], up[3]+1)
	}
}

func TestEncodeWheelLegacyClampsLargeCoordinates(t *testing.T) {
	mode := emu.ModeMouseButton
	got, _ := EncodeWheel(mode, Up, 500, 500, 0)
	// Legacy encoding can't represent coordinates beyond 223 in a
	// single byte (255 - 32 offset); must clamp, not overflow/wrap.
	if got[4] != byte(223+32) || got[5] != byte(223+32) {
		t.Fatalf("legacy encoding did not clamp large coordinates: %v", got)
	}
}

func TestEncodeButtonNoTracking(t *testing.T) {
	if _, ok := EncodeButton(0, ButtonLeft, true, 1, 1, 0); ok {
		t.Fatal("expected ok=false with no tracking mode active")
	}
}

func TestEncodeButtonPressSGR(t *testing.T) {
	mode := emu.ModeMouseButton | emu.ModeMouseSgr
	got, ok := EncodeButton(mode, ButtonLeft, true, 10, 20, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[<0;10;20M"); !bytes.Equal(got, want) {
		t.Fatalf("left press SGR = %q, want %q", got, want)
	}

	got, _ = EncodeButton(mode, ButtonRight, true, 10, 20, 0)
	if want := []byte("\x1b[<2;10;20M"); !bytes.Equal(got, want) {
		t.Fatalf("right press SGR = %q, want %q", got, want)
	}
}

func TestEncodeButtonReleaseSGR(t *testing.T) {
	mode := emu.ModeMouseButton | emu.ModeMouseSgr
	got, ok := EncodeButton(mode, ButtonLeft, false, 10, 20, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Release is 'm' (lowercase), not 'M', in SGR mode.
	if want := []byte("\x1b[<0;10;20m"); !bytes.Equal(got, want) {
		t.Fatalf("left release SGR = %q, want %q", got, want)
	}
}

func TestEncodeButtonReleaseSuppressedUnderX10Only(t *testing.T) {
	// X10 mode predates release tracking; clients don't expect it.
	mode := emu.ModeMouseX10
	if _, ok := EncodeButton(mode, ButtonLeft, false, 1, 1, 0); ok {
		t.Fatal("expected release to be suppressed under X10-only tracking")
	}
	// Press is still reported under X10.
	if _, ok := EncodeButton(mode, ButtonLeft, true, 1, 1, 0); !ok {
		t.Fatal("expected press to be reported under X10-only tracking")
	}
}

func TestEncodeButtonReleaseReportedUnderButtonMode(t *testing.T) {
	mode := emu.ModeMouseButton | emu.ModeMouseSgr
	if _, ok := EncodeButton(mode, ButtonLeft, false, 1, 1, 0); !ok {
		t.Fatal("expected release to be reported once ModeMouseButton is set")
	}
}

func TestEncodeButtonLegacy(t *testing.T) {
	mode := emu.ModeMouseButton
	press, ok := EncodeButton(mode, ButtonMiddle, true, 5, 6, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := []byte{0x1b, '[', 'M', byte(ButtonMiddle) + 32, byte(5 + 32), byte(6 + 32)}
	if !bytes.Equal(press, want) {
		t.Fatalf("middle press legacy = %v, want %v", press, want)
	}

	release, _ := EncodeButton(mode, ButtonMiddle, false, 5, 6, 0)
	if release[3] != 3+32 {
		t.Fatalf("legacy release button byte = %#x, want X10 release code 3+32", release[3])
	}
}

func TestMotionCapable(t *testing.T) {
	cases := []struct {
		mode emu.ModeFlag
		want bool
	}{
		{emu.ModeMouseX10, false},
		{emu.ModeMouseButton, false},
		{emu.ModeMouseMotion, true},
		{emu.ModeMouseMany, true},
	}
	for _, c := range cases {
		if got := motionCapable(c.mode); got != c.want {
			t.Errorf("motionCapable(%v) = %v, want %v", c.mode, got, c.want)
		}
	}
}

func TestEncodeMotionRequiresMotionMode(t *testing.T) {
	// Plain button-click tracking doesn't include drag reporting.
	if _, ok := EncodeMotion(emu.ModeMouseButton, ButtonLeft, 1, 1, 0); ok {
		t.Fatal("expected ok=false under ModeMouseButton alone (no motion tracking)")
	}
	if _, ok := EncodeMotion(emu.ModeMouseMotion, ButtonLeft, 1, 1, 0); !ok {
		t.Fatal("expected ok=true under ModeMouseMotion")
	}
}

func TestEncodeMotionSGRSetsMotionBit(t *testing.T) {
	mode := emu.ModeMouseMotion | emu.ModeMouseSgr
	got, ok := EncodeMotion(mode, ButtonLeft, 10, 20, 0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Motion adds 32 to the button code (0 for left -> 32).
	if want := []byte("\x1b[<32;10;20M"); !bytes.Equal(got, want) {
		t.Fatalf("motion SGR = %q, want %q", got, want)
	}
}

func TestEncodeButtonModifiers(t *testing.T) {
	mode := emu.ModeMouseButton | emu.ModeMouseSgr
	got, _ := EncodeButton(mode, ButtonLeft, true, 1, 1, ModShift|ModAlt)
	// button 0 (left) | 4 (shift) | 8 (alt) = 12
	if want := []byte("\x1b[<12;1;1M"); !bytes.Equal(got, want) {
		t.Fatalf("press with modifiers = %q, want %q", got, want)
	}
}
