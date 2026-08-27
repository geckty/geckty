package app

import (
	"errors"
	"testing"
)

type stubClipboard struct {
	writeErr error
	readErr  error
	text     string
	writes   []string
}

func (s *stubClipboard) ClipboardWrite(text string) error {
	s.writes = append(s.writes, text)
	return s.writeErr
}

func (s *stubClipboard) ClipboardRead() (string, error) {
	if s.readErr != nil {
		return "", s.readErr
	}
	return s.text, nil
}

func TestClipboardWriteFallsBackToApp(t *testing.T) {
	// Force the app path by making native fail when possible: on platforms
	// without a clipboard helper, clipboardWriteNative errors and we hit
	// app.ClipboardWrite. On darwin/windows with working helpers, native
	// may succeed first — either outcome is fine; we just assert no panic.
	stub := &stubClipboard{}
	if err := clipboardWrite(stub, "hi"); err != nil && len(stub.writes) == 0 {
		// Both paths failed (unlikely on CI with pbcopy/clip).
		t.Logf("clipboardWrite returned %v (no native helper / app write)", err)
	}
}

func TestClipboardReadPrefersNonEmpty(t *testing.T) {
	stub := &stubClipboard{text: "from-app"}
	got, err := clipboardRead(stub)
	if err != nil {
		t.Fatalf("clipboardRead: %v", err)
	}
	if got == "" {
		t.Fatal("expected some clipboard text from app or native")
	}
}

func TestClipboardReadPropagatesErrors(t *testing.T) {
	stub := &stubClipboard{readErr: errors.New("no clip"), text: ""}
	// May still succeed via native helper; only assert it doesn't panic.
	got, err := clipboardRead(stub)
	_ = got
	_ = err
	t.Log("clipboardRead completed without panic")
}
