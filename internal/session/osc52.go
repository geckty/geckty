package session

import "sync"

// osc52Bridge implements emu.OSC52Handler.
//
// Write: emu calls OSC52Write synchronously from Parse, which runs on the
// PTY read goroutine — nowhere near gio's clipboard.WriteCmd, which can
// only be executed from the UI goroutine during frame processing (it needs
// a live layout.Context). So the write is stashed here instead of applied
// directly; the UI layer drains it via Session.TakeClipboardWrite on its
// next frame and performs the actual gtx.Execute(clipboard.WriteCmd{...}).
//
// Read (OSC 52 query — a program asking to read the OS clipboard through
// the terminal): disabled unconditionally, matching kitty's default.
// Letting arbitrary programs running in the terminal silently read the OS
// clipboard is a real exfiltration risk (e.g. a compromised `curl | bash`
// script reading a password you'd copied) — and even setting that aside,
// gio's clipboard read is asynchronous (clipboard.ReadCmd delivers a later
// transfer.DataEvent), which doesn't fit OSC52Read's synchronous contract
// without a second, more complex hand-off. Both reasons point the same
// way, so this isn't offered as a config toggle for now.
type osc52Bridge struct {
	mu      sync.Mutex
	pending []byte
}

func newOSC52Bridge() *osc52Bridge {
	return &osc52Bridge{}
}

// OSC52Write implements emu.OSC52Handler. pc is ignored — emu only ever
// calls this with pc == "c" (it filters everything else itself before
// dispatching, per handleOSC52).
//
// Does not call onDirty: OSC52Write only ever runs synchronously from
// within Term.Parse, which Session.Run always calls inside the same
// `n > 0` block that unconditionally calls onDirty afterward — an onDirty
// call here would just double-fire it for every OSC 52 write. Found while
// adding M8's Kitty-graphics handleAPC, which had the identical bug (see
// its doc comment in graphics.go) — this is the same fix applied here too.
func (b *osc52Bridge) OSC52Write(_ string, data []byte) {
	b.mu.Lock()
	b.pending = append([]byte(nil), data...)
	b.mu.Unlock()
}

// OSC52Read implements emu.OSC52Handler. Always declines — see the type
// doc comment.
func (b *osc52Bridge) OSC52Read(_ string) ([]byte, bool) {
	return nil, false
}

// take atomically takes and clears the pending clipboard write, if any.
func (b *osc52Bridge) take() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		return nil, false
	}
	data := b.pending
	b.pending = nil
	return data, true
}

// TakeClipboardWrite atomically takes and clears any pending OSC 52
// clipboard write the shell sent (see osc52Bridge), for the UI layer to
// hand off to the OS clipboard. Returns ok=false if there's nothing
// pending — the common case, checked once per frame.
func (s *Session) TakeClipboardWrite() ([]byte, bool) {
	return s.osc52.take()
}
