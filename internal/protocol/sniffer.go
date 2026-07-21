// Package protocol hosts pieces shared by geckty's protocol encoders and
// decoders that don't warrant their own subpackage — currently just
// Sniffer, used by internal/protocol/kittygfx.
package protocol

// maxAPCBuffer bounds how much an in-progress APC sequence can accumulate
// before Sniffer gives up on it and returns to ground state. Defensive
// cap: PTY output can come from a remote, untrusted program (e.g. over
// SSH), so an APC sequence that never terminates must not be allowed to
// grow Sniffer's buffer without bound.
const maxAPCBuffer = 256 << 20 // 256MiB of base64 text comfortably covers any real image.

// Sniffer extracts complete Application Program Command payloads — the
// bytes between "ESC _" and the terminating "ESC \" (ST) — from a raw PTY
// byte stream, for sequences vt.Terminal's vendored emu doesn't recognize
// at all (confirmed by the project's M0 spike: an unrecognized APC never
// reaches emu's grid-mutating calls, so this is safe to run in parallel).
// Feed it the exact same bytes vt.Terminal.Parse receives, unmodified —
// Sniffer only observes, it never filters or strips anything, so both
// consumers see identical input.
//
// Currently the only consumer is internal/protocol/kittygfx (Kitty
// graphics protocol), which owns interpreting the payload (checking for
// its 'G' marker byte, everything after that is protocol-specific).
type Sniffer struct {
	state sniffState
	buf   []byte
	onAPC func(payload []byte)
}

type sniffState int

const (
	stateGround sniffState = iota
	stateEscape
	stateAPC
	stateAPCEscape
)

// NewSniffer returns a Sniffer that calls onAPC, synchronously from Write,
// once for every complete APC sequence found in the fed bytes. onAPC's
// payload slice is reused by Sniffer after it returns — callers that need
// to retain it must copy.
func NewSniffer(onAPC func(payload []byte)) *Sniffer {
	return &Sniffer{onAPC: onAPC}
}

// Write feeds raw PTY bytes into the sniffer. Safe to call with arbitrarily
// split chunks (as PTY reads naturally are) — state persists across calls.
// Always returns (len(p), nil); it exists mainly so a Sniffer can be handed
// anywhere an io.Writer is expected.
func (s *Sniffer) Write(p []byte) (int, error) {
	for _, b := range p {
		s.step(b)
	}
	return len(p), nil
}

func (s *Sniffer) step(b byte) {
	switch s.state {
	case stateGround:
		if b == 0x1b {
			s.state = stateEscape
		}

	case stateEscape:
		switch b {
		case '_':
			s.state = stateAPC
			s.buf = s.buf[:0]
		case 0x1b:
			// A run of consecutive ESC bytes — stay put, the most
			// recent one is still the candidate APC introducer.
		default:
			s.state = stateGround
		}

	case stateAPC:
		if b == 0x1b {
			s.state = stateAPCEscape
			return
		}
		s.appendAPC(b)

	case stateAPCEscape:
		switch b {
		case '\\':
			if s.onAPC != nil {
				s.onAPC(s.buf)
			}
			s.buf = nil
			s.state = stateGround
		case 0x1b:
			// Another ESC immediately after one that didn't turn
			// out to be ST — the previous one is discarded (real
			// APC payloads, base64 text, never legitimately
			// contain ESC bytes) and this one becomes the new
			// candidate terminator.
		default:
			// Not a terminator after all: the ESC was itself
			// content, followed by this byte.
			s.appendAPC(0x1b)
			s.appendAPC(b)
			s.state = stateAPC
		}
	}
}

func (s *Sniffer) appendAPC(b byte) {
	if len(s.buf) >= maxAPCBuffer {
		// Runaway or malicious sequence that never terminates —
		// give up on it rather than growing without bound.
		s.buf = nil
		s.state = stateGround
		return
	}
	s.buf = append(s.buf, b)
}
