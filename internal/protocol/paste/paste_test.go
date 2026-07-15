package paste

import (
	"bytes"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestEncodeWithBracketedPasteMode(t *testing.T) {
	got := Encode(emu.ModeBracketedPaste, "hello\nworld")
	want := []byte("\x1b[200~hello\nworld\x1b[201~")
	if !bytes.Equal(got, want) {
		t.Fatalf("Encode = %q, want %q", got, want)
	}
}

func TestEncodeWithoutBracketedPasteMode(t *testing.T) {
	got := Encode(0, "hello\nworld")
	if !bytes.Equal(got, []byte("hello\nworld")) {
		t.Fatalf("Encode = %q, want plain text unmodified", got)
	}
}

func TestEncodeOtherModesDontTrigger(t *testing.T) {
	// Some unrelated mode bit being set must not accidentally enable
	// bracketing.
	got := Encode(emu.ModeFocus|emu.ModeAltScreen, "hi")
	if !bytes.Equal(got, []byte("hi")) {
		t.Fatalf("Encode = %q, want plain text unmodified", got)
	}
}

func TestEncodeEmptyText(t *testing.T) {
	got := Encode(emu.ModeBracketedPaste, "")
	want := []byte("\x1b[200~\x1b[201~")
	if !bytes.Equal(got, want) {
		t.Fatalf("Encode(empty) = %q, want %q", got, want)
	}
}
