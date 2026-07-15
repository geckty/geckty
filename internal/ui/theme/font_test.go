package theme

import (
	"runtime"
	"strings"
	"testing"
)

func TestDefaultMonospaceFamily(t *testing.T) {
	got := defaultMonospaceFamily()
	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(got, "Consolas") || !strings.Contains(got, "Cascadia") {
			t.Fatalf("windows default = %q, want Cascadia…/Consolas list", got)
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
