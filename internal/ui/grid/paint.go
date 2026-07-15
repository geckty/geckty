// Package grid renders a VT terminal's cell grid as gio draw ops.
//
// Text is painted per cell on a fixed col×CellWidth grid so selection and
// glyphs stay aligned. Kitty graphics are composited on top via Placement.
package grid

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/geckty/geckty/internal/ui/theme"
	"github.com/geckty/geckty/internal/vt"
	"github.com/geckty/geckty/internal/vt/emu"
)

// ContentPad is the terminal content inset on all sides (Terminal.app-like).
const ContentPad = unit.Dp(8)

// Selection describes a text selection to highlight, in live-screen cell
// coordinates (row/col, not scrollback-adjusted). Bounds are inclusive and
// assumed already normalized (Start at-or-before End in reading order) —
// see session.Session.Selection, which returns them that way.
type Selection struct {
	Active     bool
	Start, End struct{ Col, Row int }
}

// Placement is one decoded Kitty-graphics image, positioned in the same
// chronological line addressing viewport() uses (AbsLine, an index into
// the conceptual History()+Screen() sequence) so it scrolls with the
// surrounding text — see session.Placement, which this mirrors at the
// grid package's boundary (grid doesn't import internal/session, matching
// how Selection is grid's own type rather than session's).
type Placement struct {
	// Seq is a stable per-placement identity used to cache its
	// paint.ImageOp across frames — see the package doc comment.
	Seq        uint64
	Image      image.Image
	AbsLine    int
	Col        int
	Cols, Rows int // requested cell span; 0 means "compute from pixel size"
}

// Painter draws a vt.Terminal's screen onto a gio frame.
type Painter struct {
	Theme      *material.Theme
	Palette    theme.Palette
	FontSize   unit.Sp
	LineHeight unit.Dp // optional override; 0 → 1.3× FontSize in Sp
	// CellWidth is the fixed monospace glyph advance width, in device
	// pixels. It must be measured against the actual font/size/DPI in
	// use (see internal/ui/app.go) — Paint does not measure it itself,
	// since that requires a live gio frame context to convert unit.Sp
	// to pixels correctly.
	CellWidth int

	// imgOps caches one paint.ImageOp per live Placement.Seq across
	// frames — see the package doc comment. Lazily initialized.
	imgOps map[uint64]paint.ImageOp
}

// LineHeightPx returns the row stride in device pixels — derived from
// FontSize (Sp) so text, selection, and hit-testing share one metric.
func (p *Painter) LineHeightPx(gtx layout.Context) int {
	if p.LineHeight > 0 {
		h := gtx.Dp(p.LineHeight)
		if h < 1 {
			return 1
		}
		return h
	}
	h := int(float32(gtx.Sp(p.FontSize))*1.3 + 0.5)
	if h < 1 {
		return 1
	}
	return h
}

// Paint fills the frame background, draws each cell, cursor (when
// blinkOn), and an optional transient scrollbar.
func (p *Painter) Paint(gtx layout.Context, term *vt.Terminal, scrollOffset int, sel Selection, placements []Placement, blinkOn, showScrollBar bool) layout.Dimensions {
	term.RLock()
	defer term.RUnlock()

	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, p.Palette.Background, clip.Rect{Max: size}.Op())

	sz := term.Size()
	lineHeightPx := p.LineHeightPx(gtx)

	if scrollOffset == 0 && sel.Active {
		p.paintSelection(gtx, sel, sz.C, lineHeightPx)
	}

	lines, top := viewport(term, sz.R, scrollOffset)
	for row, line := range lines {
		y := row * lineHeightPx
		if y >= size.Y {
			break
		}
		p.paintRow(gtx, line, sz.C, y, size.X, lineHeightPx)
	}

	p.paintPlacements(gtx, placements, top, sz.R, lineHeightPx, size)

	if scrollOffset == 0 && blinkOn {
		p.paintCursor(gtx, term, lineHeightPx)
	}

	if showScrollBar {
		hist := len(term.History())
		p.paintScrollBar(gtx, size, scrollOffset, hist, sz.R)
	}

	return layout.Dimensions{Size: size}
}

func (p *Painter) paintSelection(gtx layout.Context, sel Selection, cols, lineHeightPx int) {
	highlight := p.Palette.Selection
	if highlight.A == 0 {
		highlight = color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff}
	}

	for row := sel.Start.Row; row <= sel.End.Row; row++ {
		colStart, colEnd, ok := selectionColRange(sel, row, cols)
		if !ok {
			continue
		}
		y := row * lineHeightPx
		rect := image.Rect(colStart*p.CellWidth, y, colEnd*p.CellWidth, y+lineHeightPx)
		paint.FillShape(gtx.Ops, highlight, clip.Rect(rect).Op())
	}
}

func selectionColRange(sel Selection, row, cols int) (colStart, colEnd int, ok bool) {
	if row < sel.Start.Row || row > sel.End.Row {
		return 0, 0, false
	}
	colStart, colEnd = 0, cols
	if row == sel.Start.Row {
		colStart = sel.Start.Col
	}
	if row == sel.End.Row {
		colEnd = sel.End.Col + 1
	}
	if colEnd <= colStart {
		return 0, 0, false
	}
	return colStart, colEnd, true
}

func viewport(term *vt.Terminal, rows, scrollOffset int) (lines []emu.Line, top int) {
	history := term.History()
	if scrollOffset <= 0 {
		return term.Screen(), len(history)
	}

	screen := term.Screen()
	total := len(history) + len(screen)

	combined := make([]emu.Line, 0, total)
	combined = append(combined, history...)
	combined = append(combined, screen...)

	top = total - rows - scrollOffset
	if top < 0 {
		top = 0
	}
	bottom := top + rows
	if bottom > total {
		bottom = total
	}
	return combined[top:bottom], top
}

type cellStyle struct {
	fg, bg    emu.Color
	bold      bool
	italic    bool
	underline bool
}

func styleOf(g emu.Glyph) cellStyle {
	fg, bg := g.FG, g.BG
	if g.Mode&emu.AttrReverse != 0 {
		fg, bg = bg, fg
	}
	return cellStyle{
		fg:        fg,
		bg:        bg,
		bold:      g.Mode&emu.AttrBold != 0,
		italic:    g.Mode&emu.AttrItalic != 0,
		underline: g.Underline.Mode != emu.UnderlineNone,
	}
}

// paintRow paints each cell at its fixed grid X so selection/hit-testing
// stay locked to the monospace lattice.
func (p *Painter) paintRow(gtx layout.Context, line emu.Line, cols, y, width, lineHeightPx int) {
	if p.CellWidth <= 0 {
		return
	}

	for col := 0; col < cols; {
		var g emu.Glyph
		if col < len(line) {
			g = line[col]
		}
		if g.Char == 0 {
			col++
			continue
		}

		w := g.Width()
		if w < 1 {
			w = 1
		}
		x0 := col * p.CellWidth
		cellW := w * p.CellWidth
		if x0 >= width {
			break
		}
		if x0+cellW > width {
			cellW = width - x0
		}

		st := styleOf(g)
		fgColor := p.Palette.Resolve(st.fg)
		bgColor := p.Palette.Resolve(st.bg)

		if bgColor != p.Palette.Background {
			rect := image.Rect(x0, y, x0+cellW, y+lineHeightPx)
			paint.FillShape(gtx.Ops, bgColor, clip.Rect(rect).Op())
		}

		off := op.Offset(image.Pt(x0, y)).Push(gtx.Ops)
		clipStack := clip.Rect{Max: image.Pt(cellW, lineHeightPx)}.Push(gtx.Ops)
		cellGtx := gtx
		cellGtx.Constraints = layout.Exact(image.Pt(cellW, lineHeightPx))
		layout.Center.Layout(cellGtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(p.Theme, p.FontSize, string(g.Char))
			label.Color = fgColor
			label.Font.Typeface = theme.MonospaceTypeface
			label.MaxLines = 1
			if st.bold {
				label.Font.Weight = font.Bold
			}
			if st.italic {
				label.Font.Style = font.Italic
			}
			return label.Layout(gtx)
		})
		clipStack.Pop()
		off.Pop()

		if st.underline {
			const thickness = 1
			uy := y + lineHeightPx - thickness
			rect := image.Rect(x0, uy, x0+cellW, uy+thickness)
			paint.FillShape(gtx.Ops, fgColor, clip.Rect(rect).Op())
		}

		col += w
	}
}

func (p *Painter) paintCursor(gtx layout.Context, term *vt.Terminal, lineHeightPx int) {
	if !term.CursorVisible() {
		return
	}
	cursor := term.Cursor()
	x0 := cursor.C * p.CellWidth
	y0 := cursor.R * lineHeightPx

	var rect image.Rectangle
	switch cursor.Style {
	case emu.CursorStyleUnderline, emu.CursorStyleBlinkUnderline:
		const thickness = 2
		rect = image.Rect(x0, y0+lineHeightPx-thickness, x0+p.CellWidth, y0+lineHeightPx)
	case emu.CursorStyleBar, emu.CursorStyleBlinkBar:
		const thickness = 2
		rect = image.Rect(x0, y0, x0+thickness, y0+lineHeightPx)
	default:
		rect = image.Rect(x0, y0, x0+p.CellWidth, y0+lineHeightPx)
	}
	paint.FillShape(gtx.Ops, p.Palette.Foreground, clip.Rect(rect).Op())
}

func (p *Painter) paintScrollBar(gtx layout.Context, size image.Point, scrollOffset, historyLen, rows int) {
	if historyLen <= 0 || size.Y < 8 {
		return
	}
	trackW := gtx.Dp(unit.Dp(7))
	if trackW < 4 {
		trackW = 4
	}
	margin := gtx.Dp(unit.Dp(2))
	track := image.Rect(size.X-trackW-margin, margin, size.X-margin, size.Y-margin)
	if track.Dx() < 2 || track.Dy() < 8 {
		return
	}
	trackBG := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x28}
	paint.FillShape(gtx.Ops, trackBG, clip.UniformRRect(track, track.Dx()/2).Op(gtx.Ops))

	total := historyLen + rows
	thumbH := track.Dy() * rows / total
	minThumb := gtx.Dp(unit.Dp(24))
	if thumbH < minThumb {
		thumbH = minThumb
	}
	if thumbH > track.Dy() {
		thumbH = track.Dy()
	}
	travel := track.Dy() - thumbH
	thumbY := track.Max.Y - thumbH // live bottom
	if historyLen > 0 {
		// scrollOffset=0 → bottom; scrollOffset=historyLen → top
		thumbY = track.Min.Y + travel*(historyLen-scrollOffset)/historyLen
	}
	thumb := image.Rect(track.Min.X, thumbY, track.Max.X, thumbY+thumbH)
	thumbFG := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x70}
	paint.FillShape(gtx.Ops, thumbFG, clip.UniformRRect(thumb, thumb.Dx()/2).Op(gtx.Ops))
}

func (p *Painter) paintPlacements(gtx layout.Context, placements []Placement, top, rows, lineHeightPx int, size image.Point) {
	if len(placements) == 0 {
		p.imgOps = nil
		return
	}
	if p.imgOps == nil {
		p.imgOps = make(map[uint64]paint.ImageOp)
	}

	clipStack := clip.Rect{Max: size}.Push(gtx.Ops)
	live := make(map[uint64]bool, len(placements))
	for _, pl := range placements {
		live[pl.Seq] = true

		localRow := pl.AbsLine - top
		if localRow < 0 || localRow >= rows {
			continue
		}

		imgOp, ok := p.imgOps[pl.Seq]
		if !ok {
			imgOp = paint.NewImageOp(pl.Image)
			p.imgOps[pl.Seq] = imgOp
		}

		cols, rowSpan := cellSpan(pl, imgOp.Size(), p.CellWidth, lineHeightPx)
		x0, y0 := pl.Col*p.CellWidth, localRow*lineHeightPx
		w, h := cols*p.CellWidth, rowSpan*lineHeightPx

		off := op.Offset(image.Pt(x0, y0)).Push(gtx.Ops)
		imgGtx := gtx
		imgGtx.Constraints = layout.Exact(image.Pt(w, h))
		widget.Image{Src: imgOp, Fit: widget.Fill}.Layout(imgGtx)
		off.Pop()
	}
	clipStack.Pop()

	for seq := range p.imgOps {
		if !live[seq] {
			delete(p.imgOps, seq)
		}
	}
}

func cellSpan(pl Placement, pixelSize image.Point, cellWidth, lineHeightPx int) (cols, rows int) {
	cols, rows = pl.Cols, pl.Rows
	if cols <= 0 && cellWidth > 0 {
		cols = (pixelSize.X + cellWidth - 1) / cellWidth
	}
	if rows <= 0 && lineHeightPx > 0 {
		rows = (pixelSize.Y + lineHeightPx - 1) / lineHeightPx
	}
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return cols, rows
}
