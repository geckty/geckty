// Package theme resolves geckty's configured colors and emu.Color values
// into concrete RGB values for painting.
package theme

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/vt/emu"
)

// Theme is the full runtime theme: VT palette + UI chrome tokens + glass
// blend parameters. Built from config via New.
type Theme struct {
	Palette Palette
	UI      UITokens
	Glass   GlassParams
}

// UITokens holds paintable colors for UX chrome that is not part of the
// VT ANSI palette (visual bell, scrollbar, indicators, overlays).
type UITokens struct {
	VisualBell           color.NRGBA
	ScrollbarTrack       color.NRGBA
	ScrollbarThumb       color.NRGBA
	URLUnderline         color.NRGBA
	SearchMatch          color.NRGBA
	HintLabelBG          color.NRGBA
	HintLabelFG          color.NRGBA
	PaneFocusBorder      color.NRGBA
	CommandRunning       color.NRGBA
	CommandSuccess       color.NRGBA
	CommandFailed        color.NRGBA
	CommandBorderEnabled bool
	CommandDotEnabled    bool
	ContentBrackets      color.NRGBA // A==0 disables; top-of-grid [ ] marks
}

// GlassParams are blend factors for derived tab chrome fills and rim.
type GlassParams struct {
	BarLift   float32
	Inactive  float32
	Hover     float32
	Active    float32
	Drag      float32
	PlusHover float32
	Rim       float32 // edge highlight lift toward white (0–1)
	RimAlpha  float32 // outline opacity (0–1)
	FillAlpha float32 // frosted pill opacity (0–1)
}

// DefaultGlass returns the glass theme's blend defaults (config.Glass*).
func DefaultGlass() GlassParams {
	return GlassParams{
		BarLift:   float32(config.GlassBarLift),
		Inactive:  float32(config.GlassInactive),
		Hover:     float32(config.GlassHover),
		Active:    float32(config.GlassActive),
		Drag:      float32(config.GlassDrag),
		PlusHover: float32(config.GlassPlusHover),
		Rim:       float32(config.GlassRim),
		RimAlpha:  float32(config.GlassRimAlpha),
		FillAlpha: float32(config.GlassFillAlpha),
	}
}

// New builds a Theme from a loaded Config (colors + ui already resolved).
func New(cfg *config.Config) (Theme, error) {
	glass := glassFromConfig(cfg.UI.Glass)
	pal, err := NewPalette(cfg.Colors, glass)
	if err != nil {
		return Theme{}, err
	}
	if err := ApplyCursorColor(&pal, cfg.Cursor.Color); err != nil {
		return Theme{}, err
	}
	ui, err := newUITokens(cfg.UI, pal)
	if err != nil {
		return Theme{}, err
	}
	return Theme{Palette: pal, UI: ui, Glass: glass}, nil
}

func glassFromConfig(g config.GlassConfig) GlassParams {
	out := DefaultGlass()
	if g.BarLift != nil {
		out.BarLift = float32(*g.BarLift)
	}
	if g.Inactive != nil {
		out.Inactive = float32(*g.Inactive)
	}
	if g.Hover != nil {
		out.Hover = float32(*g.Hover)
	}
	if g.Active != nil {
		out.Active = float32(*g.Active)
	}
	if g.Drag != nil {
		out.Drag = float32(*g.Drag)
	}
	if g.PlusHover != nil {
		out.PlusHover = float32(*g.PlusHover)
	}
	if g.Rim != nil {
		out.Rim = float32(*g.Rim)
	}
	if g.RimAlpha != nil {
		out.RimAlpha = float32(*g.RimAlpha)
	}
	if g.FillAlpha != nil {
		out.FillAlpha = float32(*g.FillAlpha)
	}
	return out
}

func newUITokens(cfg config.UIConfig, pal Palette) (UITokens, error) {
	var t UITokens
	var err error
	parseRGBA := func(hex, key string, fallback color.NRGBA) (color.NRGBA, error) {
		if hex == "" {
			return fallback, nil
		}
		c, err := parseHexAlpha(hex)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("ui.%s: %w", key, err)
		}
		return c, nil
	}

	if t.VisualBell, err = parseRGBA(cfg.VisualBell, "visual_bell",
		color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55}); err != nil {
		return UITokens{}, err
	}
	if t.ScrollbarTrack, err = parseRGBA(cfg.ScrollbarTrack, "scrollbar_track",
		color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x28}); err != nil {
		return UITokens{}, err
	}
	if t.ScrollbarThumb, err = parseRGBA(cfg.ScrollbarThumb, "scrollbar_thumb",
		color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x70}); err != nil {
		return UITokens{}, err
	}
	ansiBrightBlue := withAlpha(pal.ANSI[ansiBrightBlueIdx], 0xff)
	ansiBrightYellow := withAlpha(pal.ANSI[ansiBrightYellowIdx], 0xff)
	ansiCyan := withAlpha(pal.ANSI[ansiCyanIdx], 0xff)
	ansiGreen := withAlpha(pal.ANSI[ansiGreenIdx], 0xff)
	ansiRed := withAlpha(pal.ANSI[ansiRedIdx], 0xff)

	if t.URLUnderline, err = parseRGBA(cfg.URLUnderline, "url_underline", ansiBrightBlue); err != nil {
		return UITokens{}, err
	}
	if t.SearchMatch, err = parseRGBA(cfg.SearchMatch, "search_match", withAlpha(ansiBrightYellow, 0x90)); err != nil {
		return UITokens{}, err
	}
	if t.HintLabelBG, err = parseRGBA(cfg.HintLabelBG, "hint_label_bg", ansiBrightYellow); err != nil {
		return UITokens{}, err
	}
	if t.HintLabelFG, err = parseRGBA(cfg.HintLabelFG, "hint_label_fg", pal.Background); err != nil {
		return UITokens{}, err
	}
	if t.PaneFocusBorder, err = parseRGBA(cfg.PaneFocusBorder, "pane_focus_border", withAlpha(ansiBrightBlue, 0xaa)); err != nil {
		return UITokens{}, err
	}
	if t.CommandRunning, err = parseRGBA(cfg.CommandRunning, "command_running", ansiCyan); err != nil {
		return UITokens{}, err
	}
	if t.CommandSuccess, err = parseRGBA(cfg.CommandSuccess, "command_success", ansiGreen); err != nil {
		return UITokens{}, err
	}
	if t.CommandFailed, err = parseRGBA(cfg.CommandFailed, "command_failed", ansiRed); err != nil {
		return UITokens{}, err
	}
	if cfg.CommandBorderEnabled != nil {
		t.CommandBorderEnabled = *cfg.CommandBorderEnabled
	}
	if cfg.CommandDotEnabled != nil {
		t.CommandDotEnabled = *cfg.CommandDotEnabled
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ContentBrackets)) {
	case "", config.ContentBracketsOff, "none", "false":
		if cfg.ContentBrackets == "" {
			// Unset → soft default (glass chrome).
			t.ContentBrackets = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55}
		} else {
			t.ContentBrackets = color.NRGBA{} // explicitly disabled
		}
	default:
		if t.ContentBrackets, err = parseRGBA(cfg.ContentBrackets, "content_brackets",
			color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55}); err != nil {
			return UITokens{}, err
		}
	}
	return t, nil
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// DefaultSelection is the fallback selection highlight when colors.selection*
// are unset (matches glass theme #525252).
var DefaultSelection = color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff}

// Standard ANSI palette indices used as semantic UI fallbacks when [ui]
// tokens are empty.
const (
	ansiRedIdx          = 1
	ansiGreenIdx        = 2
	ansiCyanIdx         = 6
	ansiBrightYellowIdx = 11
	ansiBrightBlueIdx   = 12
)

// Palette maps emu.Color values and chrome slots to paintable colors.
type Palette struct {
	Foreground  color.NRGBA
	Background  color.NRGBA
	Selection   color.NRGBA // selection background (legacy name)
	SelectionFG color.NRGBA
	Cursor      color.NRGBA
	ANSI        [16]color.NRGBA

	TabBarBG      color.NRGBA
	ActiveTabFG   color.NRGBA
	ActiveTabBG   color.NRGBA
	InactiveTabFG color.NRGBA
	InactiveTabBG color.NRGBA
	HoverTabBG    color.NRGBA
	PlusButtonBG  color.NRGBA
}

// NewPalette parses cfg's hex color strings into a Palette. Empty chrome
// keys are derived with glass blends from Background/Foreground so a
// palette-only config keeps the previous look.
func NewPalette(cfg config.ColorsConfig, glass ...GlassParams) (Palette, error) {
	g := DefaultGlass()
	if len(glass) > 0 {
		g = glass[0]
	}
	var p Palette
	var err error
	if p.Foreground, err = parseHex(cfg.Foreground); err != nil {
		return Palette{}, fmt.Errorf("colors.foreground: %w", err)
	}
	if p.Background, err = parseHex(cfg.Background); err != nil {
		return Palette{}, fmt.Errorf("colors.background: %w", err)
	}

	selBG := cfg.SelectionBackground
	if selBG == "" {
		selBG = cfg.Selection
	}
	if selBG != "" {
		if p.Selection, err = parseHex(selBG); err != nil {
			return Palette{}, fmt.Errorf("colors.selection_background: %w", err)
		}
	} else {
		p.Selection = DefaultSelection
	}

	if cfg.SelectionForeground != "" {
		if p.SelectionFG, err = parseHex(cfg.SelectionForeground); err != nil {
			return Palette{}, fmt.Errorf("colors.selection_foreground: %w", err)
		}
	} else {
		p.SelectionFG = p.Foreground
	}

	if cfg.Cursor != "" {
		if p.Cursor, err = parseHex(cfg.Cursor); err != nil {
			return Palette{}, fmt.Errorf("colors.cursor: %w", err)
		}
	} else {
		p.Cursor = p.Foreground
	}

	for i, s := range cfg.ANSI {
		if p.ANSI[i], err = parseHex(s); err != nil {
			return Palette{}, fmt.Errorf("colors.ansi[%d]: %w", i, err)
		}
	}

	if err := fillChrome(&p, cfg, g); err != nil {
		return Palette{}, err
	}
	return p, nil
}

// ApplyCursorColor overrides the caret color when hex is non-empty
// (used for [cursor].color taking precedence over colors.cursor).
func ApplyCursorColor(p *Palette, hex string) error {
	if hex == "" {
		return nil
	}
	c, err := parseHex(hex)
	if err != nil {
		return fmt.Errorf("cursor.color: %w", err)
	}
	p.Cursor = c
	return nil
}

func fillChrome(p *Palette, cfg config.ColorsConfig, g GlassParams) error {
	var err error
	parseOr := func(hex string, key string, derived color.NRGBA) (color.NRGBA, error) {
		if hex == "" {
			return derived, nil
		}
		c, err := parseHex(hex)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("colors.%s: %w", key, err)
		}
		return c, nil
	}

	if p.TabBarBG, err = parseOr(cfg.TabBarBackground, "tab_bar_background",
		glassFill(p.Background, g.BarLift)); err != nil {
		return err
	}
	if p.ActiveTabBG, err = parseOr(cfg.ActiveTabBackground, "active_tab_background",
		glassFill(p.Background, g.Active)); err != nil {
		return err
	}
	if p.InactiveTabBG, err = parseOr(cfg.InactiveTabBackground, "inactive_tab_background",
		glassFill(p.Background, g.Inactive)); err != nil {
		return err
	}
	if p.HoverTabBG, err = parseOr(cfg.HoverTabBackground, "hover_tab_background",
		glassFill(p.Background, g.Hover)); err != nil {
		return err
	}
	if p.PlusButtonBG, err = parseOr(cfg.PlusButtonBackground, "plus_button_background",
		glassFill(p.Background, 0.10)); err != nil {
		return err
	}
	if p.ActiveTabFG, err = parseOr(cfg.ActiveTabForeground, "active_tab_foreground",
		p.Foreground); err != nil {
		return err
	}
	if p.InactiveTabFG, err = parseOr(cfg.InactiveTabForeground, "inactive_tab_foreground",
		dimFG(p.Foreground, p.Background, 0.32)); err != nil {
		return err
	}
	return nil
}

// Resolve converts an emu.Color (ANSI-16, xterm-256, or truecolor) to RGB.
func (p Palette) Resolve(c emu.Color) color.NRGBA {
	switch c {
	case emu.DefaultFG:
		return p.Foreground
	case emu.DefaultBG:
		return p.Background
	}
	if r, g, b, ok := c.RGB(); ok {
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}
	}
	if idx, ok := c.ANSI(); ok {
		return p.ANSI[idx]
	}
	if idx, ok := c.XTerm(); ok {
		return xterm256(uint16(idx))
	}
	return p.Foreground
}

// TabFill returns the pill fill for a tab's interaction state.
func (p Palette) TabFill(active, hovering, dragging bool) color.NRGBA {
	switch {
	case dragging:
		return glassFill(p.Background, 0.42)
	case active:
		return p.ActiveTabBG
	case hovering:
		return p.HoverTabBG
	default:
		return p.InactiveTabBG
	}
}

// TabFillGlass uses Theme glass params for the drag state fill.
func (p Palette) TabFillGlass(g GlassParams, active, hovering, dragging bool) color.NRGBA {
	switch {
	case dragging:
		return glassFill(p.Background, g.Drag)
	case active:
		return p.ActiveTabBG
	case hovering:
		return p.HoverTabBG
	default:
		return p.InactiveTabBG
	}
}

// TabTitleFG returns the title color for a tab's interaction state.
func (p Palette) TabTitleFG(active, hovering, dragging bool) color.NRGBA {
	switch {
	case active || dragging:
		return p.ActiveTabFG
	case hovering:
		return dimFG(p.Foreground, p.Background, 0.10)
	default:
		return p.InactiveTabFG
	}
}

// These color helpers live with the theme so chrome widgets can consume
// Theme without creating the former theme → chrome import cycle.
func glassFill(bg color.NRGBA, factor float32) color.NRGBA {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	blend := func(c uint8) uint8 { return uint8(float32(c) + (255-float32(c))*factor) }
	return color.NRGBA{R: blend(bg.R), G: blend(bg.G), B: blend(bg.B), A: 0xff}
}

func dimFG(fg, bg color.NRGBA, towardBG float32) color.NRGBA {
	if towardBG < 0 {
		towardBG = 0
	}
	if towardBG > 1 {
		towardBG = 1
	}
	mix := func(a, b uint8) uint8 { return uint8(float32(a)*(1-towardBG) + float32(b)*towardBG) }
	return color.NRGBA{R: mix(fg.R, bg.R), G: mix(fg.G, bg.G), B: mix(fg.B, bg.B), A: fg.A}
}

// xterm256 implements the standard xterm 256-color palette formula for
// indices [16, 256): a 6x6x6 RGB color cube (16-231) followed by a 24-step
// grayscale ramp (232-255). Indices [0, 16) are the customizable ANSI
// colors and are not handled here.
func xterm256(n uint16) color.NRGBA {
	switch {
	case n >= 232 && n <= 255:
		level := uint8(8 + (n-232)*10)
		return color.NRGBA{R: level, G: level, B: level, A: 0xff}
	case n >= 16 && n <= 231:
		n -= 16
		r, g, b := n/36, (n/6)%6, n%6
		return color.NRGBA{R: cubeLevel(r), G: cubeLevel(g), B: cubeLevel(b), A: 0xff}
	default:
		return color.NRGBA{A: 0xff}
	}
}

func cubeLevel(v uint16) uint8 {
	if v == 0 {
		return 0
	}
	return uint8(55 + v*40)
}

func parseHex(s string) (color.NRGBA, error) {
	c, err := parseHexAlpha(s)
	if err != nil {
		return color.NRGBA{}, err
	}
	if len(s) == 7 {
		c.A = 0xff
	}
	return c, nil
}

// parseHexAlpha accepts #rrggbb or #rrggbbaa.
func parseHexAlpha(s string) (color.NRGBA, error) {
	switch len(s) {
	case 7:
		if s[0] != '#' {
			return color.NRGBA{}, fmt.Errorf("invalid color %q (want #rrggbb or #rrggbbaa)", s)
		}
		r, ok1 := parseHexByte(s[1:3])
		g, ok2 := parseHexByte(s[3:5])
		b, ok3 := parseHexByte(s[5:7])
		if !ok1 || !ok2 || !ok3 {
			return color.NRGBA{}, fmt.Errorf("invalid color %q (want #rrggbb or #rrggbbaa)", s)
		}
		return color.NRGBA{R: r, G: g, B: b, A: 0xff}, nil
	case 9:
		if s[0] != '#' {
			return color.NRGBA{}, fmt.Errorf("invalid color %q (want #rrggbb or #rrggbbaa)", s)
		}
		r, ok1 := parseHexByte(s[1:3])
		g, ok2 := parseHexByte(s[3:5])
		b, ok3 := parseHexByte(s[5:7])
		a, ok4 := parseHexByte(s[7:9])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return color.NRGBA{}, fmt.Errorf("invalid color %q (want #rrggbb or #rrggbbaa)", s)
		}
		return color.NRGBA{R: r, G: g, B: b, A: a}, nil
	default:
		return color.NRGBA{}, fmt.Errorf("invalid color %q (want #rrggbb or #rrggbbaa)", s)
	}
}

func parseHexByte(s string) (uint8, bool) {
	var v uint8
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= uint8(c - '0')
		case c >= 'a' && c <= 'f':
			v |= uint8(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v |= uint8(c-'A') + 10
		default:
			return 0, false
		}
	}
	return v, true
}
