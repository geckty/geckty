// Package theme resolves geckty's configured colors and emu.Color values
// into concrete RGB values for painting.
package theme

import (
	"fmt"
	"image/color"
	"runtime"
	"strings"

	"gioui.org/font"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/vt/emu"
)

// MonospaceTypeface is the font.Font.Typeface every fixed-width label uses.
// Platform default: Menlo (macOS), Consolas (Windows — Menlo is absent and
// a bad fallback widens cells so ConPTY reports fewer columns and
// PowerShell mid-truncates paths), monospace elsewhere.
var MonospaceTypeface = font.Typeface(defaultMonospaceFamily())

func defaultMonospaceFamily() string {
	switch runtime.GOOS {
	case "windows":
		return "Consolas"
	case "darwin":
		return "Menlo"
	default:
		return "monospace"
	}
}

// SetMonospaceFamily applies a configured font family. Empty / generic
// "monospace" keeps the platform default so Windows never stays on Menlo.
func SetMonospaceFamily(family string) {
	family = strings.TrimSpace(family)
	if family == "" || strings.EqualFold(family, "monospace") {
		MonospaceTypeface = font.Typeface(defaultMonospaceFamily())
		return
	}
	MonospaceTypeface = font.Typeface(family)
}

// Palette maps emu.Color values to paintable colors.
type Palette struct {
	Foreground color.NRGBA
	Background color.NRGBA
	Selection  color.NRGBA
	ANSI       [16]color.NRGBA
}

// NewPalette parses cfg's hex color strings into a Palette.
func NewPalette(cfg config.ColorsConfig) (Palette, error) {
	var p Palette
	var err error
	if p.Foreground, err = parseHex(cfg.Foreground); err != nil {
		return Palette{}, fmt.Errorf("colors.foreground: %w", err)
	}
	if p.Background, err = parseHex(cfg.Background); err != nil {
		return Palette{}, fmt.Errorf("colors.background: %w", err)
	}
	if cfg.Selection != "" {
		if p.Selection, err = parseHex(cfg.Selection); err != nil {
			return Palette{}, fmt.Errorf("colors.selection: %w", err)
		}
	} else {
		// Fallback: Terminal Pro-like mid-grey when the config predates
		// the selection field.
		p.Selection = color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff}
	}
	for i, s := range cfg.ANSI {
		if p.ANSI[i], err = parseHex(s); err != nil {
			return Palette{}, fmt.Errorf("colors.ansi[%d]: %w", i, err)
		}
	}
	return p, nil
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
