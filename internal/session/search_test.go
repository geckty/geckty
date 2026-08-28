package session

import (
	"testing"
)

func TestFindInScrollbackForward(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 16)
	s := newTestSession(p, 20, 3, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "hello world\r\nfoo BAR baz\r\n")

	hit, ok := s.FindInScrollback("bar", 0, 0, true)
	if !ok {
		t.Fatal("expected to find \"bar\"")
	}
	if hit.AbsLine != 1 || hit.Col != 4 || hit.Len != 3 {
		t.Fatalf("hit = %+v, want AbsLine=1 Col=4 Len=3", hit)
	}
}

func TestFindInScrollbackBackward(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 16)
	s := newTestSession(p, 20, 3, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "aaa\r\nbbb\r\naaa\r\n")

	hit, ok := s.FindInScrollback("aaa", 2, 3, false)
	if !ok {
		t.Fatal("expected a backward hit")
	}
	if hit.AbsLine != 2 || hit.Col != 0 {
		// beforeCol=3 on line "aaa" — last match starting before col 3 is col 0
		t.Fatalf("hit = %+v, want AbsLine=2 Col=0", hit)
	}
	hit, ok = s.FindInScrollback("aaa", 2, 0, false)
	if !ok || hit.AbsLine != 0 {
		t.Fatalf("second backward hit = %+v, %v, want AbsLine=0", hit, ok)
	}
}

func TestFindInScrollbackEmptyQuery(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	if _, ok := s.FindInScrollback("", 0, 0, true); ok {
		t.Fatal("empty query must not match")
	}
}

func TestCountInScrollback(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 16)
	s := newTestSession(p, 20, 3, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "xx xx\r\nxx\r\n")
	if n := s.CountInScrollback("xx"); n != 3 {
		t.Fatalf("CountInScrollback = %d, want 3", n)
	}
}

func TestScrollToAbsLine(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 32)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	for i := 0; i < 6; i++ {
		writeAndWaitDirty(t, p, dirty, "line"+string(rune('0'+i))+"\r\n")
	}
	hist := len(s.Term.History())
	if hist == 0 {
		t.Fatal("expected history")
	}
	off := s.ScrollToAbsLine(0)
	if off != hist {
		t.Fatalf("ScrollToAbsLine(0) offset = %d, want histLen %d (oldest at top)", off, hist)
	}
	if top := s.ViewportTopAbsLine(); top != 0 {
		t.Fatalf("ViewportTopAbsLine = %d, want 0", top)
	}
}
