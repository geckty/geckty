package gogpu

import (
	"sync"
	"time"
)

// resizeDebounceDelay absorbs continuous window-drag resize events without
// feeling laggy once the drag stops.
const resizeDebounceDelay = 120 * time.Millisecond

// resizeDebouncer coalesces rapid size changes with leading+trailing
// applies: first Trigger in a burst runs apply immediately; later ones
// only update the target; after delay with no further Triggers, the
// newest size is applied if it differs. PTY/emu resize is expensive
// (scrollback dewrap), so per-frame apply freezes the UI with many tabs.
type resizeDebouncer struct {
	delay time.Duration
	apply func(cols, rows int)

	applyMu sync.Mutex // serializes leading/trailing apply

	mu          sync.Mutex
	timer       *time.Timer
	cols, rows  int
	burstActive bool // leading apply fired; trailing not yet
	dirty       bool // size changed since leading apply
}

func newResizeDebouncer(delay time.Duration, apply func(cols, rows int)) *resizeDebouncer {
	return &resizeDebouncer{delay: delay, apply: apply}
}

// Trigger records a new target size (called from the draw callback per frame).
func (d *resizeDebouncer) Trigger(cols, rows int) {
	d.mu.Lock()
	d.cols, d.rows = cols, rows
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, d.trailingFire)

	if d.burstActive {
		d.dirty = true
		d.mu.Unlock()
		return
	}
	d.burstActive = true
	d.dirty = false
	cols, rows = d.cols, d.rows
	d.mu.Unlock()

	go d.callApply(cols, rows)
}

// trailingFire runs delay after the last Trigger in a burst. It only
// calls apply if the burst produced a size beyond the one the leading
// edge already applied — an isolated single Trigger, the common case
// outside of a drag, otherwise ends up calling apply twice with the
// identical size for no benefit.
func (d *resizeDebouncer) trailingFire() {
	d.mu.Lock()
	cols, rows := d.cols, d.rows
	dirty := d.dirty
	d.burstActive = false
	d.dirty = false
	d.mu.Unlock()

	if dirty {
		d.callApply(cols, rows)
	}
}

func (d *resizeDebouncer) callApply(cols, rows int) {
	d.applyMu.Lock()
	defer d.applyMu.Unlock()
	d.apply(cols, rows)
}

// Stop cancels any pending trailing apply, without invoking it — used on
// window shutdown so a resize scheduled just before close doesn't fire
// against tabs that are already being torn down. It cannot cancel a
// leading apply that has already been dispatched (see Trigger) — only the
// trailing catch-up, which by definition hasn't run yet.
func (d *resizeDebouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.burstActive = false
	d.dirty = false
}
