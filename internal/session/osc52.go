package session

import (
	"log/slog"
	"strings"
	"sync"
)

// ClipboardPolicy controls OSC 52 read/write behaviour (see config [clipboard]).
type ClipboardPolicy struct {
	WriteAllow bool
	ReadAllow  bool
	MaxSize    int // bytes; 0 means use defaultMaxOSC52
}

const defaultMaxOSC52 = 5 << 20

// osc52Bridge implements emu.OSC52Handler.
//
// Write: emu calls OSC52Write synchronously from Parse (PTY read goroutine).
// The payload is stashed; the UI drains via Session.TakeClipboardWrite.
//
// Read: denied unless ReadAllow is set (exfiltration risk; kitty default).
type osc52Bridge struct {
	mu      sync.Mutex
	pending []byte
	// clearPending is set when an empty OSC 52 write should clear the OS clipboard.
	clearPending bool
	policy       ClipboardPolicy
	log          *slog.Logger
}

func newOSC52Bridge(policy ClipboardPolicy, log *slog.Logger) *osc52Bridge {
	if policy.MaxSize <= 0 {
		policy.MaxSize = defaultMaxOSC52
	}
	if log == nil {
		log = slog.Default()
	}
	return &osc52Bridge{policy: policy, log: log}
}

// OSC52Write implements emu.OSC52Handler.
func (b *osc52Bridge) OSC52Write(_ string, data []byte) {
	const op = "session.OSC52Write"
	if !b.policy.WriteAllow {
		b.log.Debug(op, slog.String("result", "denied"))
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(data) == 0 {
		b.pending = nil
		b.clearPending = true
		return
	}
	if len(data) > b.policy.MaxSize {
		b.log.Warn(op, slog.String("result", "too_large"), slog.Int("bytes", len(data)), slog.Int("max", b.policy.MaxSize))
		return
	}
	b.clearPending = false
	b.pending = append([]byte(nil), data...)
}

// OSC52Read implements emu.OSC52Handler.
func (b *osc52Bridge) OSC52Read(_ string) ([]byte, bool) {
	if !b.policy.ReadAllow {
		return nil, false
	}
	// ReadAllow is reserved for a future UI-confirmed clipboard read path.
	// Until then, never return host clipboard data from the PTY goroutine.
	return nil, false
}

func (b *osc52Bridge) take() (data []byte, clear bool, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clearPending {
		b.clearPending = false
		b.pending = nil
		return nil, true, true
	}
	if b.pending == nil {
		return nil, false, false
	}
	data = b.pending
	b.pending = nil
	return data, false, true
}

// TakeClipboardWrite drains a pending OSC 52 write or clear for the UI.
// clear=true means the OS clipboard should be emptied; data may be empty.
func (s *Session) TakeClipboardWrite() (data []byte, clear bool, ok bool) {
	return s.osc52.take()
}

// ParseClipboardPolicy maps config strings to a ClipboardPolicy.
func ParseClipboardPolicy(write, read string, maxSize int) ClipboardPolicy {
	p := ClipboardPolicy{
		WriteAllow: !strings.EqualFold(strings.TrimSpace(write), "deny"),
		ReadAllow:  strings.EqualFold(strings.TrimSpace(read), "allow"),
		MaxSize:    maxSize,
	}
	if p.MaxSize <= 0 {
		p.MaxSize = defaultMaxOSC52
	}
	return p
}
