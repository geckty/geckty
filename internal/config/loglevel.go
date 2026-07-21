package config

import (
	"fmt"
	"log/slog"
	"strings"
)

// ParseLogLevel converts a log_level config value (or the -log-level flag,
// see cmd/geckty/main.go) into a slog.Level. Matching is case-insensitive;
// an empty string is rejected rather than silently defaulting, so callers
// fall back to Default().LogLevel explicitly instead of masking a typo'd
// config value.
func ParseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q (want debug, info, warn, or error)", s)
	}
}
