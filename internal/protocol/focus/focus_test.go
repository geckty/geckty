package focus

import (
	"bytes"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestEncodeGainedWithFocusMode(t *testing.T) {
	got := Encode(emu.ModeFocus, true)
	if !bytes.Equal(got, []byte("\x1b[I")) {
		t.Fatalf("Encode(gained) = %q, want \\x1b[I", got)
	}
}

func TestEncodeLostWithFocusMode(t *testing.T) {
	got := Encode(emu.ModeFocus, false)
	if !bytes.Equal(got, []byte("\x1b[O")) {
		t.Fatalf("Encode(lost) = %q, want \\x1b[O", got)
	}
}

func TestEncodeWithoutFocusModeReturnsNil(t *testing.T) {
	if got := Encode(0, true); got != nil {
		t.Fatalf("Encode without ModeFocus = %q, want nil", got)
	}
	if got := Encode(0, false); got != nil {
		t.Fatalf("Encode without ModeFocus = %q, want nil", got)
	}
}

func TestEncodeOtherModesDontTrigger(t *testing.T) {
	if got := Encode(emu.ModeBracketedPaste|emu.ModeAltScreen, true); got != nil {
		t.Fatalf("Encode = %q, want nil", got)
	}
}
