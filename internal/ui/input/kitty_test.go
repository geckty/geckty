package input

import (
	"bytes"
	"testing"

	"gioui.org/io/key"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestEncodeKittyLegacyModeFallsThrough(t *testing.T) {
	if _, ok := EncodeKitty(emu.KeyLegacy, key.Event{Name: key.NameEscape, State: key.Press}); ok {
		t.Fatal("expected ok=false when Kitty protocol is not active")
	}
}

func TestEncodeKittyEscape(t *testing.T) {
	got, ok := EncodeKitty(emu.KeyDisambiguateEscape, key.Event{Name: key.NameEscape, State: key.Press})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[27u"); !bytes.Equal(got, want) {
		t.Fatalf("EncodeKitty(Escape) = %q, want %q", got, want)
	}
}

func TestEncodeKittyCtrlLetter(t *testing.T) {
	got, ok := EncodeKitty(emu.KeyDisambiguateEscape, key.Event{Name: "A", Modifiers: key.ModCtrl, State: key.Press})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[97;5u"); !bytes.Equal(got, want) {
		t.Fatalf("EncodeKitty(Ctrl+A) = %q, want %q", got, want)
	}
}

func TestEncodeKittyCommandMapsToSuper(t *testing.T) {
	// key.ModCommand (macOS Cmd) and key.ModSuper (Linux/Windows logo
	// key) both map to the Kitty protocol's single "super" bit —
	// there's no separate Kitty modifier for Cmd.
	withCommand, ok := EncodeKitty(emu.KeyDisambiguateEscape, key.Event{Name: "A", Modifiers: key.ModCommand | key.ModCtrl, State: key.Press})
	if !ok {
		t.Fatal("expected ok=true")
	}
	withSuper, ok := EncodeKitty(emu.KeyDisambiguateEscape, key.Event{Name: "A", Modifiers: key.ModSuper | key.ModCtrl, State: key.Press})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !bytes.Equal(withCommand, withSuper) {
		t.Fatalf("ModCommand and ModSuper encoded differently: %q vs %q", withCommand, withSuper)
	}
}

func TestEncodeKittyPlainLetterFallsThrough(t *testing.T) {
	if _, ok := EncodeKitty(emu.KeyDisambiguateEscape, key.Event{Name: "A", State: key.Press}); ok {
		t.Fatal("expected ok=false for a plain unmodified letter (goes through EditEvent instead)")
	}
}

func TestEncodeKittyRelease(t *testing.T) {
	got, ok := EncodeKitty(emu.KeyDisambiguateEscape|emu.KeyReportEventTypes, key.Event{Name: key.NameEscape, State: key.Release})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[27;1:3u"); !bytes.Equal(got, want) {
		t.Fatalf("EncodeKitty(Escape release) = %q, want %q", got, want)
	}
}
