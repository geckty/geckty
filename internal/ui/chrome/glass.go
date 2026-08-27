package chrome

import (
	"image/color"

	"github.com/geckty/geckty/internal/config"
)

// Flat glass-pill factors — synced from config.Glass* (embedded TOML) at init.
var (
	GlassBarLift  float32
	GlassInactive float32
	GlassHover    float32
	GlassActive   float32
	GlassDrag     float32
	GlassRim      float32
	GlassDragA    = uint8(0x6a) // ~42% tint over warped underlay
	GlassRimA     uint8
	GlassFillA    uint8
)

func init() {
	syncGlassFromConfig()
}

// syncGlassFromConfig copies config.Glass* into chrome float/alpha vars.
// Called from init; exported for tests that mutate config.Glass*.
func syncGlassFromConfig() {
	GlassBarLift = float32(config.GlassBarLift)
	GlassInactive = float32(config.GlassInactive)
	GlassHover = float32(config.GlassHover)
	GlassActive = float32(config.GlassActive)
	GlassDrag = float32(config.GlassDrag)
	GlassRim = float32(config.GlassRim)
	GlassRimA = alphaByte(config.GlassRimAlpha)
	GlassFillA = alphaByte(config.GlassFillAlpha)
}

func alphaByte(a float64) uint8 {
	v := a*255 + 0.5
	if v > 255 {
		v = 255
	}
	if v < 0 {
		v = 0
	}
	return uint8(v)
}

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
