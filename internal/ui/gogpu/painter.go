package gogpu

import (
	"image"
	"image/color"

	"golang.org/x/image/font"

	"github.com/geckty/geckty/internal/ui/theme"
	"github.com/geckty/geckty/internal/vt"
	"github.com/geckty/geckty/internal/vt/emu"
)

// Selection describes a text selection to highlight, in live-screen cell
// coordinates (row/col, not scrollback-adjusted). Bounds are inclusive and
// assumed already normalized (Start at-or-before End in reading order).
type Selection struct {
	Active     bool
	Start, End struct{ Col, Row int }
}

// Placement is one decoded Kitty-graphics image, positioned in the same
// chronological line addressing viewport() uses (AbsLine, an index into
// the conceptual History()+Screen() sequence) so it scrolls with the
// surrounding text.
type Placement struct {
	Seq        uint64
	Image      *image.RGBA
	AbsLine    int
	Col        int
	Cols, Rows int // requested cell span; 0 means "compute from pixel size"
}

// Painter renders a vt.Terminal's screen as RGBA pixels into a caller-owned
// buffer. Unlike the old gio-based grid.Painter, it does not measure its
// own cell metrics from a live frame context — Face/CellWidth/CellHeight/
// Ascent are computed once by loadFontBundle (see font.go) and kept in sync
// by the caller (app.go) across font/DPI changes.
type Painter struct {
	Palette    theme.Palette
	Face       font.Face
	CellWidth  int
	CellHeight int
	Ascent     int

	atlas *glyphAtlas
}

// ensureAtlas (re)creates the glyph atlas when the face or ascent changed.
func (p *Painter) ensureAtlas() {
	if !p.atlas.valid(p.Face, p.Ascent) {
		p.atlas = newGlyphAtlas(p.Face, p.Ascent)
	}
}

// Paint fills the grid's rect (originX,originY)-(originX+cols*CellWidth,
// originY+rows*CellHeight) in buf, draws each cell, the cursor (when
// blinkOn), and any Kitty-graphics placements. buf is RGBA8, stride =
// frameW*4. Returns true if any pixel was written.
func (p *Painter) Paint(buf []byte, frameW, frameH, originX, originY int, term *vt.Terminal, scrollOffset int, sel Selection, placements []Placement, blinkOn bool) bool {
	term.RLock()
	defer term.RUnlock()

	if p.CellWidth <= 0 || p.CellHeight <= 0 {
		return false
	}
	p.ensureAtlas()

	sz := term.Size()
	gridW := sz.C * p.CellWidth
	gridH := sz.R * p.CellHeight
	fillRect(buf, frameW, originX, originY, originX+gridW, originY+gridH, toRGBA(p.Palette.Background))

	if scrollOffset == 0 && sel.Active {
		p.paintSelection(buf, frameW, sel, sz.C, originX, originY)
	}

	lines, top := viewport(term, sz.R, scrollOffset)
	for row, line := range lines {
		y := originY + row*p.CellHeight
		if y >= originY+gridH || y >= frameH {
			break
		}
		p.paintRow(buf, frameW, frameH, line, sz.C, originX, y)
	}

	p.paintPlacements(buf, frameW, frameH, placements, top, sz.R, originX, originY, gridW)

	if scrollOffset == 0 && blinkOn {
		p.paintCursor(buf, frameW, term, originX, originY)
	}

	return true
}

func (p *Painter) paintSelection(buf []byte, frameW int, sel Selection, cols, originX, originY int) {
	highlight := p.Palette.Selection
	if highlight.A == 0 {
		highlight = color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff}
	}
	for row := sel.Start.Row; row <= sel.End.Row; row++ {
		colStart, colEnd, ok := selectionColRange(sel, row, cols)
		if !ok {
			continue
		}
		y := originY + row*p.CellHeight
		fillRect(buf, frameW, originX+colStart*p.CellWidth, y, originX+colEnd*p.CellWidth, y+p.CellHeight, toRGBA(highlight))
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

// glyphBleedPx is how far past a cell's own [x0,x1) a glyph's ink may still
// be painted, to cover side-bearing overhang (e.g. an italic slant or a
// wide serif) without letting it bleed into the next cell over.
const glyphBleedPx = 3

// paintRow paints each cell at its fixed grid X. Unlike the old gio
// renderer, bold/italic are not yet applied to the glyph rasterization
// (the atlas caches one regular-weight rasterization per rune) — see the
// "bold/italic faces" follow-up note in font.go.
func (p *Painter) paintRow(buf []byte, frameW, frameH int, line emu.Line, cols, originX, y int) {
	y1 := y + p.CellHeight
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
		x0 := originX + col*p.CellWidth
		x1 := x0 + w*p.CellWidth

		st := styleOf(g)
		fgColor := toRGBA(p.Palette.Resolve(st.fg))
		bgColor := toRGBA(p.Palette.Resolve(st.bg))

		if bgColor != toRGBA(p.Palette.Background) {
			fillRect(buf, frameW, x0, y, x1, y1, bgColor)
		}

		if g.Char != ' ' {
			if e, ok := p.atlas.get(g.Char); ok {
				dr := e.drRel.Add(image.Pt(x0, y))
				blitGlyphClipped(buf, frameW, frameH, dr, e.mask, e.maskp, fgColor, x0-glyphBleedPx, y, x1+glyphBleedPx, y1)
			}
		}

		if st.underline {
			const thickness = 1
			uy := y1 - thickness
			fillRect(buf, frameW, x0, uy, x1, uy+thickness, fgColor)
		}

		col += w
	}
}

func (p *Painter) paintCursor(buf []byte, frameW int, term *vt.Terminal, originX, originY int) {
	if !term.CursorVisible() {
		return
	}
	cursor := term.Cursor()
	x0 := originX + cursor.C*p.CellWidth
	y0 := originY + cursor.R*p.CellHeight
	fg := toRGBA(p.Palette.Foreground)

	switch cursor.Style {
	case emu.CursorStyleUnderline, emu.CursorStyleBlinkUnderline:
		const thickness = 2
		fillRect(buf, frameW, x0, y0+p.CellHeight-thickness, x0+p.CellWidth, y0+p.CellHeight, fg)
	case emu.CursorStyleBar, emu.CursorStyleBlinkBar:
		const thickness = 2
		fillRect(buf, frameW, x0, y0, x0+thickness, y0+p.CellHeight, fg)
	default:
		fillRect(buf, frameW, x0, y0, x0+p.CellWidth, y0+p.CellHeight, fg)
	}
}

func (p *Painter) paintPlacements(buf []byte, frameW, frameH int, placements []Placement, top, rows, originX, originY, gridW int) {
	for _, pl := range placements {
		localRow := pl.AbsLine - top
		if localRow < 0 || localRow >= rows || pl.Image == nil {
			continue
		}
		cols, rowSpan := cellSpan(pl, pl.Image.Bounds().Size(), p.CellWidth, p.CellHeight)
		x0 := originX + pl.Col*p.CellWidth
		y0 := originY + localRow*p.CellHeight
		w := cols * p.CellWidth
		h := rowSpan * p.CellHeight
		if x0+w > originX+gridW {
			w = originX + gridW - x0
		}
		blitImageScaled(buf, frameW, frameH, pl.Image, x0, y0, w, h)
	}
}

func cellSpan(pl Placement, pixelSize image.Point, cellWidth, cellHeight int) (cols, rows int) {
	cols, rows = pl.Cols, pl.Rows
	if cols <= 0 && cellWidth > 0 {
		cols = (pixelSize.X + cellWidth - 1) / cellWidth
	}
	if rows <= 0 && cellHeight > 0 {
		rows = (pixelSize.Y + cellHeight - 1) / cellHeight
	}
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return cols, rows
}

func toRGBA(c color.NRGBA) color.RGBA {
	return color.RGBA(c)
}
