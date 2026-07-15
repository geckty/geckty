// Command geckty is a GUI terminal emulator.
package main

import (
	"context"
	"log/slog"
	"os"

	"gioui.org/app"

	"github.com/glaciforge/slogsafe"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/ui"
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

	// Run through the Backend interface, not the concrete ui.Run
	// function directly — main.go doesn't need to know the UI is
	// gio-based, only that something implements Backend. Today
	// ui.GioBackend is the only implementation; the point of routing
	// through the interface here, rather than just documenting it as a
	// theoretical future option, is that main.go is already written the
	// way it'd need to be to swap backends later.
	var backend ui.Backend = ui.GioBackend{}

	go func() {
		if err := backend.Run(cfg); err != nil {
			slog.ErrorContext(ctx, "geckty exited", slogsafe.Err(err))
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}
