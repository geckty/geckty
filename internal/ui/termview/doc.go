// Package termview paints the terminal cell grid (and related glyphs) into
// an RGBA buffer: fonts, glyph atlas, Painter, and a retained TerminalWidget
// for a future gogpu/ui present path. The live hot path is Painter.Paint
// invoked from internal/ui/app.
package termview
