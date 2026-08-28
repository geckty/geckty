package session

import (
	"sync"

	"github.com/geckty/geckty/internal/protocol"
	"github.com/geckty/geckty/internal/protocol/kittygfx"
)

// maxPlacements bounds how many decoded Kitty-graphics images a session
// keeps at once, evicting the oldest — a defensive cap against a runaway
// or malicious PTY stream (e.g. over SSH) placing unbounded images.
const maxPlacements = 64

// Placement is a decoded Kitty-graphics image anchored to a specific
// terminal line, ready for ui/grid to paint.
//
// AbsLine addresses the line chronologically within History()+Screen()
// (len(History()) at decode time, plus the cursor's row) — the same
// addressing ui/grid's viewport() already uses for scrollback — so the
// image scrolls with the surrounding text instead of staying pinned to a
// fixed on-screen row, and remains correctly positioned whether the view
// is live or scrolled back.
//
// Known limitation: emu prunes old scrollback once it exceeds its
// configured limit (see emu.WithHistoryLimit), which shifts every
// still-live line's index down by however many were dropped — a signal
// this package has no way to observe. A placement anchored to a line that
// existed before pruning started will drift out of sync (typically just
// disappearing, not corrupting anything) once enough output has scrolled
// past it. Not fixed for M8: real-world impact is bounded to very old
// images in a long-lived, high-output session, and a correct fix means
// patching emu again for a monotonic eviction counter (the same kind of
// patch M6 made for OSC 52) — out of scope for this milestone.
type Placement struct {
	kittygfx.Placement
	Seq     uint64
	AbsLine int
	Col     int
}

// graphics holds the Kitty-graphics decoding state for one Session: the
// APC sniffer, the protocol decoder it feeds, and the resulting
// placements. Kept as its own embeddable type to keep Session.newWithPTY
// from getting any more crowded.
type graphics struct {
	sniffer *protocol.Sniffer
	decoder *kittygfx.Decoder

	mu         sync.Mutex
	placements []Placement
	nextSeq    uint64
}

func newGraphics(s *Session) *graphics {
	g := &graphics{decoder: kittygfx.NewDecoder()}
	g.sniffer = protocol.NewSniffer(func(payload []byte) {
		s.handleAPC(payload)
	})
	return g
}

// handleAPC runs on the PTY read goroutine (called from Sniffer.Write
// inside Run, in parallel with Term.Parse — see internal/protocol.Sniffer)
// for every complete APC sequence found in the shell's output. It decodes
// Kitty-graphics commands, writes back any protocol response, and records
// completed placements.
func (s *Session) handleAPC(payload []byte) {
	result := s.gfx.decoder.Feed(payload)
	if result.Resp != nil {
		_, _ = s.Write(result.Resp)
	}
	if result.DeleteAll {
		s.clearPlacements()
		return
	}
	if result.DeleteID != 0 {
		s.deletePlacementID(result.DeleteID)
		return
	}
	if result.Placement == nil {
		return
	}
	placement := result.Placement

	s.Term.RLock()
	cursor := s.Term.Cursor()
	absLine := len(s.Term.History()) + cursor.R
	s.Term.RUnlock()

	s.gfx.mu.Lock()
	s.gfx.nextSeq++
	s.gfx.placements = append(s.gfx.placements, Placement{
		Placement: *placement,
		Seq:       s.gfx.nextSeq,
		AbsLine:   absLine,
		Col:       cursor.C,
	})
	if len(s.gfx.placements) > maxPlacements {
		s.gfx.placements = s.gfx.placements[len(s.gfx.placements)-maxPlacements:]
	}
	s.gfx.mu.Unlock()
	// No onDirty() call here: handleAPC only ever runs synchronously
	// inside Run()'s sniffer.Write call, which is itself inside the same
	// `n > 0` block that unconditionally calls onDirty afterward — a
	// second call here would just double-fire it for every decoded
	// placement (see the identical fix applied to osc52Bridge.OSC52Write
	// for the same reason).
}

// deletePlacementID drops placements whose Kitty image id matches id.
func (s *Session) deletePlacementID(id uint32) {
	s.gfx.mu.Lock()
	defer s.gfx.mu.Unlock()
	kept := s.gfx.placements[:0]
	for _, p := range s.gfx.placements {
		if p.ID != id {
			kept = append(kept, p)
		}
	}
	s.gfx.placements = kept
}

// Placements returns a snapshot of the session's currently decoded
// Kitty-graphics image placements, for ui/grid to paint.
func (s *Session) Placements() []Placement {
	s.gfx.mu.Lock()
	defer s.gfx.mu.Unlock()
	return append([]Placement(nil), s.gfx.placements...)
}

// clearPlacements drops all decoded image placements — called on alt-
// screen entry (see Run), mirroring ResetScroll's existing alt-screen-
// entry-only handling: full-screen apps (vim, less, htop) shouldn't see
// graphics left over from the main screen, and like scroll position,
// placements aren't tracked per-screen-buffer, only cleared on the one
// transition that matters in practice.
func (s *Session) clearPlacements() {
	s.gfx.mu.Lock()
	s.gfx.placements = nil
	s.gfx.mu.Unlock()
}
