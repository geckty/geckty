package vt

import (
	"bytes"
	"sync"
	"testing"
)

// M0 spike: confirm emu.Parse's behavior on an unrecognized APC sequence
// (used by the Kitty graphics protocol). cy/emu has no APC handler, so the
// underlying go-vte parser routes these bytes through its "SOS/PM/APC
// string" state, which discards every byte without invoking any Terminal
// callback (verified against go-vte v1.0.4's state table: every entry in
// that state is ignoreAction).
//
// This confirms geckty's sniffer design is safe: PTY bytes can be fed to
// emu.Parse unmodified while a second, independent scanner watches the same
// stream for APC/OSC52 payloads emu doesn't surface.
func TestParseIgnoresUnknownAPCSequence(t *testing.T) {
	term := New(10, 2, &bytes.Buffer{}, nil)

	before := term.Cell(0, 0)

	apc := []byte("\x1b_Gsome-kitty-graphics-payload-that-looks-like-a=1,f=100\x1b\\")
	n := term.Parse(apc)
	if n != len(apc) {
		t.Fatalf("Parse consumed %d of %d bytes", n, len(apc))
	}

	after := term.Cell(0, 0)
	if after != before {
		t.Fatalf("APC sequence mutated grid state: before=%+v after=%+v", before, after)
	}

	cursor := term.Cursor()
	if cursor.C != 0 || cursor.R != 0 {
		t.Fatalf("APC sequence moved the cursor: %+v", cursor)
	}

	term.Parse([]byte("hi"))
	if term.Cell(0, 0).Char != 'h' || term.Cell(1, 0).Char != 'i' {
		t.Fatalf("terminal did not resume normal parsing after APC: %q %q",
			term.Cell(0, 0).Char, term.Cell(1, 0).Char)
	}
}

// Terminal's RLock/RUnlock exist to give a reader (the UI painter) a
// consistent snapshot against a concurrent writer (the PTY read loop
// calling Parse). This drives both concurrently under -race to prove the
// locking actually prevents a race, not just that the API compiles —
// removing the mu.Lock()/mu.RLock() pairs in terminal.go should make this
// fail under `go test -race`.
func TestParseAndReadAreRaceFree(t *testing.T) {
	term := New(20, 5, &bytes.Buffer{}, nil)

	var wg sync.WaitGroup
	wg.Add(2)

	stop := make(chan struct{})

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			term.Parse([]byte("hello world\r\n"))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			term.RLock()
			_ = term.Cell(0, 0)
			_ = term.Cursor()
			_ = term.Size()
			_ = term.History()
			term.RUnlock()
		}
		close(stop)
	}()

	wg.Wait()
	t.Log("2000 concurrent reads completed against a continuously-parsing writer with no race detected")
}
