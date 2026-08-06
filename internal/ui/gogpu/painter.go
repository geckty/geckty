package gogpu

import (
	"image"
	"image/color"

	"golang.org/x/image/font"

	"github.com/geckty/geckty/internal/ui/theme"
	"github.com/geckty/geckty/internal/vt"
	"github.com/geckty/geckty/internal/vt/emu"
)

// Selection describes a text selection to highlight in viewport cell
// coordinates (row 0 = top visible row). Bounds are inclusive and assumed
// already normalized (Start at-or-before End in reading order). Callers
// convert absolute History()+Screen() selection bounds via gridSelection.
// When Rect is set, every row uses the same [minCol, maxCol] column span
// (Alt-drag rectangular selection) instead of stream wrapping.
type Selection struct {
	Active     bool
	Rect       bool
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
// buffer. It does not measure its own cell metrics from a live frame
// context — Fonts/CellWidth/CellHeight/Ascent are computed once by
// loadFontBundle (see font.go) and kept in sync by the caller (app.go)
// across font/DPI changes.
type Painter struct {
	Palette    theme.Palette
	Fonts      fontBundle
	CellWidth  int
	CellHeight int
	Ascent     int

	// atlases[styleIndex(bold,italic)] caches that style's glyph
	// rasterizations; index 0 (regular) is always populated once Fonts is,
	// the other three only when Fonts has a dedicated face for them (see
	// faceAndAtlas's fallback when it doesn't).
	atlases [4]*glyphAtlas

	// fallbackFace/atlas rasterize glyphs the primary mono face lacks
	// (emoji outlines, symbols, box-drawing from a symbol font).
	fallbackFace  font.Face
	fallbackAtlas *glyphAtlas
}

// styleIndex maps a cell's bold/italic attributes to a slot in
// Painter.atlases (and the equivalent face in fontBundle.face).
func styleIndex(bold, italic bool) int {
	switch {
	case bold && italic:
		return 3
	case bold:
		return 1
	case italic:
		return 2
	default:
		return 0
	}
}

// ensureAtlas (re)creates each style's glyph atlas when its face or the
// shared ascent changed.
func (p *Painter) ensureAtlas() {
	faces := [4]font.Face{p.Fonts.regular, p.Fonts.bold, p.Fonts.italic, p.Fonts.boldItalic}
	for i, f := range faces {
		if f == nil {
			continue
		}
		if !p.atlases[i].valid(f, p.Ascent) {
			p.atlases[i] = newGlyphAtlas(f, p.Ascent)
		}
	}
	if p.fallbackFace != nil && !p.fallbackAtlas.valid(p.fallbackFace, p.Ascent) {
		p.fallbackAtlas = newGlyphAtlas(p.fallbackFace, p.Ascent)
	}
}

// faceAndAtlas returns the face and atlas for a cell's bold/italic
// attributes, delegating to fontBundle.face for which style (falling back
// to regular when Fonts has no dedicated face for the requested one) so
// the two never disagree about which face is "in use".
func (p *Painter) faceAndAtlas(bold, italic bool) (font.Face, *glyphAtlas) {
	face, idx := p.Fonts.face(bold, italic)
	return face, p.atlases[idx]
}

// glyphEntryFor looks up r in the style atlas, then the symbol/emoji
// fallback atlas when the primary face has no glyph.
func (p *Painter) glyphEntryFor(bold, italic bool, r rune) (glyphEntry, bool) {
	if _, atlas := p.faceAndAtlas(bold, italic); atlas != nil {
		if e, ok := atlas.get(r); ok {
			return e, true
		}
	}
	if p.fallbackAtlas != nil {
		return p.fallbackAtlas.get(r)
	}
	return glyphEntry{}, false
}

// Paint fills the grid's rect (originX,originY)-(originX+cols*CellWidth,
// originY+rows*CellHeight) in buf, draws each cell, the cursor (when
// blinkOn), and any Kitty-graphics placements. buf is RGBA8, stride =
// frameW*4. Returns true if any pixel was written.
//
// When dirtyRows is non-nil, only those view rows are cleared and repainted
// (caller must not have wiped the whole grid); the cursor is always
// refreshed. Pass nil for a full grid paint.
func (p *Painter) Paint(buf []byte, frameW, frameH, originX, originY int, term *vt.Terminal, scrollOffset int, sel Selection, placements []Placement, blinkOn bool, dirtyRows map[int]bool) bool {
	term.RLock()
	defer term.RUnlock()

	if p.CellWidth <= 0 || p.CellHeight <= 0 {
		return false
	}
	p.ensureAtlas()

	sz := term.Size()
	gridW := sz.C * p.CellWidth
	gridH := sz.R * p.CellHeight
	bg := toRGBA(p.Palette.Background)

	if dirtyRows == nil {
		fillRect(buf, frameW, originX, originY, originX+gridW, originY+gridH, bg)

		// Selection is in live viewport cell rows (gridSelection already
		// clipped absolute History()+Screen() bounds to the visible window),
		// so it paints whether or not we're scrolled into history.
		if sel.Active {
			p.paintSelection(buf, frameW, sel, sz.C, originX, originY)
		}

		lines, top := viewport(term, sz.R, scrollOffset)
		for row, line := range lines {
			y := originY + row*p.CellHeight
			if y >= originY+gridH || y >= frameH {
				break
			}
			p.paintRow(buf, frameW, frameH, line, sz.C, originX, y, sel, row)
		}

		p.paintPlacements(buf, frameW, frameH, placements, top, sz.R, originX, originY, gridW)
	} else {
		lines, top := viewport(term, sz.R, scrollOffset)
		for row := range dirtyRows {
			if row < 0 || row >= len(lines) || row >= sz.R {
				continue
			}
			y := originY + row*p.CellHeight
			if y >= originY+gridH || y >= frameH {
				continue
			}
			fillRect(buf, frameW, originX, y, originX+gridW, y+p.CellHeight, bg)
			if sel.Active {
				// Re-paint selection highlight for this row only.
				colStart, colEnd, ok := selectionColRange(sel, row, sz.C)
				if ok {
					highlight := p.Palette.Selection
					if highlight.A == 0 {
						highlight = color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff}
					}
					fillRect(buf, frameW, originX+colStart*p.CellWidth, y, originX+colEnd*p.CellWidth, y+p.CellHeight, toRGBA(highlight))
				}
			}
			p.paintRow(buf, frameW, frameH, lines[row], sz.C, originX, y, sel, row)
		}
		_ = top // placements skipped on partial paint (best-effort)
	}

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
	if sel.Rect {
		c0, c1 := sel.Start.Col, sel.End.Col
		if c1 < c0 {
			c0, c1 = c1, c0
		}
		if c0 < 0 {
			c0 = 0
		}
		if c1 >= cols {
			c1 = cols - 1
		}
		if c1 < c0 {
			return 0, 0, false
		}
		return c0, c1 + 1, true
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
	fg, bg          emu.Color
	bold            bool
	italic          bool
	underline       bool
	underlineMode   emu.UnderlineMode
	underlineColor  emu.Color
	strikethrough   bool
	dim             bool
	invisible       bool
}

func styleOf(g emu.Glyph) cellStyle {
	fg, bg := g.FG, g.BG
	if g.Mode&emu.AttrReverse != 0 {
		fg, bg = bg, fg
	}
	return cellStyle{
		fg:             fg,
		bg:             bg,
		bold:           g.Mode&emu.AttrBold != 0,
		italic:         g.Mode&emu.AttrItalic != 0,
		underline:      g.Underline.Mode != emu.UnderlineNone,
		underlineMode:  g.Underline.Mode,
		underlineColor: g.Underline.Color,
		strikethrough:  g.Mode&emu.AttrStrikethrough != 0,
		dim:            g.Mode&emu.AttrDim != 0,
		invisible:      g.Mode&emu.AttrInvisible != 0,
	}
}

// glyphBleedPx is how far past a cell's own [x0,x1) a glyph's ink may still
// be painted, to cover side-bearing overhang (e.g. an italic slant or a
// wide serif) without letting it bleed into the next cell over.
const glyphBleedPx = 3

// paintRow paints each cell at its fixed grid X, rasterizing bold/italic
// cells from Fonts' dedicated bold/italic/boldItalic faces when the config
// enabled them (see ensureFonts) and Fonts actually has one — otherwise
// faceAndAtlas falls back to the regular face/atlas. When sel covers a
// cell, glyphs use Palette.SelectionFG instead of the cell's foreground.
func (p *Painter) paintRow(buf []byte, frameW, frameH int, line emu.Line, cols, originX, y int, sel Selection, viewRow int) {
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
		if st.dim {
			fgColor = dimRGBA(fgColor, bgColor)
		}
		if sel.Active {
			if colStart, colEnd, ok := selectionColRange(sel, viewRow, cols); ok && col >= colStart && col < colEnd {
				fgColor = toRGBA(p.Palette.SelectionFG)
			}
		}

		if bgColor != toRGBA(p.Palette.Background) {
			fillRect(buf, frameW, x0, y, x1, y1, bgColor)
		}

		if !st.invisible && g.Char != ' ' {
			if e, ok := p.glyphEntryFor(st.bold, st.italic, g.Char); ok {
				dr := e.drRel.Add(image.Pt(x0, y))
				blitGlyphClipped(buf, frameW, frameH, dr, e.mask, e.maskp, fgColor, x0-glyphBleedPx, y, x1+glyphBleedPx, y1)
			}
		}

		if st.underline {
			ul := fgColor
			if !st.underlineColor.Default() {
				ul = toRGBA(p.Palette.Resolve(st.underlineColor))
			}
			paintUnderline(buf, frameW, x0, x1, y1, st.underlineMode, ul)
		} else if g.Hyperlink != "" {
			// OSC 8 hyperlinks get a subtle underline so they're visible
			// without requiring hover chrome.
			linkUL := color.RGBA{R: 0x58, G: 0x33, B: 0xff, A: 0xff}
			paintUnderline(buf, frameW, x0, x1, y1, emu.UnderlineSingle, linkUL)
		}
		if st.strikethrough {
			sy := y + p.CellHeight/2
			fillRect(buf, frameW, x0, sy, x1, sy+1, fgColor)
		}

		col += w
	}
}

func dimRGBA(fg, bg color.RGBA) color.RGBA {
	return color.RGBA{
		R: uint8((int(fg.R) + int(bg.R)) / 2),
		G: uint8((int(fg.G) + int(bg.G)) / 2),
		B: uint8((int(fg.B) + int(bg.B)) / 2),
		A: fg.A,
	}
}

func paintUnderline(buf []byte, frameW, x0, x1, y1 int, mode emu.UnderlineMode, fg color.RGBA) {
	const thickness = 1
	uy := y1 - thickness
	switch mode {
	case emu.UnderlineDouble:
		fillRect(buf, frameW, x0, uy, x1, uy+thickness, fg)
		fillRect(buf, frameW, x0, uy-2, x1, uy-2+thickness, fg)
	case emu.UnderlineDotted:
		for x := x0; x < x1; x += 2 {
			xe := x + 1
			if xe > x1 {
				xe = x1
			}
			fillRect(buf, frameW, x, uy, xe, uy+thickness, fg)
		}
	case emu.UnderlineDashed:
		for x := x0; x < x1; x += 4 {
			xe := x + 2
			if xe > x1 {
				xe = x1
			}
			fillRect(buf, frameW, x, uy, xe, uy+thickness, fg)
		}
	case emu.UnderlineCurly:
		// Sine-ish undercurl: one full wave every ~cell-ish period.
		const period = 8
		const amp = 2
		base := uy - 1
		for x := x0; x < x1; x++ {
			phase := (x - x0) % period
			var dy int
			switch {
			case phase < 2:
				dy = 0
			case phase < 4:
				dy = -amp
			case phase < 6:
				dy = 0
			default:
				dy = amp
			}
			y := base + dy
			fillRect(buf, frameW, x, y, x+1, y+thickness, fg)
		}
	default:
		fillRect(buf, frameW, x0, uy, x1, uy+thickness, fg)
	}
}

func (p *Painter) paintCursor(buf []byte, frameW int, term *vt.Terminal, originX, originY int) {
	if !term.CursorVisible() {
		return
	}
	cursor := term.Cursor()
	x0 := originX + cursor.C*p.CellWidth
	y0 := originY + cursor.R*p.CellHeight
	fg := toRGBA(p.Palette.Cursor)

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
