package theme

import (
	"runtime"
	"testing"
)

func TestDefaultMonospaceFamily(t *testing.T) {
	got := defaultMonospaceFamily()
	switch runtime.GOOS {
	case "windows":
		if got != "Consolas" {
			t.Fatalf("windows default = %q, want Consolas", got)
		}
	case "darwin":
		if got != "Menlo" {
			t.Fatalf("darwin default = %q, want Menlo", got)
		}
	default:
		if got != "monospace" {
			t.Fatalf("default = %q, want monospace", got)
		}
	}
}

func TestSetMonospaceFamily(t *testing.T) {
	prev := MonospaceTypeface
	t.Cleanup(func() { MonospaceTypeface = prev })

	SetMonospaceFamily("Courier New")
	if string(MonospaceTypeface) != "Courier New" {
		t.Fatalf("SetMonospaceFamily(Courier New) = %q", MonospaceTypeface)
	}
	SetMonospaceFamily("monospace")
	if string(MonospaceTypeface) != defaultMonospaceFamily() {
		t.Fatalf("SetMonospaceFamily(monospace) = %q, want platform default %q",
			MonospaceTypeface, defaultMonospaceFamily())
	}
	SetMonospaceFamily("")
	if string(MonospaceTypeface) != defaultMonospaceFamily() {
		t.Fatalf("SetMonospaceFamily(\"\") = %q, want platform default", MonospaceTypeface)
	}
}
