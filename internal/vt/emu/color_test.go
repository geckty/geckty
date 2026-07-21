package emu

import "testing"

func TestXTermColor(t *testing.T) {
	c := XTermColor(200)
	if c.Default() {
		t.Fatal("XTermColor should not be Default")
	}
	got, ok := c.XTerm()
	if !ok || got != 200 {
		t.Fatalf("XTerm() = %d,%v, want 200,true", got, ok)
	}
}

func TestRGBColor(t *testing.T) {
	c := RGBColor(10, 20, 30)
	if c.Default() {
		t.Fatal("RGBColor should not be Default")
	}
	r, g, b, ok := c.RGB()
	if !ok || r != 10 || g != 20 || b != 30 {
		t.Fatalf("RGB() = %d,%d,%d,%v, want 10,20,30,true", r, g, b, ok)
	}
}

func TestColorDefault(t *testing.T) {
	if !DefaultFG.Default() {
		t.Fatal("DefaultFG should be Default")
	}
	if !DefaultBG.Default() {
		t.Fatal("DefaultBG should be Default")
	}
	if Red.Default() {
		t.Fatal("an ANSI color should not be Default")
	}
}

func TestColorRGBOnDefaultReturnsZeroOK(t *testing.T) {
	r, g, b, ok := DefaultFG.RGB()
	if ok || r != 0 || g != 0 || b != 0 {
		t.Fatalf("RGB() on default color = %d,%d,%d,%v, want 0,0,0,false", r, g, b, ok)
	}
}

func TestColorANSI(t *testing.T) {
	got, ok := Red.ANSI()
	if !ok || got != 1 {
		t.Fatalf("Red.ANSI() = %d,%v, want 1,true", got, ok)
	}

	if _, ok := DefaultFG.ANSI(); ok {
		t.Fatal("DefaultFG.ANSI() should not be ok")
	}

	if _, ok := XTermColor(200).ANSI(); ok {
		t.Fatal("a color >= 16 should not report ok from ANSI()")
	}

	if _, ok := RGBColor(1, 2, 3).ANSI(); ok {
		t.Fatal("an RGB color should not report ok from ANSI()")
	}
}

func TestColorXTerm(t *testing.T) {
	got, ok := XTermColor(200).XTerm()
	if !ok || got != 200 {
		t.Fatalf("XTerm() = %d,%v, want 200,true", got, ok)
	}

	if _, ok := DefaultFG.XTerm(); ok {
		t.Fatal("DefaultFG.XTerm() should not be ok")
	}

	if _, ok := RGBColor(1, 2, 3).XTerm(); ok {
		t.Fatal("an RGB color should not report ok from XTerm()")
	}
}
