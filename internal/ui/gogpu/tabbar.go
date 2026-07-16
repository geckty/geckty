package gogpu

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"golang.org/x/image/font"

	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
	"github.com/geckty/geckty/internal/ui/theme"
)

// Tab-bar sizing, in logical (DPI-independent) pixels — same values as the
// old gio-based chrome.Height/MinTabWidth/etc, converted to device pixels
// by the caller (app.go) via dpToPx before reaching chrome.ComputeGeometry,
// which is pure-Go and reused unchanged from the old chrome package.
const (
	TabBarHeightDp   = 32 // total tab strip height
	MinTabWidthDp    = 80 // narrowest a tab pill may shrink to before tabs scroll instead
	PlusWidthDp      = 36 // "+" button's reserved slot width
	CloseZoneWidthDp = 20 // hit-zone width for a tab's close ×, from its left edge
	CloseEdgePadDp   = 8  // gap between a tab's left edge and its close-zone
	tabInsetDp       = 10 // horizontal gap between a tab's edge and its title text
	capsuleVPadDp    = 4  // vertical gap between the bar edge and a tab pill's rounded rect
	capsuleSegInDp   = 3  // horizontal gap between adjacent tab pills

	tabTitleMaxRunes = 28 // longest title before truncateTitle elides its middle

	inactiveFGDim = 0.32 // how far an inactive tab's title dims toward the bar background
	hoverFGDim    = 0.10 // how far a hovered (but inactive) tab's title dims

	glassPlusDefault = 0.10 // "+" chip's glass fill factor at rest — chrome has no plus-button concept, so this stays local
	glassPlusHover   = 0.24 // "+" chip's glass fill factor while hovered

	titleCloseReserveDp = 4 // gap reserved after the close-zone before centered title text may start
	closeArmLenDp       = 4 // close "×" per-arm half-length
	closeArmThicknessDp = 1 // close "×" stroke thickness
	plusChipMarginDp    = 2 // gap between the "+" chip circle and the bar edges
	plusArmLenDp        = 5 // "+" arm half-length — 1px larger than closeArmLenDp since the "+" sits in a bigger chip and reads too small at the same size
	plusArmThicknessDp  = 2 // "+" stroke — thicker than closeArmThicknessDp to stay balanced against the larger arm length
)

// dpToPx converts a logical (DPI-independent) pixel size to device pixels
// at the given scale factor, rounding half up rather than truncating so
// small UI elements (a 1dp stroke, a 2dp margin) don't collapse to 0px at
// fractional scale factors.
func dpToPx(dp int, scale float64) int { return int(float64(dp)*scale + 0.5) }

// TabBar renders geckty's tab strip as direct pixel writes, reusing the old
// chrome package's pure-Go geometry/hit-test functions (ComputeGeometry,
// TabAtScrolledPinned, DropIndexByOverlap, VisualTabSlot, etc.) and its
// glass/dim color-blend helpers (GlassStyle, GlassFill, DimFG) — all
// unchanged, already gio-free — and adding only the paint side.
type TabBar struct {
	Face   font.Face // tab-bar UI font face, at its own (smaller) size
	Ascent int
	Scale  float64 // device pixels per logical (dp) pixel

	atlas *glyphAtlas
}

// NewTabBar returns an empty TabBar. Callers must set Face/Ascent/Scale
// (see app.go's ensureFonts) before the first Layout call — an unset Face
// makes drawText/drawTextCentered/drawTextAt no-ops.
func NewTabBar() *TabBar { return &TabBar{} }

func (tb *TabBar) ensureAtlas() {
	if !tb.atlas.valid(tb.Face, tb.Ascent) {
		tb.atlas = newGlyphAtlas(tb.Face, tb.Ascent)
	}
}

// drawText draws s left-aligned starting at x0, clipped to [x0,x0+w) x
// [y0,y0+h).
func (tb *TabBar) drawText(buf []byte, frameW, frameH int, s string, x0, y0, w, h int, fg color.RGBA) {
	tb.drawTextAt(buf, frameW, frameH, s, x0, x0, y0, w, h, fg)
}

// drawTextCentered draws s horizontally centered within [x0,x0+w), still
// clipped to that box so an unexpectedly long string doesn't bleed past it.
func (tb *TabBar) drawTextCentered(buf []byte, frameW, frameH int, s string, x0, y0, w, h int, fg color.RGBA) {
	textW := tb.measureText(s)
	start := x0 + (w-textW)/2
	if start < x0 {
		start = x0
	}
	tb.drawTextAt(buf, frameW, frameH, s, start, x0, y0, w, h, fg)
}

// drawTextAt draws s starting at startX (the glyph origin), clipped to the
// box [clipX0,clipX0+w) x [y0,y0+h).
func (tb *TabBar) drawTextAt(buf []byte, frameW, frameH int, s string, startX, clipX0, y0, w, h int, fg color.RGBA) {
	if tb.Face == nil {
		return
	}
	tb.ensureAtlas()
	// Center vertically using the face ascent.
	dotY := y0 + (h+tb.Ascent)/2
	x := startX
	for _, r := range s {
		if r == ' ' {
			if a, ok := tb.Face.GlyphAdvance(r); ok {
				x += a.Round()
			}
			continue
		}
		if e, ok := tb.atlas.get(r); ok {
			dr := e.drRel.Add(image.Pt(x, dotY-tb.Ascent))
			blitGlyphClipped(buf, frameW, frameH, dr, e.mask, e.maskp, fg, clipX0, y0, clipX0+w, y0+h)
		}
		if a, ok := tb.Face.GlyphAdvance(r); ok {
			x += a.Round()
		}
	}
}

// measureText returns s's rendered width in pixels at the tab bar's face.
func (tb *TabBar) measureText(s string) int {
	if tb.Face == nil {
		return 0
	}
	x := 0
	for _, r := range s {
		if a, ok := tb.Face.GlyphAdvance(r); ok {
			x += a.Round()
		}
	}
	return x
}

// Layout draws the tab bar into buf's top barH rows: the bar background,
// each tab pill (in two passes so a dragged tab paints last, on top of the
// others), the "+" button, and any plugin status text. drag carries the
// in-progress drag-reorder's visual state (chrome.DragVisual — which tab is
// being dragged, its horizontal offset, and the current drop-index preview)
// and is the zero value when no drag is active. hoverPlus lightens the "+"
// button's chip/cross when the pointer is over it, independent of activeID
// or any tab hover state.
func (tb *TabBar) Layout(buf []byte, frameW, frameH, barH int, pal theme.Palette, tabs []session.Tab, activeID int, statusText string, drag chrome.DragVisual, hoverPlus bool) {
	barBG := toRGBA(chrome.GlassFill(pal.Background, chrome.GlassBarLift))
	fillRect(buf, frameW, 0, 0, frameW, barH, barBG)

	minTabW := dpToPx(MinTabWidthDp, tb.Scale)
	plusW := dpToPx(PlusWidthDp, tb.Scale)
	closeZoneW := dpToPx(CloseZoneWidthDp, tb.Scale)
	inset := dpToPx(capsuleSegInDp, tb.Scale)

	g := chrome.ComputeGeometry(frameW, len(tabs), minTabW, plusW, closeZoneW)
	scrollX := drag.ScrollX
	if max := chrome.ScrollMax(g, len(tabs)); scrollX > max {
		scrollX = max
	}
	if scrollX < 0 {
		scrollX = 0
	}
	activeIdx := -1
	for i := range tabs {
		if tabs[i].ID == activeID {
			activeIdx = i
			break
		}
	}

	for pass := 0; pass < 2; pass++ {
		for i, t := range tabs {
			isDragged := drag.Active && t.ID == drag.TabID
			if pass == 0 && isDragged {
				continue
			}
			if pass == 1 && !isDragged {
				continue
			}

			slot := i
			if drag.Active {
				slot = chrome.VisualTabSlot(i, drag.From, drag.Over)
			}
			x0 := slot*g.TabWidth - scrollX
			if !drag.Active {
				x0 = chrome.TabVisualLeft(i, activeIdx, scrollX, g, len(tabs))
			}
			if isDragged {
				x0 = drag.From*g.TabWidth - scrollX + drag.DX
				if x0 < 0 {
					x0 = 0
				}
				if g.PlusX >= g.TabWidth && x0+g.TabWidth > g.PlusX {
					x0 = g.PlusX - g.TabWidth
				}
			}
			active := t.ID == activeID
			hovered := !isDragged && i == drag.HoverIdx
			hoverStyle := hovered && !active
			showCloseGlyph := g.ShowClose && hovered && !isDragged
			tb.paintTab(buf, frameW, frameH, pal, t, x0, g.TabWidth, barH, inset, active, hoverStyle, isDragged, g.ShowClose, showCloseGlyph)
		}
	}

	if g.PlusWidth > 0 {
		tb.paintPlusButton(buf, frameW, frameH, pal, g.PlusX, g.PlusWidth, barH, inset, hoverPlus)
	}
	if statusText != "" {
		tb.paintStatus(buf, frameW, frameH, pal, g, barH, statusText)
	}
}

func (tb *TabBar) paintTab(buf []byte, frameW, frameH int, pal theme.Palette, t session.Tab, x0, w, h, inset int, active, hoverStyle, dragging, reserveClose, showCloseGlyph bool) {
	vpad := dpToPx(capsuleVPadDp, tb.Scale)
	rx0, ry0, rx1, ry1 := x0+inset, vpad, x0+w-inset, h-vpad
	if rx1 > rx0 && ry1 > ry0 {
		radius := (ry1 - ry0) / 2
		fill := chrome.GlassFill(pal.Background, chrome.GlassStyle(active, hoverStyle, dragging))
		fillC := toRGBA(fill)
		if dragging {
			fillC.A = chrome.GlassDragA
		}
		fillRoundRect(buf, frameW, rx0, ry0, rx1, ry1, radius, fillC)
	}

	fg := pal.Foreground
	switch {
	case !active && !dragging && hoverStyle:
		fg = chrome.DimFG(pal.Foreground, pal.Background, hoverFGDim)
	case !active && !dragging:
		fg = chrome.DimFG(pal.Foreground, pal.Background, inactiveFGDim)
	}

	// Title is centered in the space left after the tab's side insets —
	// and, when this tab can ever show a close ×, after its zone too (the
	// × sits near the tab's left edge, termizard-style, so a centered
	// title would otherwise run straight through it). This reservation is
	// keyed on reserveClose (whether the tab is wide enough / there's
	// more than one tab), not on showCloseGlyph (whether the × happens to
	// be visible on THIS frame) — keying it on hover would shift the
	// title sideways the instant the pointer entered the tab, which read
	// as a jarring jump rather than a hover effect.
	titleLeft := dpToPx(tabInsetDp, tb.Scale)
	titleX0 := x0 + titleLeft
	titleX1 := x0 + w - titleLeft
	edgePad := dpToPx(CloseEdgePadDp, tb.Scale)
	zoneW := dpToPx(CloseZoneWidthDp, tb.Scale)
	if reserveClose {
		reserved := x0 + edgePad + zoneW + dpToPx(titleCloseReserveDp, tb.Scale)
		if reserved > titleX0 {
			titleX0 = reserved
		}
	}
	titleWidth := titleX1 - titleX0
	if titleWidth < 0 {
		titleWidth = 0
	}
	title := truncateTitle(tabTitle(t))
	tb.drawTextCentered(buf, frameW, frameH, title, titleX0, 0, titleWidth, h, toRGBA(fg))

	if showCloseGlyph {
		closeFG := toRGBA(chrome.DimFG(fg, pal.Background, 0.15))
		ccx, ccy := x0+edgePad+zoneW/2, h/2
		armLen := dpToPx(closeArmLenDp, tb.Scale)
		if armLen < 3 {
			armLen = 3
		}
		thickness := dpToPx(closeArmThicknessDp, tb.Scale)
		if thickness < 1 {
			thickness = 1
		}
		fillDiagonalCross(buf, frameW, ccx, ccy, armLen, thickness, closeFG)
	}
}

// paintPlusButton draws the "new tab" button as a vector cross (two filled
// bars) rather than a font glyph — guarantees pixel-exact centering
// regardless of the UI font's own glyph bearing, and lets hover brighten
// both the chip and the cross consistently.
func (tb *TabBar) paintPlusButton(buf []byte, frameW, frameH int, pal theme.Palette, x0, w, h, inset int, hovered bool) {
	// The chip is a big, generously-sized circle (bigger than the tab
	// pills' own vertical inset would give) — only the cross drawn inside
	// it is kept small.
	margin := dpToPx(plusChipMarginDp, tb.Scale)
	chip := h - 2*margin
	if chip > w-2*inset {
		chip = w - 2*inset
	}
	if chip > h {
		chip = h
	}
	cx, cy := x0+w/2, h/2
	chipFactor := float32(glassPlusDefault)
	fgDim := float32(0.30)
	if hovered {
		chipFactor = glassPlusHover
		fgDim = 0.12
	}
	if chip >= 2 {
		left, top := cx-chip/2, cy-chip/2
		fillRoundRect(buf, frameW, left, top, left+chip, top+chip, chip/2, toRGBA(chrome.GlassFill(pal.Background, chipFactor)))
	}

	fg := toRGBA(chrome.DimFG(pal.Foreground, pal.Background, fgDim))
	armLen := dpToPx(plusArmLenDp, tb.Scale)
	if armLen < 3 {
		armLen = 3
	}
	thickness := dpToPx(plusArmThicknessDp, tb.Scale)
	if thickness < 1 {
		thickness = 1
	}
	fillRect(buf, frameW, cx-armLen, cy-thickness/2, cx+armLen, cy-thickness/2+thickness, fg)
	fillRect(buf, frameW, cx-thickness/2, cy-armLen, cx-thickness/2+thickness, cy+armLen, fg)
}

func (tb *TabBar) paintStatus(buf []byte, frameW, frameH int, pal theme.Palette, g chrome.Geometry, h int, text string) {
	statusX := g.PlusX + g.PlusWidth
	if statusX >= frameW {
		return
	}
	left := dpToPx(tabInsetDp, tb.Scale)
	tb.drawText(buf, frameW, frameH, text, statusX+left, 0, frameW-statusX-left, h, toRGBA(pal.Foreground))
}

func tabTitle(t session.Tab) string {
	t.Session.Term.RLock()
	title := t.Session.Term.Title()
	t.Session.Term.RUnlock()
	// Windows often leaves the console title as the shell .exe path; prefer
	// the spawn directory (cwd) so the tab shows a real path, not powershell.exe.
	if title == "" || looksLikeShellExeTitle(title) {
		if dir := strings.TrimSpace(t.Session.Dir); dir != "" {
			return dir
		}
		if title == "" {
			return fmt.Sprintf("Tab %d", t.ID+1)
		}
	}
	return title
}

func looksLikeShellExeTitle(title string) bool {
	s := strings.ReplaceAll(title, `/`, `\`)
	base := s
	if i := strings.LastIndex(s, `\`); i >= 0 {
		base = s[i+1:]
	}
	base = strings.ToLower(base)
	switch base {
	case "powershell.exe", "pwsh.exe", "cmd.exe", "windowsterminal.exe":
		return true
	}
	low := strings.ToLower(s)
	return strings.HasSuffix(base, ".exe") &&
		(strings.Contains(low, `\windows\system32\`) ||
			strings.Contains(low, `\powershell\`))
}

// truncateTitle keeps the end of long titles (paths) so the leaf segment
// stays visible: "…\System32\WindowsPowerShell".
func truncateTitle(title string) string {
	r := []rune(title)
	if len(r) <= tabTitleMaxRunes {
		return title
	}
	return "…" + string(r[len(r)-tabTitleMaxRunes:])
}
