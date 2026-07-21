package session

import (
	"io"
	"testing"
	"time"
)

func TestSessionResize(t *testing.T) {
	p := newFakePTY()
	s := newWithPTY(p, 10, 2, nil, nil)
	defer func() { _ = s.Close() }()

	if err := s.Resize(20, 5); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	p.mu.Lock()
	cols, rows := p.cols, p.rows
	p.mu.Unlock()
	if cols != 20 || rows != 5 {
		t.Fatalf("PTY size after Resize = %d,%d, want 20,5", cols, rows)
	}
	if sz := s.Term.Size(); sz.C != 20 || sz.R != 5 {
		t.Fatalf("Term size after Resize = %+v, want C=20,R=5", sz)
	}
}

// TestTerminalRepliesRouteThroughSessionWrite exercises the writerFunc(s.Write)
// path installed in New/newWithPTY: when the terminal emulator parses a
// query sequence (e.g. DSR "\x1b[5n") from "shell output", its own reply is
// written back through the Session's guarded Write, not a raw io.Writer —
// this is what lets the shell see terminal-generated responses at all.
func TestTerminalRepliesRouteThroughSessionWrite(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newWithPTY(p, 10, 2, func() { dirty <- struct{}{} }, nil)
	defer func() { _ = s.Close() }()

	go s.Run()

	// The reply reader must be running before the DSR query is written:
	// the terminal's reply write (Term.Parse -> writerFunc(s.Write) ->
	// fakePTY.Write -> the fromSession io.Pipe) blocks until something
	// reads it, and Run's onDirty fire happens only after Parse returns
	// — so waiting for <-dirty first would deadlock against this same
	// unread pipe write.
	reply := make([]byte, 4)
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(p.fromSession, reply)
		done <- err
	}()

	if _, err := p.toSessionW.Write([]byte("\x1b[5n")); err != nil {
		t.Fatalf("write shell output: %v", err)
	}

	select {
	case <-dirty:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnDirty")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("reading terminal's DSR reply: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the terminal's DSR reply")
	}
	if string(reply) != "\x1b[0n" {
		t.Fatalf("DSR reply = %q, want \"\\x1b[0n\"", reply)
	}
}
