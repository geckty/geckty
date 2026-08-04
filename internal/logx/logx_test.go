package logx

import (
	"context"
	"log/slog"
	"testing"
)

func TestOpTagsLoggerAndContext(t *testing.T) {
	base := slog.Default()
	ctx := With(context.Background(), base)
	ctx2, log := Op(ctx, "pkg.Func", slog.String("tab", "1"))
	if log == nil {
		t.Fatal("Op returned nil logger")
	}
	if From(ctx2) != log {
		t.Fatal("From(ctx) should return the Op logger")
	}
}

func TestFromNilContext(t *testing.T) {
	// From documents nil-safety (same as With/Op); SA1012 would otherwise
	// forbid exercising that path.
	var ctx context.Context // intentionally nil
	//nolint:staticcheck // SA1012: nil ctx is the API under test
	if From(ctx) == nil {
		t.Fatal("From(nil) must not return nil")
	}
}

func TestWithNilContextAndLogger(t *testing.T) {
	var ctx context.Context
	//nolint:staticcheck // SA1012: With documents nil-safety
	got := With(ctx, nil)
	if From(got) == nil {
		t.Fatal("With(nil, nil) must still yield a usable logger via From")
	}
}

func TestFromBackgroundWithoutLogger(t *testing.T) {
	if From(context.Background()) == nil {
		t.Fatal("From(Background) must fall back to slog.Default")
	}
}

func TestOpNilContext(t *testing.T) {
	var ctx context.Context
	//nolint:staticcheck // SA1012: Op documents nil-safety
	ctx2, log := Op(ctx, "test.op")
	if log == nil || From(ctx2) != log {
		t.Fatal("Op(nil, ...) must return a tagged logger on a non-nil context")
	}
}
