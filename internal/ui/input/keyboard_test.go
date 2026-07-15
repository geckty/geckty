package input

import (
	"bytes"
	"testing"

	"gioui.org/io/key"
)

func TestEncodeNamedKeys(t *testing.T) {
	for name := range namedKeys {
		b, ok := Encode(key.Event{Name: name, State: key.Press})
		if !ok {
			t.Errorf("Encode(%q): ok=false", name)
			continue
		}
		if len(b) == 0 {
			t.Errorf("Encode(%q): empty output", name)
		}
	}
}

func TestEncodeIgnoresRelease(t *testing.T) {
	if _, ok := Encode(key.Event{Name: key.NameReturn, State: key.Release}); ok {
		t.Fatal("expected Release events to be ignored")
	}
}

func TestEncodeCtrlLetter(t *testing.T) {
	cases := map[key.Name]byte{
		"A": 1, "C": 3, "D": 4, "Z": 26,
	}
	for name, want := range cases {
		b, ok := Encode(key.Event{Name: name, State: key.Press, Modifiers: key.ModCtrl})
		if !ok || len(b) != 1 || b[0] != want {
			t.Errorf("Encode(Ctrl+%s) = %v, ok=%v; want [%d]", name, b, ok, want)
		}
	}
}

func TestEncodePlainLetterIsUnhandled(t *testing.T) {
	// Plain letters arrive as key.EditEvent, not key.Event — Encode must
	// not also produce output for them, or typed text would double up.
	if _, ok := Encode(key.Event{Name: "A", State: key.Press}); ok {
		t.Fatal("Encode should leave plain letter presses to EditEvent")
	}
}

func TestEncodeText(t *testing.T) {
	if got := EncodeText("hello"); !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("EncodeText = %q", got)
	}
}
