package ui

import (
	"sync"
	"testing"
	"time"
)

// TestResizeDebouncerCoalescesRapidTriggers: a drag burst collapses to
// leading + trailing apply (last size), not one apply per frame.
func TestResizeDebouncerCoalescesRapidTriggers(t *testing.T) {
	const delay = 20 * time.Millisecond

	var (
		mu    sync.Mutex
		calls []struct{ cols, rows int }
	)
	record := func(cols, rows int) {
		mu.Lock()
		calls = append(calls, struct{ cols, rows int }{cols, rows})
		mu.Unlock()
	}

	d := newResizeDebouncer(delay, record)

	// Simulate a burst of intermediate resize frames, each arriving
	// faster than the debounce delay — the same pattern a continuous
	// window-drag produces.
	for cols := 80; cols <= 120; cols += 5 {
		d.Trigger(cols, 24)
		time.Sleep(delay / 4)
	}

	// Give the debouncer's timer time to fire after the last Trigger.
	time.Sleep(delay * 3)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("apply was called %d times, want exactly 2 (leading + trailing, rapid triggers between them should coalesce): %v", len(calls), calls)
	}
	if got := calls[0]; got.cols != 80 || got.rows != 24 {
		t.Fatalf("leading apply(%d, %d), want the first triggered size (80, 24)", got.cols, got.rows)
	}
	if got := calls[1]; got.cols != 120 || got.rows != 24 {
		t.Fatalf("trailing apply(%d, %d), want the last triggered size (120, 24)", got.cols, got.rows)
	}
}

// TestResizeDebouncerFiresAfterSettling covers the leading edge: a single,
// isolated Trigger (not part of a rapid burst) applies right away rather
// than waiting out the debounce window — this is what fixes the
// truncation regression a trailing-only debounce caused (see
// resizeDebouncer's doc comment): the grid must not paint with a stale
// size for up to delay after every resize, only during an actual burst.
func TestResizeDebouncerFiresAfterSettling(t *testing.T) {
	const delay = 60 * time.Millisecond
	done := make(chan struct{}, 1)
	var got struct{ cols, rows int }

	d := newResizeDebouncer(delay, func(cols, rows int) {
		got.cols, got.rows = cols, rows
		done <- struct{}{}
	})

	d.Trigger(100, 40)

	select {
	case <-done:
	case <-time.After(delay / 2):
		t.Fatal("apply was never called well before the debounce window elapsed (leading edge should fire immediately)")
	}
	if got.cols != 100 || got.rows != 40 {
		t.Fatalf("apply(%d, %d), want (100, 40)", got.cols, got.rows)
	}

	// No further trigger arrived, so the trailing timer firing after
	// delay should be a no-op — confirm no second apply call shows up.
	select {
	case <-done:
		t.Fatal("apply was called a second time for a single, isolated Trigger")
	case <-time.After(delay * 2):
	}
}

// TestResizeDebouncerStopCancelsPendingTrailingApply covers Stop's actual
// contract under leading+trailing semantics: it can't cancel a leading
// apply (Trigger already dispatched it before Stop could ever run), but it
// must cancel the trailing catch-up for whatever's left of an in-progress
// burst.
func TestResizeDebouncerStopCancelsPendingTrailingApply(t *testing.T) {
	const delay = 15 * time.Millisecond
	var (
		mu    sync.Mutex
		calls []struct{ cols, rows int }
	)
	record := func(cols, rows int) {
		mu.Lock()
		calls = append(calls, struct{ cols, rows int }{cols, rows})
		mu.Unlock()
	}

	d := newResizeDebouncer(delay, record)
	d.Trigger(80, 24) // leading edge: applies right away
	time.Sleep(delay / 3)
	d.Trigger(90, 30) // still mid-burst: only marks the trailing target dirty
	d.Stop()          // cancel that trailing catch-up before its timer fires

	time.Sleep(delay * 3)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("apply was called %d times, want exactly 1 (leading only; Stop should cancel the trailing catch-up): %v", len(calls), calls)
	}
	if calls[0].cols != 80 || calls[0].rows != 24 {
		t.Fatalf("apply(%d, %d), want the leading trigger's size (80, 24)", calls[0].cols, calls[0].rows)
	}
}

func TestResizeDebouncerSeparateTriggersEachApply(t *testing.T) {
	const delay = 15 * time.Millisecond
	var (
		mu    sync.Mutex
		calls int
	)
	d := newResizeDebouncer(delay, func(int, int) {
		mu.Lock()
		calls++
		mu.Unlock()
	})

	d.Trigger(80, 24)
	time.Sleep(delay * 3)
	d.Trigger(90, 30)
	time.Sleep(delay * 3)

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (two separate, well-spaced resizes should each apply)", calls)
	}
}
