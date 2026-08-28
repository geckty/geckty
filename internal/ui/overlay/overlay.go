// Package overlay holds search and URL-hints UI state that sits above the
// terminal grid. Toolkit-agnostic: the active UI backend paints these
// structures; behavior lives here so gogpu/ui widgets and the legacy
// present path can share the same models.
package overlay

import "github.com/geckty/geckty/internal/session"

// Search holds in-progress scrollback search state.
type Search struct {
	Active  bool
	Query   string
	Hit     session.SearchHit
	HasHit  bool
	Message string // status / "no matches" text
}

// Hints holds the URL-hints overlay (keyboard label picker).
type Hints struct {
	Active bool
	Hits   []session.URLHit
	Labels []string
}
