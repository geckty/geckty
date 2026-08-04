// Package logx wires structured slog logging through context with a stable
// "op" attribute — the same pattern used at geckty call boundaries so
// failures and debug traces show where work happened.
package logx

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// With attaches log to ctx so later From(ctx) retrieves it.
func With(ctx context.Context, log *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = slog.Default()
	}
	return context.WithValue(ctx, ctxKey{}, log)
}

// From returns the logger stored by With, or slog.Default().
func From(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if log, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && log != nil {
		return log
	}
	return slog.Default()
}

// Op returns a child logger tagged with slog.String("op", op) plus attrs,
// and a context carrying that logger.
func Op(ctx context.Context, op string, attrs ...any) (context.Context, *slog.Logger) {
	if ctx == nil {
		ctx = context.Background()
	}
	args := make([]any, 0, 1+len(attrs))
	args = append(args, slog.String("op", op))
	args = append(args, attrs...)
	log := From(ctx).With(args...)
	return With(ctx, log), log
}
