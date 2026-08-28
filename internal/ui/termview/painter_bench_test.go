package termview

import (
	"io"
	"strings"
	"testing"

	"github.com/geckty/geckty/internal/vt"
	"github.com/geckty/geckty/internal/vt/emu"
)

func benchPaintGrid(b *testing.B, cols, rows int) {
	bundle := LoadFontBundle("", 13, 2, RoleMono)
	if bundle.Regular == nil {
		b.Fatal("Regular face expected")
	}
	p := &Painter{
		Palette:    testPalette(),
		Fonts:      bundle,
		CellWidth:  bundle.CellW,
		CellHeight: bundle.CellH,
		Ascent:     bundle.Ascent,
	}
	term := vt.New(cols, rows, io.Discard, nil, 10000)
	var sb strings.Builder
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			sb.WriteByte(byte('!' + (c % 90)))
		}
		sb.WriteByte('\n')
	}
	_, _ = term.Write([]byte(sb.String()))

	fw := cols * p.CellWidth
	fh := rows * p.CellHeight
	buf := make([]byte, fw*fh*4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Paint(buf, fw, fh, 0, 0, term, 0, Selection{}, nil, true, nil, cols, rows)
	}
}

func BenchmarkPaintGrid80x24(b *testing.B) { benchPaintGrid(b, 80, 24) }
func BenchmarkPaintGrid80x40(b *testing.B) { benchPaintGrid(b, 80, 40) }

func BenchmarkPaintRow(b *testing.B) {
	p := testPainter()
	p.ensureAtlas()
	line := make(emu.Line, 80)
	for i := range line {
		line[i].Char = rune('!' + (i % 90))
	}
	buf := make([]byte, 80*p.CellWidth*24*p.CellHeight*4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for row := 0; row < 24; row++ {
			p.paintRow(buf, 80*p.CellWidth, 24*p.CellHeight, line, 80, 0, row*p.CellHeight, Selection{}, row)
		}
	}
}
