package gogpu

import (
	"sync"
	"testing"
	"time"
)

type resizeCall struct{ cols, rows int }

func newRecordingDebouncer(delay time.Duration) (*resizeDebouncer, func() []resizeCall) {
	var mu sync.Mutex
	var calls []resizeCall
	d := newResizeDebouncer(delay, func(cols, rows int) {
		mu.Lock()
		calls = append(calls, resizeCall{cols, rows})
		mu.Unlock()
	})
	get := func() []resizeCall {
		mu.Lock()
		defer mu.Unlock()
		out := make([]resizeCall, len(calls))
		copy(out, calls)
		return out
	}
	return d, get
}

func waitForCalls(t *testing.T, get func() []resizeCall, n int) []resizeCall {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if calls := get(); len(calls) >= n {
			return calls
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d apply call(s), got %d", n, len(get()))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestResizeDebouncerSingleTriggerFiresOnce(t *testing.T) {
	d, calls := newRecordingDebouncer(30 * time.Millisecond)
	defer d.Stop()

	d.Trigger(80, 24)
	got := waitForCalls(t, calls, 1)
	if got[0] != (resizeCall{80, 24}) {
		t.Fatalf("apply call = %+v, want {80,24}", got[0])
	}

	// An isolated trigger shouldn't produce a second, redundant trailing
	// apply of the same size once the debounce delay elapses.
	time.Sleep(60 * time.Millisecond)
	if got := calls(); len(got) != 1 {
		t.Fatalf("apply called %d time(s), want 1", len(got))
	}
}

func TestResizeDebouncerBurstFiresLeadingAndTrailing(t *testing.T) {
	d, calls := newRecordingDebouncer(40 * time.Millisecond)
	defer d.Stop()

	d.Trigger(80, 24)  // leading edge — applies immediately
	d.Trigger(90, 30)  // mid-burst — only updates the target
	d.Trigger(100, 40) // only the last size in the burst should matter

	got := waitForCalls(t, calls, 2)
	if got[0] != (resizeCall{80, 24}) {
		t.Fatalf("leading apply = %+v, want {80,24}", got[0])
	}
	if got[1] != (resizeCall{100, 40}) {
		t.Fatalf("trailing apply = %+v, want {100,40} (the burst's last size)", got[1])
	}
}

func TestResizeDebouncerStopSuppressesPendingTrailing(t *testing.T) {
	d, calls := newRecordingDebouncer(30 * time.Millisecond)
	d.Trigger(80, 24)
	waitForCalls(t, calls, 1)
	d.Trigger(90, 30)
	d.Stop()

	time.Sleep(60 * time.Millisecond)
	if got := calls(); len(got) != 1 {
		t.Fatalf("apply called %d time(s) after Stop, want 1 (trailing should be suppressed)", len(got))
	}
}
