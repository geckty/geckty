// Package assets embeds geckty's bundled binary assets so they ship inside
// the compiled binary rather than depending on a filesystem path relative
// to it (which wouldn't exist for an installed/distributed binary).
package assets

import "embed"

// Icon is geckty's app icon (see icon.png — the gecko-over-a-terminal
// mark), embedded as raw PNG bytes. Decode with image/png before use.
//
//go:embed icon.png
var Icon []byte

// Fonts holds geckty's bundled default typefaces, each shipped as static
// Regular/Bold/Italic/BoldItalic TTFs (not variable fonts — the opentype
// parsing this project uses, golang.org/x/image/font/sfnt, has no fvar/gvar
// axis support, so a variable font would always rasterize at its neutral
// instance regardless of requested weight):
//
//   - fonts/mono/IBMPlexMono-{Regular,Bold,Italic,BoldItalic}.ttf — the
//     terminal grid's default font (see internal/config's Font.Family),
//     replacing the old embedded gomono.TTF fallback with something that
//     actually looks good out of the box on every platform, not just where
//     a decent system monospace happens to be installed.
//   - fonts/ui/PTSans-{Regular,Bold,Italic,BoldItalic}.ttf — the tab bar's
//     default font (see internal/config's UIFont.Family), distinct from the
//     terminal grid's since chrome text has no reason to be monospaced.
//
// Both are OFL-licensed (see each subdirectory's OFL.txt) and pulled from
// google/fonts (github.com/google/fonts) — IBM Plex Mono by IBM, PT Sans by
// ParaType.
//
//go:embed fonts
var Fonts embed.FS

// Themes holds shipped theme TOML files (themes/<name>.toml). These are the
// primary built-in theme source; config.Glass* and defaultUI() are populated
// from the embedded glass file at init / load. User theme files under
// ~/.config/geckty/themes/ take precedence when present.
//
//go:embed themes
var Themes embed.FS
