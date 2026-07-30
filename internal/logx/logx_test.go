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
	if From(nil) == nil {
		t.Fatal("From(nil) must not return nil")
	}
}
