package chrome

import (
	"image"
	"image/color"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// Terminal.app Pro tab fills — flat opaque blends (gio has no vibrancy).
// Inactive pills stay barely above the bar; active / hover / drag lift more.
const (
	glassBarLift  = 0.08        // tab strip slightly above terminal bg
	glassInactive = 0.07        // inactive: just-visible dark pill
	glassHover    = 0.14        // hovered inactive
	glassActive   = 0.34        // active capsule (Terminal mid-grey)
	glassDrag     = 0.42        // dragged fill strength (alpha applied separately)
	glassDragA    = uint8(0x99) // ~60% — translucent frosted drag pin
)

func glassStyle(active, hovering, dragging bool) float32 {
	switch {
	case dragging:
		return glassDrag
	case active:
		return glassActive
	case hovering:
		return glassHover
	default:
		return glassInactive
	}
}

// paintGlassCapsule draws a frosted capsule. factor <= 0 skips.
// alpha < 255 makes the pill translucent (dragged tab).
func paintGlassCapsule(ops *op.Ops, bg color.NRGBA, rect image.Rectangle, radius int, factor float32, alpha uint8) {
	if factor <= 0 || ops == nil || rect.Empty() || rect.Dx() < 2 || rect.Dy() < 2 {
		return
	}
	if radius < 0 {
		radius = 0
	}
	if rmax := rect.Dy() / 2; radius > rmax {
		radius = rmax
	}
	if rmax := rect.Dx() / 2; radius > rmax {
		radius = rmax
	}
	shape := clip.UniformRRect(rect, radius).Op(ops)
	fill := glassFill(bg, factor)
	fill.A = alpha
	paint.FillShape(ops, fill, shape)
}

func glassFill(bg color.NRGBA, factor float32) color.NRGBA {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	return color.NRGBA{
		R: blendChannel(bg.R, factor),
		G: blendChannel(bg.G, factor),
		B: blendChannel(bg.B, factor),
		A: 0xff,
	}
}

func dimFG(fg, bg color.NRGBA, towardBG float32) color.NRGBA {
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

func blendChannel(c uint8, factor float32) uint8 {
	v := float32(c) + (255-float32(c))*factor
	if v > 255 {
		v = 255
	}
	if v < 0 {
		v = 0
	}
	return uint8(v)
}
