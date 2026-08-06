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

func TestTrimURLTrailer(t *testing.T) {
	if got := trimURLTrailer("https://a.com/x)."); got != "https://a.com/x" {
		t.Fatalf("trimURLTrailer = %q", got)
	}
}
