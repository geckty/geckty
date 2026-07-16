package ui

import "github.com/geckty/geckty/internal/config"

// Backend runs geckty's UI for a given config, blocking until the window
// closes. internal/ui/gogpu.Backend is the only implementation today (it
// replaced an earlier gio-based one after gio's D3D11 backend turned out
// to corrupt glyph rendering on Windows) — the interface exists so
// cmd/geckty/main.go doesn't need to know which UI toolkit backs it.
//
// Deliberately thin: this doesn't try to split rendering/input/chrome
// behind their own interfaces. That would be a much larger, currently
// unmotivated restructuring — nothing in geckty needs a second backend
// today, so there's nothing to design this interface's shape against yet
// beyond "runs, given a config, blocking until done."
type Backend interface {
	Run(cfg *config.Config) error
}
