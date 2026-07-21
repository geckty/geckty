// Package assets embeds geckty's bundled binary assets so they ship inside
// the compiled binary rather than depending on a filesystem path relative
// to it (which wouldn't exist for an installed/distributed binary).
package assets

import _ "embed"

// Icon is geckty's app icon (see icon.png — the gecko-over-a-terminal
// mark), embedded as raw PNG bytes. Decode with image/png before use.
//
//go:embed icon.png
var Icon []byte
