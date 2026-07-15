// Package chrome renders geckty's window chrome — currently just the tab
// bar — as opposed to the terminal grid itself (internal/ui/grid).
package chrome

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/theme"
)

// Layout constants for the tab bar and each tab's chrome.
// Sized between earlier thin strip and Terminal.app chunky pills.
const (
	// Height is the tab bar's fixed height. Callers reserve this much
	// space above the terminal grid.
	Height = 32

	// MinTabWidth is the narrowest a tab may shrink to; below this we
	// keep MinTabWidth and scroll the strip (Terminal.app overflow).
	MinTabWidth = unit.Dp(80)
	// PlusWidth is the "+" button's fixed reserved width — Terminal.app-
	// style: carved out first, never competes with tabs.
	PlusWidth = unit.Dp(36)
	// CloseZoneWidth is the close-button hit/glyph zone width (× is
	// indented from the pill edge by CloseEdgePad).
	CloseZoneWidth = unit.Dp(20)
	// CloseEdgePad is how far the × sits in from the tab's left edge.
	CloseEdgePad = unit.Dp(8)

	tabInset         = unit.Dp(10)
	tabTitleMaxRunes = 28
	closeGlyph       = "✕"
	newTabGlyph      = "+"
	statusFontSize   = unit.Sp(12)
	tabFontSize      = unit.Sp(12)
	newTabFontSize   = unit.Sp(18)
	capsuleVPad      = unit.Dp(4)
	capsuleSegInset  = unit.Dp(3)
	inactiveFGDim    = 0.32
	hoverFGDim       = 0.10
)

// Geometry is the tab strip's computed layout for one frame.
type Geometry struct {
	TabWidth  int // 0 if there are no tabs
	PlusX     int // "+" button's left edge
	PlusWidth int // 0 if there's no room for it at all
	ShowClose bool
}

// ComputeGeometry lays out numTabs tabs plus the "+" button within
// totalWidth pixels. Widths are already in px (gtx.Dp).
//
// Tabs share the strip evenly. When equal share would drop below
// minTabWidth, width sticks at min and the strip scrolls.
func ComputeGeometry(totalWidth, numTabs, minTabWidth, plusWidth, closeZoneWidth int) Geometry {
	if totalWidth <= 0 {
		return Geometry{}
	}
	if numTabs <= 0 {
		return Geometry{PlusX: totalWidth - plusWidth, PlusWidth: plusWidth}
	}

	tabsArea := totalWidth - plusWidth
	if tabsArea < minTabWidth {
		tabsArea = totalWidth
		plusWidth = 0
	}

	tabW := tabsArea / maxInt(numTabs, 1)
	if tabW < minTabWidth {
		tabW = minTabWidth
	}
	if tabW < 1 {
		tabW = 1
	}

	return Geometry{
		TabWidth:  tabW,
		PlusX:     totalWidth - plusWidth,
		PlusWidth: plusWidth,
		ShowClose: numTabs > 1 && tabW >= closeZoneWidth*2,
	}
}

// TabsEnd is the right edge of the tab cluster (not the "+" zone).
func TabsEnd(g Geometry, numTabs int) int {
	if numTabs <= 0 || g.TabWidth <= 0 {
		return 0
	}
	return numTabs * g.TabWidth
}

// ScrollMax returns the maximal horizontal scroll offset for the tab
// strip (0 when everything fits).
func ScrollMax(g Geometry, numTabs int) int {
	total := TabsEnd(g, numTabs)
	max := total - g.PlusX
	if max < 0 {
		return 0
	}
	return max
}

// ActiveTabPin mirrors termizard: when tabs overflow and the active tab
// would be cut by scroll, keep it visually pinned at the left or right
// edge of the visible strip.
func ActiveTabPin(activeIdx, numTabs, tabW, tabsAreaW, scrollX int) (pinLeft, pinRight bool) {
	if activeIdx < 0 || activeIdx >= numTabs || tabW <= 0 {
		return false, false
	}
	if numTabs*tabW <= tabsAreaW+2 {
		return false, false
	}
	tabLeft := activeIdx * tabW
	tabRight := tabLeft + tabW
	cutLeft := tabLeft < scrollX-1
	cutRight := tabRight > scrollX+tabsAreaW+1
	switch {
	case cutLeft && !cutRight:
		return true, false
	case cutRight && !cutLeft:
		return false, true
	case cutLeft && cutRight:
		return true, false
	}
	return false, false
}

func clampScroll(scroll, max int) int {
	if scroll < 0 {
		return 0
	}
	if scroll > max {
		return max
	}
	return scroll
}

// TabAt maps a physical X position to a tab index, or -1 if x is outside
// the tab cluster (empty gap, "+", or past the strip).
func TabAt(x int, g Geometry, numTabs int) int {
	end := TabsEnd(g, numTabs)
	if numTabs <= 0 || g.TabWidth <= 0 || x < 0 || x >= end {
		return -1
	}
	idx := x / g.TabWidth
	if idx >= numTabs {
		return -1
	}
	return idx
}

// TabAtScrolled maps x to tab index with horizontal strip scroll applied.
func TabAtScrolled(x, scroll int, g Geometry, numTabs int) int {
	if x < 0 || x >= g.PlusX {
		return -1
	}
	return TabAt(x+scroll, g, numTabs)
}

// TabAtScrolledPinned maps x to index with scroll + sticky active pin.
func TabAtScrolledPinned(x, scroll int, g Geometry, numTabs, activeIdx int) int {
	if x < 0 || x >= g.PlusX {
		return -1
	}
	pinL, pinR := ActiveTabPin(activeIdx, numTabs, g.TabWidth, g.PlusX, scroll)
	if pinL && x < g.TabWidth {
		return activeIdx
	}
	if pinR {
		pinX := g.PlusX - g.TabWidth
		if pinX < 0 {
			pinX = 0
		}
		if x >= pinX {
			return activeIdx
		}
	}
	return TabAtScrolled(x, scroll, g, numTabs)
}

// DropOverlapFrac is how much of a tab the dragged pill must cover before
// the underlying tab slides out of the way (45%).
const DropOverlapFrac = 0.45

// DropIndex maps a pointer X to the drop-target slot during a drag,
// using each slot's midpoint (fallback / tests).
func DropIndex(x int, g Geometry, numTabs int) int {
	if numTabs <= 0 || g.TabWidth <= 0 {
		return -1
	}
	if x < 0 {
		return 0
	}
	end := TabsEnd(g, numTabs)
	if x >= end {
		return numTabs - 1
	}
	over := 0
	for i := 0; i < numTabs; i++ {
		mid := i*g.TabWidth + g.TabWidth/2
		if x < mid {
			return i
		}
		over = i
	}
	return over
}

// DropIndexScrolled maps x to drag drop slot with horizontal scroll.
func DropIndexScrolled(x, scroll int, g Geometry, numTabs int) int {
	if x < 0 {
		return 0
	}
	if x >= g.PlusX {
		return numTabs - 1
	}
	return DropIndex(x+scroll, g, numTabs)
}

// DropIndexByOverlap picks the drop slot from the dragged pill's left edge:
// the tab under the drag only slides once overlap ≥ DropOverlapFrac of
// TabWidth. Until then currentOver (usually from) is kept.
func DropIndexByOverlap(dragLeft, scroll, tabW, numTabs, from, currentOver int) int {
	if numTabs <= 0 || tabW <= 0 {
		return -1
	}
	if currentOver < 0 || currentOver >= numTabs {
		currentOver = from
	}
	left := dragLeft + scroll
	right := left + tabW
	thresh := int(float64(tabW)*DropOverlapFrac + 0.5)
	if thresh < 1 {
		thresh = 1
	}
	bestIdx, bestOv := currentOver, -1
	for i := 0; i < numTabs; i++ {
		if i == from {
			continue
		}
		sL, sR := i*tabW, (i+1)*tabW
		ov := minInt(right, sR) - maxInt(left, sL)
		if ov > bestOv {
			bestOv = ov
			bestIdx = i
		}
	}
	if bestOv >= thresh {
		return bestIdx
	}
	return from
}

// TabVisualLeft returns the painted left X of idx for current scroll and
// active pin state (non-drag render/hit-test path).
func TabVisualLeft(idx, activeIdx, scroll int, g Geometry, numTabs int) int {
	x := idx*g.TabWidth - scroll
	if idx == activeIdx {
		if pinL, pinR := ActiveTabPin(activeIdx, numTabs, g.TabWidth, g.PlusX, scroll); pinL {
			x = 0
		} else if pinR {
			x = g.PlusX - g.TabWidth
			if x < 0 {
				x = 0
			}
		}
	}
	return x
}

// IsPlusHit reports whether x falls within the "+" button's zone.
func IsPlusHit(x int, g Geometry) bool {
	return g.PlusWidth > 0 && x >= g.PlusX && x < g.PlusX+g.PlusWidth
}

// IsCloseHit reports whether localX — x relative to the start of the tab
// it falls within — is inside that tab's close-button zone (inset from
// the left edge by closeEdgePad).
func IsCloseHit(localX int, g Geometry, closeZoneWidth, closeEdgePad int) bool {
	if !g.ShowClose || closeZoneWidth <= 0 {
		return false
	}
	if closeEdgePad < 0 {
		closeEdgePad = 0
	}
	return localX >= closeEdgePad && localX < closeEdgePad+closeZoneWidth
}

// VisualTabSlot returns the preview layout slot for tab index while a
// drag is in progress — other tabs slide to make room. Ported from
// termizard's visualTabSlot.
func VisualTabSlot(index, from, over int) int {
	if from == over || index == from {
		return index
	}
	if from < over && index > from && index <= over {
		return index - 1
	}
	if from > over && index >= over && index < from {
		return index + 1
	}
	return index
}

// TabBar renders the tab strip. Interaction is driven by the caller via
// the hit-test helpers above (see app.go) — no per-tab Clickable/Drag.
type TabBar struct{}

// NewTabBar returns a TabBar.
func NewTabBar() *TabBar {
	return &TabBar{}
}

// DragVisual describes an in-progress drag-reorder for rendering:
// the dragged tab follows the pointer (DX), and the others preview their
// shifted slots before MoveTo commits on release.
type DragVisual struct {
	Active   bool
	From     int // source index in Tabs()
	Over     int // current drop-target index
	DX       int // pointer delta from press, in px
	ScrollX  int
	TabID    int // stable id of the dragged tab
	HoverIdx int // inactive tab under pointer; -1 = none
}

// Layout draws the tab bar.
func (tb *TabBar) Layout(gtx layout.Context, th *material.Theme, pal theme.Palette, tabs []session.Tab, activeID int, statusText string, drag DragVisual) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(Height))
	gtx.Constraints = layout.Exact(size)
	barBG := glassFill(pal.Background, glassBarLift)
	paint.FillShape(gtx.Ops, barBG, clip.Rect{Max: size}.Op())

	g := ComputeGeometry(size.X, len(tabs), gtx.Dp(MinTabWidth), gtx.Dp(PlusWidth), gtx.Dp(CloseZoneWidth))
	h := size.Y
	inset := gtx.Dp(capsuleSegInset)
	scrollX := clampScroll(drag.ScrollX, ScrollMax(g, len(tabs)))
	activeIdx := -1
	for i := range tabs {
		if tabs[i].ID == activeID {
			activeIdx = i
			break
		}
	}
	tabsClip := clip.Rect{Max: image.Pt(g.PlusX, h)}.Push(gtx.Ops)

	// Pass 0: non-dragged tabs (they slide under). Pass 1: dragged pill on top.
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
				slot = VisualTabSlot(i, drag.From, drag.Over)
			}
			x0 := slot*g.TabWidth - scrollX
			if !drag.Active {
				x0 = TabVisualLeft(i, activeIdx, scrollX, g, len(tabs))
			}
			if isDragged {
				// Follow the pointer from the press origin (from slot).
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
			// × only while hovered (Terminal.app); still reserve close geometry.
			showCloseGlyph := g.ShowClose && hovered && !isDragged
			tb.paintTab(gtx, th, pal, t, x0, g.TabWidth, h, inset, active, hoverStyle, isDragged, g.ShowClose, showCloseGlyph)
		}
	}
	tabsClip.Pop()

	if g.PlusWidth > 0 {
		tb.paintPlusButton(gtx, th, pal, g.PlusX, g.PlusWidth, h, inset)
	}

	if statusText != "" {
		tb.paintStatus(gtx, th, pal, g, size, h, statusText)
	}

	return layout.Dimensions{Size: size}
}

func (tb *TabBar) paintTab(gtx layout.Context, th *material.Theme, pal theme.Palette, t session.Tab, x0, w, h, inset int, active, hoverStyle, dragging, reserveClose, showCloseGlyph bool) {
	vpad := gtx.Dp(capsuleVPad)
	rect := image.Rect(x0+inset, vpad, x0+w-inset, h-vpad)
	if !rect.Empty() {
		radius := rect.Dy() / 2
		alpha := uint8(0xff)
		if dragging {
			alpha = glassDragA
		}
		paintGlassCapsule(gtx.Ops, pal.Background, rect, radius, glassStyle(active, hoverStyle, dragging), alpha)
	}

	fg := pal.Foreground
	switch {
	case !active && !dragging && hoverStyle:
		fg = dimFG(pal.Foreground, pal.Background, hoverFGDim)
	case !active && !dragging:
		fg = dimFG(pal.Foreground, pal.Background, inactiveFGDim)
	}

	off := op.Offset(image.Pt(x0, 0)).Push(gtx.Ops)
	clipStack := clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops)

	// Title stays centered with normal insets; × overlays on hover so the
	// label doesn't jump when the close glyph appears.
	_ = reserveClose
	titleLeft := gtx.Dp(tabInset)
	titleWidth := w - 2*titleLeft
	if titleWidth < 0 {
		titleWidth = 0
	}
	titleOff := op.Offset(image.Pt(titleLeft, 0)).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(image.Pt(titleWidth, h))
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(th, tabFontSize, truncateTitle(tabTitle(t)))
		label.MaxLines = 1
		label.Alignment = text.End // keep path leaf visible when width-clipped
		label.Color = fg
		label.Font.Typeface = theme.MonospaceTypeface
		if active || dragging {
			label.Font.Weight = font.Medium
		}
		return label.Layout(gtx)
	})
	titleOff.Pop()

	if showCloseGlyph {
		edgePad := gtx.Dp(CloseEdgePad)
		closeOff := op.Offset(image.Pt(edgePad, 0)).Push(gtx.Ops)
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(CloseZoneWidth), h))
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(th, tabFontSize, closeGlyph)
			label.Color = dimFG(fg, pal.Background, 0.15)
			label.Font.Typeface = theme.MonospaceTypeface
			return label.Layout(gtx)
		})
		closeOff.Pop()
	}

	clipStack.Pop()
	off.Pop()
}

func (tb *TabBar) paintPlusButton(gtx layout.Context, th *material.Theme, pal theme.Palette, x0, w, h, inset int) {
	// Keep the + capsule at the pre–Height-shrink size (tabs lost 2px; + did not).
	vpad := gtx.Dp(capsuleVPad)
	chip := h - 2*vpad + 2
	if chip > w-2*inset {
		chip = w - 2*inset
	}
	if chip > h {
		chip = h
	}
	if chip >= 2 {
		top := (h - chip) / 2
		cx := x0 + (w-chip)/2
		rect := image.Rect(cx, top, cx+chip, top+chip)
		paintGlassCapsule(gtx.Ops, pal.Background, rect, chip/2, glassActive, 0xff)
	}

	off := op.Offset(image.Pt(x0, 0)).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(image.Pt(w, h))
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(th, newTabFontSize, newTabGlyph)
		label.Color = dimFG(pal.Foreground, pal.Background, 0.12)
		label.Font.Typeface = theme.MonospaceTypeface
		return label.Layout(gtx)
	})
	off.Pop()
}

func (tb *TabBar) paintStatus(gtx layout.Context, th *material.Theme, pal theme.Palette, g Geometry, size image.Point, h int, text string) {
	statusX := g.PlusX + g.PlusWidth
	if statusX >= size.X {
		return
	}
	off := op.Offset(image.Pt(statusX, 0)).Push(gtx.Ops)
	gtx.Constraints = layout.Exact(image.Pt(size.X-statusX, h))
	layout.Inset{Left: tabInset}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(th, statusFontSize, text)
		label.Color = pal.Foreground
		label.Font.Typeface = theme.MonospaceTypeface
		return label.Layout(gtx)
	})
	off.Pop()
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
