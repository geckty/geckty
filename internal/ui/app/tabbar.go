package app

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"golang.org/x/image/font"

	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
	"github.com/geckty/geckty/internal/ui/raster"
	"github.com/geckty/geckty/internal/ui/theme"
)

// Tab-bar sizing, in logical (DPI-independent) pixels — same values as the
// chrome.Height/MinTabWidth/etc, converted to device pixels
// by the caller (app.go) via dpToPx before reaching chrome.ComputeGeometry,
// which is pure-Go and reused unchanged from the old chrome package.
const (
	TabBarHeightDp   = chrome.Height         // total tab strip height
	MinTabWidthDp    = chrome.MinTabWidth    // narrowest a tab pill may shrink to before tabs scroll instead
	PlusWidthDp      = chrome.PlusWidth      // "+" button's reserved slot width
	CloseZoneWidthDp = chrome.CloseZoneWidth // hit-zone width for a tab's close ×, from its left edge
	CloseEdgePadDp   = 8                     // gap between a tab's left edge and its close-zone
	tabInsetDp       = 10                    // horizontal gap between a tab's edge and its title text
	capsuleVPadDp    = 4                     // vertical gap between the bar edge and a tab pill's rounded rect
	capsuleSegInDp   = 3                     // horizontal gap between adjacent tab pills

	tabTitleMaxRunes = 28 // longest title before truncateTitle elides its middle

	titleCloseReserveDp = 4 // gap reserved after the close-zone before centered title text may start
	closeArmLenDp       = 4 // close "×" per-arm half-length
	closeArmThicknessDp = 1 // close "×" stroke thickness
	plusChipMarginDp    = 2 // gap between the "+" chip circle and the bar edges
	plusArmLenDp        = 5 // "+" arm half-length — 1px larger than closeArmLenDp since the "+" sits in a bigger chip and reads too small at the same size
	plusArmThicknessDp  = 2 // "+" stroke — thicker than closeArmThicknessDp to stay balanced against the larger arm length

	commandDotRadiusDp = 3           // "command running/finished" indicator dot radius, see commandIndicatorColor
	glassDragBlurDp    = 3           // liquid-glass blur under a dragged tab pill
	tabSepHeightFrac   = 0.40        // vertical stick height as fraction of barH
	tabSepAlpha        = uint8(0x38) // faint divider between non-pill tabs
)

// commandIndicatorFade is how long a finished command's success/failure dot
// (see commandIndicatorColor) stays visible on its tab before fading back to
// nothing — a permanent per-tab badge that accumulates forever would be
// noise; a command that's still running has no such timeout, it stays lit
// until OSC 133;D fires.
const commandIndicatorFade = 3 * time.Second

// dpToPx converts a logical (DPI-independent) pixel size to device pixels
// at the given scale factor, rounding half up rather than truncating so
// small UI elements (a 1dp stroke, a 2dp margin) don't collapse to 0px at
// fractional scale factors.
func dpToPx(dp int, scale float64) int { return int(float64(dp)*scale + 0.5) }

// tabBarShowTabs reports whether the tab strip (the row of tab pills)
// should be shown for the current tab count, per cfg.TabBar. Callers that
// need "0 tabs" for chrome.ComputeGeometry/hit-testing when this is false
// just nil out the tabs slice they pass down — chrome's own numTabs<=0
// path already draws/hit-tests as "no tabs", so no separate code path is
// needed here.
func (s *uiState) tabBarShowTabs() bool {
	return tabBarSectionVisible(s.cfg.TabBar.Hidden, s.cfg.TabBar.ShowThreshold, len(s.mgr.Tabs()))
}

// tabBarShowPlus reports whether the "+" new-tab button should be shown
// for the current tab count, per cfg.TabBar.PlusButton — independent of
// tabBarShowTabs (see TabBarConfig's doc comment for why they're separate
// knobs), but still gated by the tab bar's own top-level Hidden.
func (s *uiState) tabBarShowPlus() bool {
	hidden := s.cfg.TabBar.Hidden || s.cfg.TabBar.PlusButton.Hidden
	return tabBarSectionVisible(hidden, s.cfg.TabBar.PlusButton.ShowThreshold, len(s.mgr.Tabs()))
}

func tabBarSectionVisible(hidden bool, threshold, numTabs int) bool {
	if hidden {
		return false
	}
	if threshold < 1 {
		threshold = 1
	}
	return numTabs >= threshold
}

// tabBarHeightPx is the tab bar's actual on-screen height in device
// pixels this frame: 0 when neither the tab strip nor the "+" button
// should show (collapsing the row entirely so the grid reclaims that
// space), otherwise the fixed TabBarHeightDp. Every place that needs to
// know "is this device-pixel Y coordinate inside the tab bar" — onDraw's
// layout and pointer.go's hit-testing alike — must go through this rather
// than recomputing dpToPx(TabBarHeightDp, ...) directly, or a hidden bar
// would still swallow clicks/scrolls in what's now grid space.
func (s *uiState) tabBarHeightPx() int {
	if !s.tabBarShowTabs() && !s.tabBarShowPlus() {
		return 0
	}
	return dpToPx(TabBarHeightDp, s.scale)
}

// TabBar renders geckty's tab strip as direct pixel writes, reusing the old
// chrome package's pure-Go geometry/hit-test functions (ComputeGeometry,
// TabAtScrolledPinned, DropIndexByOverlap, VisualTabSlot, etc.) and its
// glass/dim color-blend helpers (GlassStyle, GlassFill, DimFG) — all
// unchanged — this file adds only the paint side.
type TabBar struct {
	Face   font.Face // tab-bar UI font face, at its own (smaller) size
	Ascent int
	Scale  float64 // device pixels per logical (dp) pixel

	atlas *raster.GlyphAtlas
}

// NewTabBar returns an empty TabBar. Callers must set Face/Ascent/Scale
// (see app.go's ensureFonts) before the first Layout call — an unset Face
// makes drawText/drawTextCentered/drawTextAt no-ops.
func NewTabBar() *TabBar { return &TabBar{} }

func (tb *TabBar) ensureAtlas() {
	if !tb.atlas.Valid(tb.Face, tb.Ascent) {
		tb.atlas = raster.NewGlyphAtlas(tb.Face, tb.Ascent)
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
		if e, ok := tb.atlas.Get(r); ok {
			dr := e.DrRel.Add(image.Pt(x, dotY-tb.Ascent))
			raster.BlitGlyphClipped(buf, frameW, frameH, dr, e.Mask, e.MaskP, fg, clipX0, y0, clipX0+w, y0+h)
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
// or any tab hover state. showPlus controls the "+" button's own
// visibility (see TabBarConfig.PlusButton) — tabs is the caller's whole
// visibility lever for the tab strip itself: pass nil to paint/reserve no
// tab pills regardless of how many sessions are actually open.
func (tb *TabBar) Layout(buf []byte, frameW, frameH, barH int, thm theme.Theme, tabs []session.Tab, activeID int, statusText string, drag chrome.DragVisual, hoverPlus, showPlus bool) {
	pal := thm.Palette
	barBG := toRGBA(pal.TabBarBG)
	raster.FillRect(buf, frameW, 0, 0, frameW, barH, barBG)

	minTabW := dpToPx(MinTabWidthDp, tb.Scale)
	plusW := dpToPx(PlusWidthDp, tb.Scale)
	if !showPlus {
		plusW = 0
	}
	closeZoneW := dpToPx(CloseZoneWidthDp, tb.Scale)
	inset := dpToPx(capsuleSegInDp, tb.Scale)

	g := chrome.ComputeGeometry(frameW, len(tabs), minTabW, plusW, closeZoneW)
	scrollX := drag.ScrollX
	if scrollMax := chrome.ScrollMax(g, len(tabs)); scrollX > scrollMax {
		scrollX = scrollMax
	}
	if scrollX < 0 {
		scrollX = 0
	}
	// Subtle glass hairline under the strip — reads as a frosted edge
	// without a heavy separator.
	if barH > 0 {
		if rim, ok := glassRimColor(thm.Glass, false); ok {
			rim.A = uint8(float32(rim.A) * 0.55)
			raster.BlendRect(buf, frameW, 0, barH-1, frameW, barH, rim)
		}
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
			tb.paintTab(buf, frameW, frameH, thm, t, x0, g.TabWidth, barH, inset, active, hoverStyle, isDragged, g.ShowClose, showCloseGlyph)
		}
	}

	// Terminal.app-style sticks between plain (non-pill) tabs. Skip a
	// divider that would touch an active/hover/drag pill.
	if !drag.Active && len(tabs) > 1 {
		tb.paintTabSeparators(buf, frameW, thm, tabs, activeID, drag.HoverIdx, scrollX, activeIdx, g, barH)
	}

	if g.PlusWidth > 0 {
		tb.paintPlusButton(buf, frameW, thm, g.PlusX, g.PlusWidth, barH, inset, hoverPlus)
	}
	if statusText != "" {
		tb.paintStatus(buf, frameW, frameH, pal, g, barH, statusText)
	}
}

// paintTabSeparators draws faint vertical sticks between adjacent tabs that
// are both in the plain (title-only) state. A stick next to an active or
// hovered pill is omitted so the capsule reads cleanly.
func (tb *TabBar) paintTabSeparators(buf []byte, frameW int, thm theme.Theme, tabs []session.Tab, activeID, hoverIdx, scrollX, activeIdx int, g chrome.Geometry, barH int) {
	pillAt := func(i int) bool {
		if i < 0 || i >= len(tabs) {
			return false
		}
		if tabs[i].ID == activeID {
			return true
		}
		return i == hoverIdx
	}
	sepH := int(float64(barH) * tabSepHeightFrac)
	if sepH < 8 {
		sepH = 8
	}
	if sepH > barH-4 {
		sepH = barH - 4
	}
	y0 := (barH - sepH) / 2
	c := toRGBA(chrome.DimFG(thm.Palette.Foreground, thm.Palette.Background, 0.55))
	c.A = tabSepAlpha
	for i := 1; i < len(tabs); i++ {
		if pillAt(i-1) || pillAt(i) {
			continue
		}
		x := chrome.TabVisualLeft(i, activeIdx, scrollX, g, len(tabs))
		if x <= 0 || x >= frameW {
			continue
		}
		raster.BlendRect(buf, frameW, x, y0, x+1, y0+sepH, c)
	}
}

func (tb *TabBar) paintTab(buf []byte, frameW, frameH int, thm theme.Theme, t session.Tab, x0, w, h, inset int, active, hoverStyle, dragging, reserveClose, showCloseGlyph bool) {
	pal := thm.Palette
	showPill := active || hoverStyle || dragging
	vpad := dpToPx(capsuleVPadDp, tb.Scale)
	rx0, ry0, rx1, ry1 := x0+inset, vpad, x0+w-inset, h-vpad
	if showPill && rx1 > rx0 && ry1 > ry0 {
		radius := (ry1 - ry0) / 2
		fillC := toRGBA(pal.TabFillGlass(thm.Glass, active || dragging, hoverStyle && !dragging, false))
		if dragging {
			// Liquid glass: warp+blur underlay, same RGB tint — titles
			// under the pin distort instead of vanishing into fog.
			tint := fillC
			tint.A = chrome.GlassDragA
			blurR := dpToPx(glassDragBlurDp, tb.Scale)
			if blurR < 2 {
				blurR = 2
			}
			frostGlassRoundRect(buf, frameW, rx0, ry0, rx1, ry1, radius, blurR, tint)
		} else {
			fillC.A = glassFillAlpha(thm.Glass)
			raster.FillRoundRect(buf, frameW, rx0, ry0, rx1, ry1, radius, fillC)
		}
		if rim, ok := glassRimColor(thm.Glass, true); ok {
			raster.StrokeRoundRect(buf, frameW, rx0, ry0, rx1, ry1, radius, rim)
		}
	}

	fg := pal.TabTitleFG(active || dragging, hoverStyle && !dragging, false)

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

	switch {
	case showCloseGlyph:
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
		raster.FillDiagonalCross(buf, frameW, ccx, ccy, armLen, thickness, closeFG)
	case reserveClose && thm.UI.CommandDotEnabled:
		// Optional OSC 133 status dot in the close zone (off by default —
		// window border + loud chrome were too noisy; enable via theme).
		if dot, ok := commandIndicatorColor(t.Session, thm); ok {
			cx, cy := x0+edgePad+zoneW/2, h/2
			r := dpToPx(commandDotRadiusDp, tb.Scale)
			if r < 2 {
				r = 2
			}
			raster.FillRoundRect(buf, frameW, cx-r, cy-r, cx+r, cy+r, r, dot)
		}
	}
}

// glassFillAlpha maps theme FillAlpha (0–1) to a uint8; falls back to the
// chrome constant when unset / zero would make pills invisible.
func glassFillAlpha(g theme.GlassParams) uint8 {
	a := g.FillAlpha
	if a <= 0 {
		return chrome.GlassFillA
	}
	if a > 1 {
		a = 1
	}
	return uint8(a*255 + 0.5)
}

// glassRimColor is a translucent white-ish outline for frosted glass edges.
// Hover/active bumps alpha slightly so the rim reads without a fill glow.
// ok is false when rim_alpha is 0 (explicitly disabled).
func glassRimColor(g theme.GlassParams, emphasize bool) (color.RGBA, bool) {
	a := g.RimAlpha
	if a <= 0 {
		return color.RGBA{}, false
	}
	lift := g.Rim
	if lift <= 0 {
		lift = chrome.GlassRim
	}
	if lift > 1 {
		lift = 1
	}
	if a > 1 {
		a = 1
	}
	if emphasize {
		a *= 1.25
		if a > 0.72 {
			a = 0.72
		}
	}
	v := uint8(lift*255 + 0.5)
	return color.RGBA{R: v, G: v, B: v, A: uint8(a*255 + 0.5)}, true
}

// commandIndicatorColor reports the color a tab's OSC 133 status indicator
// should paint — its tab-bar pill dot (see paintTab) and the active tab's
// window-border highlight (see paintCommandBorder) share this — and false
// when nothing should be drawn: no command has run yet, or the last one
// finished more than commandIndicatorFade ago.
func commandIndicatorColor(sess *session.Session, thm theme.Theme) (color.RGBA, bool) {
	sess.Term.RLock()
	cmd := sess.Term.CommandState()
	sess.Term.RUnlock()

	switch {
	case cmd.Running:
		return toRGBA(thm.UI.CommandRunning), true
	case cmd.ExitCode != nil && time.Since(cmd.FinishedAt) < commandIndicatorFade:
		if *cmd.ExitCode == 0 {
			return toRGBA(thm.UI.CommandSuccess), true
		}
		return toRGBA(thm.UI.CommandFailed), true
	default:
		return color.RGBA{}, false
	}
}

// paintPlusButton draws the "new tab" button as a vector cross (two filled
// bars) rather than a font glyph — guarantees pixel-exact centering
// regardless of the UI font's own glyph bearing, and lets hover brighten
// both the chip and the cross consistently without a loud fill glow.
func (tb *TabBar) paintPlusButton(buf []byte, frameW int, thm theme.Theme, x0, w, h, inset int, hovered bool) {
	pal := thm.Palette
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
	fgDim := float32(0.30)
	plusBG := pal.PlusButtonBG
	if hovered {
		fgDim = 0.18
		plusBG = chrome.GlassFill(pal.Background, thm.Glass.PlusHover)
	}
	if chip >= 2 {
		left, top := cx-chip/2, cy-chip/2
		fillC := toRGBA(plusBG)
		fillC.A = glassFillAlpha(thm.Glass)
		raster.FillRoundRect(buf, frameW, left, top, left+chip, top+chip, chip/2, fillC)
		if rim, ok := glassRimColor(thm.Glass, hovered); ok {
			raster.StrokeRoundRect(buf, frameW, left, top, left+chip, top+chip, chip/2, rim)
		}
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
	raster.FillRect(buf, frameW, cx-armLen, cy-thickness/2, cx+armLen, cy-thickness/2+thickness, fg)
	raster.FillRect(buf, frameW, cx-thickness/2, cy-armLen, cx-thickness/2+thickness, cy+armLen, fg)
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
	if _, after, ok := strings.CutLast(s, `\`); ok {
		base = after
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
