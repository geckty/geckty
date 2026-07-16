// Command geckty is a GUI terminal emulator.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/glaciforge/slogsafe"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/ui"
	"github.com/geckty/geckty/internal/ui/gogpu"
)

func main() {
	// Dev() (text format, debug level) rather than Default() (JSON):
	// geckty is desktop software a user typically launches by double-
	// clicking, not a server — when its log output does get seen (run
	// from a terminal to debug something, or piped to a file), a human-
	// readable text format serves that better than structured JSON aimed
	// at a log aggregator. This is the only place in geckty that
	// constructs a *slogsafe.Logger directly (for FatalContext, which
	// isn't part of stdlib log/slog); everywhere else in the codebase
	// uses the standard log/slog package-level functions, which
	// SetupDefault wires to this same logger via slog.SetDefault.
	log := slogsafe.SetupDefault(
		slogsafe.WithFormat(slogsafe.FormatText),
		slogsafe.WithLevel(slog.LevelDebug),
	)
	ctx := context.Background()

	path, err := config.DefaultPath()
	if err != nil {
		log.FatalContext(ctx, "resolve config path", slogsafe.Err(err))
	}
	if err := config.EnsureDefaultFile(path); err != nil {
		slog.WarnContext(ctx, "write default config", slogsafe.Err(err))
	}
	cfg, err := config.Load(path)
	if err != nil {
		log.FatalContext(ctx, "load config", slogsafe.Err(err))
	}

	// Run through the Backend interface, not the concrete Run function
	// directly — main.go doesn't need to know the UI toolkit, only that
	// something implements Backend. gogpu.Backend replaces the former
	// gio-based ui.GioBackend (see internal/ui/gogpu's package doc for
	// why: a reproducible gio D3D11 rendering bug on Windows). Unlike gio,
	// gogpu's App.Run is itself the platform event pump, so it's called
	// directly on the main goroutine rather than via a background
	// goroutine plus a separate app.Main() call.
	var backend ui.Backend = gogpu.Backend{}

	if err := backend.Run(cfg); err != nil {
		slog.ErrorContext(ctx, "geckty exited", slogsafe.Err(err))
		os.Exit(1)
	}
}
