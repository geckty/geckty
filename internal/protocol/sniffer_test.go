package protocol

import (
	"bytes"
	"testing"
)

func collectAPC(seqs *[][]byte) func([]byte) {
	return func(payload []byte) {
		*seqs = append(*seqs, append([]byte(nil), payload...))
	}
}

func TestSnifferSingleSequenceOneWrite(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	_, _ = s.Write([]byte("hello\x1b_Gfoo\x1b\\world"))

	if len(got) != 1 {
		t.Fatalf("got %d sequences, want 1: %q", len(got), got)
	}
	if !bytes.Equal(got[0], []byte("Gfoo")) {
		t.Fatalf("payload = %q, want %q", got[0], "Gfoo")
	}
}

func TestSnifferSplitByteByByte(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	full := []byte("plain text\x1b_Gi=1,a=T;QUJD\x1b\\more text")
	for _, b := range full {
		_, _ = s.Write([]byte{b})
	}

	if len(got) != 1 {
		t.Fatalf("got %d sequences, want 1: %q", len(got), got)
	}
	if want := "Gi=1,a=T;QUJD"; string(got[0]) != want {
		t.Fatalf("payload = %q, want %q", got[0], want)
	}
}

func TestSnifferSplitAtArbitraryOffsets(t *testing.T) {
	full := "before\x1b_Gcontrol;payloaddata\x1b\\after"
	// Try every possible split point of the byte stream into two Write
	// calls, proving the state machine correctly resumes mid-sequence
	// regardless of exactly where a PTY read boundary falls.
	for split := 0; split <= len(full); split++ {
		var got [][]byte
		s := NewSniffer(collectAPC(&got))
		_, _ = s.Write([]byte(full[:split]))
		_, _ = s.Write([]byte(full[split:]))

		if len(got) != 1 {
			t.Fatalf("split=%d: got %d sequences, want 1", split, len(got))
		}
		if want := "Gcontrol;payloaddata"; string(got[0]) != want {
			t.Fatalf("split=%d: payload = %q, want %q", split, got[0], want)
		}
	}
}

func TestSnifferMultipleSequences(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	_, _ = s.Write([]byte("\x1b_Gfirst\x1b\\middle\x1b_Gsecond\x1b\\end"))

	if len(got) != 2 {
		t.Fatalf("got %d sequences, want 2: %q", len(got), got)
	}
	if string(got[0]) != "Gfirst" || string(got[1]) != "Gsecond" {
		t.Fatalf("payloads = %q, %q", got[0], got[1])
	}
}

func TestSnifferIgnoresNonAPCEscapes(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	// CSI (ESC [), OSC (ESC ]), and a lone ESC should never be mistaken
	// for an APC introducer.
	_, _ = s.Write([]byte("\x1b[31mred\x1b[0m \x1b]0;title\x07 \x1b done"))

	if len(got) != 0 {
		t.Fatalf("got %d sequences, want 0: %q", len(got), got)
	}
}

func TestSnifferIgnoresNonAPCThenFindsRealOne(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	_, _ = s.Write([]byte("\x1b[31mred\x1b_Greal\x1b\\\x1b[0m"))

	if len(got) != 1 {
		t.Fatalf("got %d sequences, want 1: %q", len(got), got)
	}
	if string(got[0]) != "Greal" {
		t.Fatalf("payload = %q, want %q", got[0], "Greal")
	}
}

func TestSnifferUnterminatedSequenceCallsNothing(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	_, _ = s.Write([]byte("\x1b_Gnever terminated"))

	if len(got) != 0 {
		t.Fatalf("got %d sequences from an unterminated APC, want 0: %q", len(got), got)
	}
}

func TestSnifferEscapeRunBeforeAPCIntroducer(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	// A run of ESC bytes followed by '_' should still be recognized as
	// starting an APC sequence (the most recent ESC is the candidate).
	_, _ = s.Write([]byte("\x1b\x1b\x1b_Gpayload\x1b\\"))

	if len(got) != 1 {
		t.Fatalf("got %d sequences, want 1: %q", len(got), got)
	}
	if string(got[0]) != "Gpayload" {
		t.Fatalf("payload = %q, want %q", got[0], "Gpayload")
	}
}

func TestSnifferEscapeRunInsideAPCBeforeTerminator(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	// Two ESCs in a row while looking for ST: the first is discarded,
	// the second is the real terminator candidate.
	_, _ = s.Write([]byte("\x1b_Gpayload\x1b\x1b\\"))

	if len(got) != 1 {
		t.Fatalf("got %d sequences, want 1: %q", len(got), got)
	}
	if string(got[0]) != "Gpayload" {
		t.Fatalf("payload = %q, want %q", got[0], "Gpayload")
	}
}

func TestSnifferBufferCapPreventsUnboundedGrowth(t *testing.T) {
	var got [][]byte
	s := NewSniffer(collectAPC(&got))

	_, _ = s.Write([]byte("\x1b_G"))
	chunk := bytes.Repeat([]byte("A"), 1<<20)
	for i := 0; i < (maxAPCBuffer/len(chunk))+2; i++ {
		_, _ = s.Write(chunk)
	}
	// The sequence never terminates and exceeds the cap, so it must be
	// abandoned (state reset) rather than growing forever. Confirm the
	// sniffer is usable afterward, not stuck.
	_, _ = s.Write([]byte("\x1b\\"))
	if len(got) != 0 {
		t.Fatalf("an abandoned oversized sequence must not be dispatched, got %d", len(got))
	}

	_, _ = s.Write([]byte("\x1b_Gfine\x1b\\"))
	if len(got) != 1 || string(got[0]) != "Gfine" {
		t.Fatalf("sniffer did not recover after an oversized sequence: got %q", got)
	}
}

func TestSnifferWriteReturnsFullLength(t *testing.T) {
	s := NewSniffer(nil)
	p := []byte("some bytes\x1b_Gx\x1b\\more")
	n, err := s.Write(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(p) {
		t.Fatalf("n = %d, want %d", n, len(p))
	}
}
