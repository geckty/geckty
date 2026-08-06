package session

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

// rgbaAPC builds a complete direct-transmission Kitty-graphics APC escape
// sequence for a solid-color w x h RGBA image.
func rgbaAPC(id, w, h int, r, g, b byte) string {
	pix := make([]byte, 0, w*h*4)
	for i := 0; i < w*h; i++ {
		pix = append(pix, r, g, b, 255)
	}
	payload := base64.StdEncoding.EncodeToString(pix)
	return "\x1b_Ga=T,f=32,s=" + strconv.Itoa(w) + ",v=" + strconv.Itoa(h) + ",i=" + strconv.Itoa(id) + ";" + payload + "\x1b\\"
}

// drainWrites continuously discards whatever the session writes back (e.g.
// a Kitty-graphics OK/error response) so tests that don't assert on the
// response bytes don't deadlock on Session.Write's blocking io.Pipe (which
// only unblocks once something reads it).
func drainWrites(p *fakePTY) {
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := p.fromSession.Read(buf); err != nil {
				return
			}
		}
	}()
}

func TestSessionDecodesKittyGraphicsAndRecordsPlacement(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 5, func() { dirty <- struct{}{} })
	go s.Run()
	drainWrites(p)
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, rgbaAPC(1, 2, 2, 10, 20, 30))

	placements := s.Placements()
	if len(placements) != 1 {
		t.Fatalf("got %d placements, want 1", len(placements))
	}
	pl := placements[0]
	if pl.ID != 1 {
		t.Fatalf("ID = %d, want 1", pl.ID)
	}
	bounds := pl.Image.Bounds()
	if bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Fatalf("image size = %v, want 2x2", bounds)
	}
	// Fresh terminal: cursor starts at (0,0), no history yet.
	if pl.AbsLine != 0 || pl.Col != 0 {
		t.Fatalf("AbsLine,Col = %d,%d, want 0,0", pl.AbsLine, pl.Col)
	}
	if pl.Seq == 0 {
		t.Fatal("expected a non-zero Seq")
	}
}

func TestSessionGraphicsResponseIsWrittenBack(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 5, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	resp := readAvailable(p.fromSession)
	writeAndWaitDirty(t, p, dirty, rgbaAPC(9, 1, 1, 1, 2, 3))

	select {
	case b := <-resp:
		if want := "\x1b_Gi=9;OK\x1b\\"; string(b) != want {
			t.Fatalf("response = %q, want %q", b, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the graphics OK response")
	}
}

func TestSessionGraphicsAnchorsToCursorPosition(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 20, 5, func() { dirty <- struct{}{} })
	go s.Run()
	drainWrites(p)
	defer func() { _ = s.Close() }()

	// Move the cursor to row 2, col 3 (1-based CUP: row 3, col 4) before
	// sending the graphics command.
	writeAndWaitDirty(t, p, dirty, "\x1b[3;4H"+rgbaAPC(1, 1, 1, 5, 5, 5))

	placements := s.Placements()
	if len(placements) != 1 {
		t.Fatalf("got %d placements, want 1", len(placements))
	}
	if placements[0].AbsLine != 2 || placements[0].Col != 3 {
		t.Fatalf("AbsLine,Col = %d,%d, want 2,3", placements[0].AbsLine, placements[0].Col)
	}
}

func TestSessionGraphicsUnsupportedCommandGetsErrorNoPlacement(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 5, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	resp := readAvailable(p.fromSession)
	writeAndWaitDirty(t, p, dirty, "\x1b_Ga=p,i=1;\x1b\\")

	select {
	case b := <-resp:
		if !bytes.Contains(b, []byte("EINVAL")) {
			t.Fatalf("response = %q, want an EINVAL error", b)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the error response")
	}
	if placements := s.Placements(); len(placements) != 0 {
		t.Fatalf("expected no placements for an unsupported command, got %d", len(placements))
	}
}

func TestSessionGraphicsClearedOnAltScreenEntry(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 5, func() { dirty <- struct{}{} })
	go s.Run()
	drainWrites(p)
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, rgbaAPC(1, 1, 1, 1, 1, 1))
	if len(s.Placements()) != 1 {
		t.Fatal("expected 1 placement before entering alt screen")
	}

	writeAndWaitDirty(t, p, dirty, "\x1b[?1049h")

	if got := s.Placements(); len(got) != 0 {
		t.Fatalf("expected placements cleared on alt-screen entry, got %d", len(got))
	}
}

func TestSessionGraphicsDeleteByID(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 5, func() { dirty <- struct{}{} })
	go s.Run()
	drainWrites(p)
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, rgbaAPC(7, 1, 1, 1, 1, 1))
	writeAndWaitDirty(t, p, dirty, rgbaAPC(8, 1, 1, 2, 2, 2))
	if len(s.Placements()) != 2 {
		t.Fatalf("got %d placements, want 2", len(s.Placements()))
	}
	writeAndWaitDirty(t, p, dirty, "\x1b_Ga=d,d=i,i=7;\x1b\\")
	got := s.Placements()
	if len(got) != 1 || got[0].ID != 8 {
		t.Fatalf("after delete id=7: %+v", got)
	}
	writeAndWaitDirty(t, p, dirty, "\x1b_Ga=d;\x1b\\")
	if len(s.Placements()) != 0 {
		t.Fatalf("after delete all: %d placements", len(s.Placements()))
	}
}

func TestSessionGraphicsBoundedPlacementCount(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 256)
	s := newTestSession(p, 10, 5, func() { dirty <- struct{}{} })
	go s.Run()
	drainWrites(p)
	defer func() { _ = s.Close() }()

	for i := 0; i < maxPlacements+10; i++ {
		writeAndWaitDirty(t, p, dirty, rgbaAPC(i+1, 1, 1, 1, 1, 1))
	}

	placements := s.Placements()
	if len(placements) != maxPlacements {
		t.Fatalf("got %d placements, want capped at %d", len(placements), maxPlacements)
	}
	// The oldest placements should have been evicted, so the first one
	// remaining should not be id 1.
	if placements[0].ID == 1 {
		t.Fatal("expected the oldest placement to have been evicted")
	}
}
