package emu

import (
	"bytes"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu/geom"
)

func TestOSC8SetsHyperlinkOnCells(t *testing.T) {
	var buf bytes.Buffer
	term := New(WithSize(geom.Vec2{C: 40, R: 3}), WithWriter(&buf))
	term.Parse([]byte("\x1b]8;;https://example.com/path\x1b\\click\x1b]8;;\x1b\\ me"))

	for col := 0; col < 5; col++ { // "click"
		g := term.Cell(col, 0)
		if g.Hyperlink != "https://example.com/path" {
			t.Fatalf("cell %d Hyperlink = %q, want https://example.com/path", col, g.Hyperlink)
		}
	}
	if term.Cell(5, 0).Hyperlink != "" {
		t.Fatalf("space after link end should have empty Hyperlink, got %q", term.Cell(5, 0).Hyperlink)
	}
	if g := term.Cell(6, 0); g.Hyperlink != "" {
		t.Fatalf("cell after OSC 8 end Hyperlink = %q, want empty", g.Hyperlink)
	}
}

func TestOSC8WithIDParam(t *testing.T) {
	var buf bytes.Buffer
	term := New(WithSize(geom.Vec2{C: 40, R: 2}), WithWriter(&buf))
	term.Parse([]byte("\x1b]8;id=1;https://a.test\x1b\\ab\x1b]8;;\x1b\\"))
	if got := term.Cell(0, 0).Hyperlink; got != "https://a.test" {
		t.Fatalf("Hyperlink = %q, want https://a.test", got)
	}
	if got := term.Cell(1, 0).Hyperlink; got != "https://a.test" {
		t.Fatalf("Hyperlink = %q, want https://a.test", got)
	}
}

func TestOSC8URIWithSemicolon(t *testing.T) {
	var buf bytes.Buffer
	term := New(WithSize(geom.Vec2{C: 80, R: 2}), WithWriter(&buf))
	term.Parse([]byte("\x1b]8;;https://ex.com/a;b=1\x1b\\x\x1b]8;;\x1b\\"))
	if got := term.Cell(0, 0).Hyperlink; got != "https://ex.com/a;b=1" {
		t.Fatalf("Hyperlink = %q, want URI with semicolon preserved", got)
	}
}
