// Package theme resolves geckty's configured colors and emu.Color values
// into concrete RGB values for painting.
package theme

import (
	"fmt"
	"image/color"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/ui/chrome"
	"github.com/geckty/geckty/internal/vt/emu"
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
func NewPalette(cfg config.ColorsConfig) (Palette, error) {
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
		p.Selection = color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff}
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

	if err := fillChrome(&p, cfg); err != nil {
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

func fillChrome(p *Palette, cfg config.ColorsConfig) error {
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
		chrome.GlassFill(p.Background, chrome.GlassBarLift)); err != nil {
		return err
	}
	if p.ActiveTabBG, err = parseOr(cfg.ActiveTabBackground, "active_tab_background",
		chrome.GlassFill(p.Background, chrome.GlassActive)); err != nil {
		return err
	}
	if p.InactiveTabBG, err = parseOr(cfg.InactiveTabBackground, "inactive_tab_background",
		chrome.GlassFill(p.Background, chrome.GlassInactive)); err != nil {
		return err
	}
	if p.HoverTabBG, err = parseOr(cfg.HoverTabBackground, "hover_tab_background",
		chrome.GlassFill(p.Background, chrome.GlassHover)); err != nil {
		return err
	}
	if p.PlusButtonBG, err = parseOr(cfg.PlusButtonBackground, "plus_button_background",
		chrome.GlassFill(p.Background, 0.10)); err != nil {
		return err
	}
	if p.ActiveTabFG, err = parseOr(cfg.ActiveTabForeground, "active_tab_foreground",
		p.Foreground); err != nil {
		return err
	}
	if p.InactiveTabFG, err = parseOr(cfg.InactiveTabForeground, "inactive_tab_foreground",
		chrome.DimFG(p.Foreground, p.Background, 0.32)); err != nil {
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
		return chrome.GlassFill(p.Background, chrome.GlassDrag)
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
		return chrome.DimFG(p.Foreground, p.Background, 0.10)
	default:
		return p.InactiveTabFG
	}
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
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("invalid color %q (want #rrggbb)", s)
	}
	r, ok1 := parseHexByte(s[1:3])
	g, ok2 := parseHexByte(s[3:5])
	b, ok3 := parseHexByte(s[5:7])
	if !ok1 || !ok2 || !ok3 {
		return color.NRGBA{}, fmt.Errorf("invalid color %q (want #rrggbb)", s)
	}
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}, nil
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
