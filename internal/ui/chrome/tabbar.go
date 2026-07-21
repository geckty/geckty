// Package chrome holds geckty's tab-bar geometry and hit-testing —
// toolkit-agnostic pure functions the UI backend (internal/ui/gogpu)
// paints against.
package chrome

// Height is the tab bar's fixed height, in logical pixels. Callers reserve
// this much space above the terminal grid.
const Height = 32

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
	overflow := total - g.PlusX
	if overflow < 0 {
		return 0
	}
	return overflow
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
