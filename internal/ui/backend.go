package ui

import "github.com/geckty/geckty/internal/config"

// Backend runs geckty's UI for a given config, blocking until the window
// closes. GioBackend, which just delegates to this package's existing Run
// function, is the only implementation today — the interface exists so a
// future alternative backend (a headless renderer for testing, a
// different UI toolkit) could be swapped in at cmd/geckty/main.go without
// that call site needing to know it's gio specifically.
//
// Deliberately thin: this wraps the existing Run function as-is rather
// than splitting internal/ui's gio-specific internals (grid rendering,
// input handling, chrome) behind their own interfaces. That would be a
// much larger, currently unmotivated restructuring — nothing in geckty
// needs a second backend today, so there's nothing to design this
// interface's shape against yet beyond "runs, given a config, blocking
// until done."
type Backend interface {
	Run(cfg *config.Config) error
}

// GioBackend is Backend's gio-based implementation.
type GioBackend struct{}

var _ Backend = GioBackend{}

// Run implements Backend by delegating to the package-level Run function.
func (GioBackend) Run(cfg *config.Config) error {
	return Run(cfg)
}
