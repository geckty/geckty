package chrome

import (
	"image/color"
)

// Flat opaque glass-pill tab fills.
// Inactive pills stay barely above the bar; active / hover / drag lift more.
const (
	GlassBarLift  = 0.08        // tab strip slightly above terminal bg
	GlassInactive = 0.07        // inactive: just-visible dark pill
	GlassHover    = 0.14        // hovered inactive
	GlassActive   = 0.34        // active capsule (mid-grey)
	GlassDrag     = 0.42        // dragged fill strength (alpha applied separately)
	GlassDragA    = uint8(0x99) // ~60% — translucent frosted drag pin
)

// GlassStyle returns the blend factor (for GlassFill) matching a tab's
// current interaction state, in priority order dragging > active > hovering.
func GlassStyle(active, hovering, dragging bool) float32 {
	switch {
	case dragging:
		return GlassDrag
	case active:
		return GlassActive
	case hovering:
		return GlassHover
	default:
		return GlassInactive
	}
}

// GlassFill blends bg toward white by factor (clamped to [0,1]), producing
// the flat "glass" fill color for a tab pill or the bar background itself.
func GlassFill(bg color.NRGBA, factor float32) color.NRGBA {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	return color.NRGBA{
		R: BlendChannel(bg.R, factor),
		G: BlendChannel(bg.G, factor),
		B: BlendChannel(bg.B, factor),
		A: 0xff,
	}
}

// DimFG mixes fg toward bg by towardBG (clamped to [0,1]) — used to dim
// inactive/hovered tab title text toward the bar background.
func DimFG(fg, bg color.NRGBA, towardBG float32) color.NRGBA {
	if towardBG < 0 {
		towardBG = 0
	}
	if towardBG > 1 {
		towardBG = 1
	}
	mix := func(a, b uint8) uint8 {
		return uint8(float32(a)*(1-towardBG) + float32(b)*towardBG)
	}
	return color.NRGBA{R: mix(fg.R, bg.R), G: mix(fg.G, bg.G), B: mix(fg.B, bg.B), A: fg.A}
}

// BlendChannel blends a single uint8 color channel toward white by factor
// (clamped to [0,255] after scaling).
func BlendChannel(c uint8, factor float32) uint8 {
	v := float32(c) + (255-float32(c))*factor
	if v > 255 {
		v = 255
	}
	if v < 0 {
		v = 0
	}
	return uint8(v)
}
