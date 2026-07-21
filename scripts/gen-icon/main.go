// Command gen-icon renders geckty's placeholder app icon (a simple
// terminal-window mark, no external assets or font rendering needed) to
// assets/icon-1024.png, colored from the project's own default theme
// (internal/config/defaults.go) rather than an arbitrary palette.
//
// Superseded: assets/icon.png (the gecko-over-a-terminal mark) is now the
// real icon art, embedded at runtime (assets/assets.go) and used by
// scripts/gen-icons.sh to build every packaged platform icon. This
// generator and assets/icon-1024.png are kept only as the placeholder
// this project shipped with before that art existed — not wired into the
// build anymore.
//
// Run with: go run ./scripts/gen-icon
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

const size = 1024

var (
	background = color.NRGBA{0x2e, 0x34, 0x40, 0xff} // config.Default().Colors.Background
	panel      = color.NRGBA{0x3b, 0x42, 0x52, 0xff} // config.Default().Colors.ANSI[0]
	accent     = color.NRGBA{0xa3, 0xbe, 0x8c, 0xff} // config.Default().Colors.ANSI[2]
	cursor     = color.NRGBA{0x88, 0xc0, 0xd0, 0xff} // config.Default().Colors.ANSI[6]
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
	log.Println("wrote assets/icon-1024.png")
}

func run() error {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	fillRoundedRect(img, image.Rect(0, 0, size, size), size/8, background)

	// A terminal-window panel inset from the edges.
	inset := size / 8
	panelRect := image.Rect(inset, inset, size-inset, size-inset)
	fillRoundedRect(img, panelRect, size/24, panel)

	// A prompt caret: a small rectangle plus a trailing accent bar,
	// suggesting a shell prompt without needing real text rendering.
	caretY := panelRect.Min.Y + panelRect.Dy()*2/3
	caretH := panelRect.Dy() / 10
	caretX := panelRect.Min.X + panelRect.Dx()/6
	fillRect(img, image.Rect(caretX, caretY, caretX+panelRect.Dx()/3, caretY+caretH), accent)
	barX := caretX + panelRect.Dx()/3 + panelRect.Dx()/12
	fillRect(img, image.Rect(barX, caretY, barX+panelRect.Dx()/10, caretY+caretH), cursor)

	out, err := os.Create("assets/icon-1024.png")
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	return png.Encode(out, img)
}

func fillRect(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

// fillRoundedRect fills r with c, rounding its corners to radius using a
// simple per-pixel circle test in each corner quadrant — accurate enough
// for a 1024px icon, not meant for general-purpose rendering.
func fillRoundedRect(img *image.NRGBA, r image.Rectangle, radius int, c color.NRGBA) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if inRoundedRect(x, y, r, radius) {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

func inRoundedRect(x, y int, r image.Rectangle, radius int) bool {
	cx, cy := x, y
	switch {
	case x < r.Min.X+radius && y < r.Min.Y+radius:
		return withinCircle(cx, cy, r.Min.X+radius, r.Min.Y+radius, radius)
	case x >= r.Max.X-radius && y < r.Min.Y+radius:
		return withinCircle(cx, cy, r.Max.X-radius, r.Min.Y+radius, radius)
	case x < r.Min.X+radius && y >= r.Max.Y-radius:
		return withinCircle(cx, cy, r.Min.X+radius, r.Max.Y-radius, radius)
	case x >= r.Max.X-radius && y >= r.Max.Y-radius:
		return withinCircle(cx, cy, r.Max.X-radius, r.Max.Y-radius, radius)
	default:
		return true
	}
}

func withinCircle(x, y, cx, cy, radius int) bool {
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}
