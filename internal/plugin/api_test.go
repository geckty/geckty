package plugin

import (
	"context"
	"testing"
)

// TestWriteToGuestRoundTrip proves the host-to-guest half of the ABI
// (malloc-based buffer passing — see writeToGuest's doc comment) actually
// works against a real compiled guest, even though no M9 hook uses it
// yet: the fixture's echo export reads back whatever the host wrote into
// its memory and reports it through statusbar_draw, the same observable
// channel other tests use.
func TestWriteToGuestRoundTrip(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := newFixtureDir(t, "echo", []string{"log", "statusbar"})

	p, err := h.Load(ctx, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ptr, ok := writeToGuest(ctx, p.mod, "hello from the host")
	if !ok {
		t.Fatal("writeToGuest: expected ok=true")
	}

	echoFn := p.mod.ExportedFunction("echo")
	if echoFn == nil {
		t.Fatal("fixture doesn't export echo")
	}
	if _, err := echoFn.Call(ctx, uint64(ptr), uint64(len("hello from the host"))); err != nil {
		t.Fatalf("calling echo: %v", err)
	}

	if got, want := p.StatusText(), "echo:hello from the host"; got != want {
		t.Fatalf("StatusText = %q, want %q", got, want)
	}
}

func TestWriteToGuestEmptyString(t *testing.T) {
	ctx := context.Background()
	h := newHost(t)
	dir := newFixtureDir(t, "echo-empty", []string{"log", "statusbar"})

	p, err := h.Load(ctx, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ptr, ok := writeToGuest(ctx, p.mod, "")
	if !ok {
		t.Fatal("writeToGuest: expected ok=true for an empty string")
	}

	echoFn := p.mod.ExportedFunction("echo")
	if _, err := echoFn.Call(ctx, uint64(ptr), 0); err != nil {
		t.Fatalf("calling echo: %v", err)
	}
	if got, want := p.StatusText(), "echo:"; got != want {
		t.Fatalf("StatusText = %q, want %q", got, want)
	}
}
