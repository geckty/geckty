// Package ui defines the toolkit-agnostic Backend interface that
// cmd/geckty/main.go drives; internal/ui/app implements it.
package ui

import "github.com/geckty/geckty/internal/config"

// Backend runs geckty's UI for a given config, blocking until the window
// closes. internal/ui/app.Backend is the only implementation today —
// the interface exists so cmd/geckty/main.go doesn't need to know which
// UI toolkit backs it.
//
// Deliberately thin: this doesn't try to split rendering/input/chrome
// behind their own interfaces. That would be a much larger, currently
// unmotivated restructuring — nothing in geckty needs a second backend
// today, so there's nothing to design this interface's shape against yet
// beyond "runs, given a config, blocking until done."
type Backend interface {
	Run(cfg *config.Config) error
}
