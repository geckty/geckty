package session

import "testing"

func TestURLAtHTTP(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 40, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "see https://example.com/path ok")
	url, ok := s.URLAt(0, 10) // inside the URL
	if !ok || url != "https://example.com/path" {
		t.Fatalf("URLAt = %q, %v, want https://example.com/path", url, ok)
	}
	if _, ok := s.URLAt(0, 0); ok {
		t.Fatal("URLAt on non-URL text should be ok=false")
	}
}

func TestURLAtWWWPrefixed(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 40, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "www.example.org/x.")
	url, ok := s.URLAt(0, 0)
	if !ok || url != "https://www.example.org/x" {
		t.Fatalf("URLAt = %q, %v, want https://www.example.org/x", url, ok)
	}
}

func TestURLAtOSC8Hyperlink(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 40, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "\x1b]8;;https://osc8.example/x\x1b\\link\x1b]8;;\x1b\\")
	url, ok := s.URLAt(0, 1)
	if !ok || url != "https://osc8.example/x" {
		t.Fatalf("URLAt OSC8 = %q, %v, want https://osc8.example/x", url, ok)
	}
}
