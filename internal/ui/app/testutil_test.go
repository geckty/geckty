package app

import (
	"image/color"
	"testing"

	"github.com/geckty/geckty/internal/ui/theme"
)

func testTheme() theme.Theme {
	pal := theme.Palette{
		Foreground:    color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Background:    color.NRGBA{R: 0, G: 0, B: 0, A: 0xff},
		Selection:     color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff},
		SelectionFG:   color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		Cursor:        color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		TabBarBG:      color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xff},
		ActiveTabFG:   color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		ActiveTabBG:   color.NRGBA{R: 0x57, G: 0x57, B: 0x57, A: 0xff},
		InactiveTabFG: color.NRGBA{R: 0xad, G: 0xad, B: 0xad, A: 0xff},
		InactiveTabBG: color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xff},
		HoverTabBG:    color.NRGBA{R: 0x24, G: 0x24, B: 0x24, A: 0xff},
		PlusButtonBG:  color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff},
	}
	for i := range pal.ANSI {
		pal.ANSI[i] = color.NRGBA{R: uint8(i * 10), A: 0xff}
	}
	return theme.Theme{
		Palette: pal,
		Glass:   theme.DefaultGlass(),
		UI: theme.UITokens{
			VisualBell:           color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55},
			ScrollbarTrack:       color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x28},
			ScrollbarThumb:       color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x70},
			URLUnderline:         pal.ANSI[12],
			SearchMatch:          color.NRGBA{R: pal.ANSI[11].R, G: pal.ANSI[11].G, B: pal.ANSI[11].B, A: 0x90},
			HintLabelBG:          pal.ANSI[11],
			HintLabelFG:          pal.Background,
			PaneFocusBorder:      color.NRGBA{R: pal.ANSI[12].R, G: pal.ANSI[12].G, B: pal.ANSI[12].B, A: 0xaa},
			CommandRunning:       pal.ANSI[6],
			CommandSuccess:       pal.ANSI[2],
			CommandFailed:        pal.ANSI[1],
			CommandBorderEnabled: true,
		},
	}
}

func testPalette() theme.Palette { return testTheme().Palette }

func newBuf(w, h int) []byte { return make([]byte, w*h*4) }

func pixelAt(buf []byte, frameW, x, y int) color.RGBA {
	_ = testing.Verbose()
	off := (y*frameW + x) * 4
	return color.RGBA{R: buf[off], G: buf[off+1], B: buf[off+2], A: buf[off+3]}
}
