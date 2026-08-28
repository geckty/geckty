// Command geckty is a GUI terminal emulator.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/glaciforge/slogsafe"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/debug"
	"github.com/geckty/geckty/internal/logx"
	"github.com/geckty/geckty/internal/ui"
	"github.com/geckty/geckty/internal/ui/app"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "@" {
		if err := runRemote(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	logLevelFlag := flag.String("log-level", "", "log level: debug, info, warn, or error (overrides config's log_level)")
	listThemes := flag.Bool("list-themes", false, "list available theme names (user + embedded) and exit")
	flag.Parse()

	if *listThemes {
		printThemeList()
		return
	}

	log := setupLogger()
	debug.MaybeStartPprof()
	ctx := context.Background()
	cfg := mustLoadConfig(ctx, log)
	stopWatch := configureLogging(ctx, log, cfg, *logLevelFlag)

	var backend ui.Backend = app.Backend{}
	runErr := backend.Run(cfg)
	stopWatch()
	if runErr != nil {
		slog.ErrorContext(ctx, "geckty exited", slogsafe.Err(runErr))
		os.Exit(1)
	}
}

func printThemeList() {
	path, err := config.DefaultPath()
	if err != nil {
		path = ""
	}
	for _, name := range config.ListThemes(path) {
		fmt.Println(name)
	}
}

func setupLogger() *slogsafe.Logger {
	// Text format suits a desktop app; initial level is a placeholder until
	// config (or -log-level) is applied via SetLevel.
	return slogsafe.SetupDefault(
		slogsafe.WithFormat(slogsafe.FormatText),
		slogsafe.WithLevel(slog.LevelError),
	)
}

func mustLoadConfig(ctx context.Context, log *slogsafe.Logger) *config.Config {
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
	return cfg
}

func configureLogging(ctx context.Context, log *slogsafe.Logger, cfg *config.Config, logLevelFlag string) (stop func()) {
	logLevelSrc := cfg.LogLevel
	if logLevelFlag != "" {
		logLevelSrc = logLevelFlag
	}
	logLevel, err := config.ParseLogLevel(logLevelSrc)
	if err != nil {
		log.FatalContext(ctx, "parse log level", slogsafe.Err(err))
	}
	log.SetLevel(logLevel)
	ctx, _ = logx.Op(ctx, "geckty.main")

	if !cfg.HotReload {
		return func() {}
	}
	// -log-level still wins over log_level after hot-reload.
	return cfg.Watch(func(reloaded *config.Config, err error) {
		if err != nil {
			slog.WarnContext(ctx, "reload config", slogsafe.Err(err))
			return
		}
		if logLevelFlag != "" {
			return
		}
		lvl, err := config.ParseLogLevel(reloaded.LogLevel)
		if err != nil {
			slog.WarnContext(ctx, "reload config: invalid log_level, keeping previous", slogsafe.Err(err))
			return
		}
		log.SetLevel(lvl)
	})
}
