package app

import (
	"bytes"
	"testing"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestEncodeKittyLegacyModeFallsThrough(t *testing.T) {
	if _, ok := EncodeKitty(emu.KeyLegacy, gpucontext.KeyEscape, 0, true); ok {
		t.Fatal("expected ok=false when Kitty protocol is not active")
	}
}

func TestEncodeKittyEscape(t *testing.T) {
	got, ok := EncodeKitty(emu.KeyDisambiguateEscape, gpucontext.KeyEscape, 0, true)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[27u"); !bytes.Equal(got, want) {
		t.Fatalf("EncodeKitty(Escape) = %q, want %q", got, want)
	}
}

func TestEncodeKittyCtrlLetter(t *testing.T) {
	got, ok := EncodeKitty(emu.KeyDisambiguateEscape, gpucontext.KeyA, gpucontext.ModControl, true)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[97;5u"); !bytes.Equal(got, want) {
		t.Fatalf("EncodeKitty(Ctrl+A) = %q, want %q", got, want)
	}
}

func TestEncodeKittySuperMapsCorrectly(t *testing.T) {
	got, ok := EncodeKitty(emu.KeyDisambiguateEscape, gpucontext.KeyA, gpucontext.ModSuper|gpucontext.ModControl, true)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty encoding")
	}
}

func TestEncodeKittyPlainLetterFallsThrough(t *testing.T) {
	if _, ok := EncodeKitty(emu.KeyDisambiguateEscape, gpucontext.KeyA, 0, true); ok {
		t.Fatal("expected ok=false for a plain unmodified letter (goes through text input instead)")
	}
}
