package kittykbd

import (
	"bytes"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestEncodeLegacyModeReturnsNotOK(t *testing.T) {
	if _, ok := Encode(emu.KeyLegacy, Event{Key: KeyEscape, Pressed: true}); ok {
		t.Fatal("expected ok=false when no Kitty protocol flag is active")
	}
}

func TestEncodeWithoutDisambiguateReturnsNotOK(t *testing.T) {
	// KeyReportEventTypes alone, without KeyDisambiguateEscape, isn't
	// something this package acts on (see package doc scope).
	if _, ok := Encode(emu.KeyReportEventTypes, Event{Key: KeyEscape, Pressed: true}); ok {
		t.Fatal("expected ok=false without KeyDisambiguateEscape")
	}
}

func TestEncodeEscape(t *testing.T) {
	// Matches the spec's functional key table: ESCAPE = 27.
	got, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: KeyEscape, Pressed: true})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[27u"); !bytes.Equal(got, want) {
		t.Fatalf("Encode(Escape) = %q, want %q", got, want)
	}
}

func TestEncodeCtrlLetter(t *testing.T) {
	// Ctrl+A: modifier value = 1 + ctrl(4) = 5, code = lowercase 'a' = 97.
	got, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: "A", Modifiers: ModCtrl, Pressed: true})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[97;5u"); !bytes.Equal(got, want) {
		t.Fatalf("Encode(Ctrl+A) = %q, want %q", got, want)
	}
}

func TestEncodeCtrlShiftLetter(t *testing.T) {
	// Spec example: "Ctrl+Shift -> 1 + (4+1) = 6".
	got, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: "A", Modifiers: ModCtrl | ModShift, Pressed: true})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[97;6u"); !bytes.Equal(got, want) {
		t.Fatalf("Encode(Ctrl+Shift+A) = %q, want %q", got, want)
	}
}

func TestEncodePlainUnmodifiedLetterFallsThrough(t *testing.T) {
	// A plain letter (or shift-only) isn't ambiguous — it should keep
	// going through the normal text path (key.EditEvent), not get
	// CSI-u encoded, even with Disambiguate active.
	if _, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: "A", Pressed: true}); ok {
		t.Fatal("expected ok=false for a plain unmodified letter")
	}
	if _, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: "A", Modifiers: ModShift, Pressed: true}); ok {
		t.Fatal("expected ok=false for a shift-only letter (unambiguous, e.g. produces 'A' via EditEvent)")
	}
}

func TestEncodeFunctionalKeys(t *testing.T) {
	cases := []struct {
		key  Key
		want string
	}{
		{KeyLeftArrow, "\x1b[57417u"},
		{KeyRightArrow, "\x1b[57418u"},
		{KeyUpArrow, "\x1b[57419u"},
		{KeyDownArrow, "\x1b[57420u"},
		{KeyPageUp, "\x1b[57421u"},
		{KeyPageDown, "\x1b[57422u"},
		{KeyHome, "\x1b[57423u"},
		{KeyEnd, "\x1b[57424u"},
		{KeyDeleteForward, "\x1b[57426u"},
		{KeyF1, "\x1b[57376u"},
		{KeyF12, "\x1b[57387u"},
	}
	for _, c := range cases {
		got, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: c.key, Pressed: true})
		if !ok {
			t.Errorf("Encode(%s): ok=false", c.key)
			continue
		}
		if string(got) != c.want {
			t.Errorf("Encode(%s) = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestEncodeEnterTabBackspaceStayLegacyWhenUnmodified(t *testing.T) {
	cases := []struct {
		key  Key
		want byte
	}{
		{KeyReturn, '\r'},
		{KeyEnter, '\r'},
		{KeyTab, '\t'},
		{KeyDeleteBackward, 0x7f},
	}
	for _, c := range cases {
		got, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: c.key, Pressed: true})
		if !ok {
			t.Errorf("Encode(%s): ok=false", c.key)
			continue
		}
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("Encode(%s) = %v, want single byte %#x", c.key, got, c.want)
		}
	}
}

func TestEncodeEnterTabBackspaceNoLegacyReleaseEvent(t *testing.T) {
	// Per the spec: "Enter, Tab and Backspace keys will not have
	// release events unless Report all keys... is also set" (not
	// implemented — see package doc), so a release of these must not
	// produce anything at all under Disambiguate-only.
	for _, key := range []Key{KeyReturn, KeyEnter, KeyTab, KeyDeleteBackward} {
		if _, ok := Encode(emu.KeyDisambiguateEscape|emu.KeyReportEventTypes, Event{Key: key, Pressed: false}); ok {
			t.Errorf("Encode(%s release): expected ok=false", key)
		}
	}
}

func TestEncodeModifiedEnterIsDisambiguated(t *testing.T) {
	// A modified Enter (e.g. Ctrl+Enter) is ambiguous with plain Enter
	// under a purely legacy encoding, so it should get CSI-u encoded
	// even though the unmodified case stays legacy.
	got, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: KeyEnter, Modifiers: ModCtrl, Pressed: true})
	if !ok {
		t.Fatal("expected ok=true for Ctrl+Enter")
	}
	if want := []byte("\x1b[13;5u"); !bytes.Equal(got, want) {
		t.Fatalf("Encode(Ctrl+Enter) = %q, want %q", got, want)
	}
}

func TestEncodeReleaseEventMatchesSpecFormat(t *testing.T) {
	// Spec example: "CSI 97;1:3 u — release, no other mods (modifiers
	// field=1, event=3)".
	got, ok := Encode(emu.KeyDisambiguateEscape|emu.KeyReportEventTypes, Event{Key: "A", Modifiers: ModCtrl, Pressed: false})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[97;5:3u"); !bytes.Equal(got, want) {
		t.Fatalf("Encode(Ctrl+A release) = %q, want %q", got, want)
	}
}

func TestEncodeReleaseWithoutReportEventTypesFallsThrough(t *testing.T) {
	// Without KeyReportEventTypes, releases aren't reported at all —
	// fall through so the UI's OnKeyRelease path writes nothing.
	if _, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: "A", Modifiers: ModCtrl, Pressed: false}); ok {
		t.Fatal("expected ok=false for release without KeyReportEventTypes")
	}
	if _, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: KeyUpArrow, Pressed: false}); ok {
		t.Fatal("expected ok=false for arrow release without KeyReportEventTypes")
	}
}

func TestEncodePressOmitsEventFieldEvenWithReportEventTypes(t *testing.T) {
	// Press is the default event type and always omittable.
	got, ok := Encode(emu.KeyDisambiguateEscape|emu.KeyReportEventTypes, Event{Key: KeyEscape, Pressed: true})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[27u"); !bytes.Equal(got, want) {
		t.Fatalf("Encode(Escape press, event reporting on) = %q, want %q", got, want)
	}
}

func TestEncodeReleaseNoModifiersStillEmitsEventSubfield(t *testing.T) {
	// Even with no real modifiers, a release under KeyReportEventTypes
	// must still emit ";1:3" — the event-type subfield forces the
	// modifiers field to appear.
	got, ok := Encode(emu.KeyDisambiguateEscape|emu.KeyReportEventTypes, Event{Key: KeyEscape, Pressed: false})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := []byte("\x1b[27;1:3u"); !bytes.Equal(got, want) {
		t.Fatalf("Encode(Escape release) = %q, want %q", got, want)
	}
}

func TestEncodeUnknownSingleCharKeyWithoutModifiersFallsThrough(t *testing.T) {
	if _, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: "[", Pressed: true}); ok {
		t.Fatal("expected ok=false for an unmodified punctuation key")
	}
}

func TestEncodeMultiCharUnknownKeyFallsThrough(t *testing.T) {
	// A key.Name this package doesn't recognize at all (not a known
	// functional key, not a single rune) must not panic and must fall
	// through to the legacy encoder.
	if _, ok := Encode(emu.KeyDisambiguateEscape, Event{Key: "SomeUnknownKey", Modifiers: ModCtrl, Pressed: true}); ok {
		t.Fatal("expected ok=false for an unrecognized multi-char key name")
	}
}
