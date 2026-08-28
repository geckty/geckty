package theme

import (
	"image/color"
	"testing"

	"github.com/geckty/geckty/internal/config"
)

func TestNewBuildsFullTheme(t *testing.T) {
	cfg := config.Default()
	thm, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if thm.Palette.Foreground.A == 0 || thm.Palette.Background.A == 0 {
		t.Fatal("palette should be populated")
	}
	if thm.UI.VisualBell.A == 0 {
		t.Fatal("ui tokens should be populated")
	}
	if thm.Glass.Drag == 0 {
		t.Fatal("glass params should be populated")
	}
}

func TestNewRejectsInvalidUIColor(t *testing.T) {
	cfg := config.Default()
	cfg.UI.VisualBell = "not-a-color"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected error for invalid ui.visual_bell")
	}
}

func TestTabFillGlassDraggingUsesGlassDrag(t *testing.T) {
	p := testPalette(t)
	g := DefaultGlass()
	g.Drag = 0.5
	got := p.TabFillGlass(g, false, false, true)
	want := glassFill(p.Background, 0.5)
	if got != want {
		t.Fatalf("TabFillGlass(drag) = %v, want %v", got, want)
	}
}

func TestParseHexAlphaEightDigit(t *testing.T) {
	got, err := parseHexAlpha("#11223344")
	if err != nil {
		t.Fatal(err)
	}
	want := color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x44}
	if got != want {
		t.Fatalf("parseHexAlpha = %v, want %v", got, want)
	}
}

func TestGlassFromConfigOverrides(t *testing.T) {
	v := 0.42
	cfg := config.GlassConfig{Drag: &v}
	got := glassFromConfig(cfg)
	if got.Drag != 0.42 {
		t.Fatalf("Drag = %v", got.Drag)
	}
}

func TestNewRejectsInvalidCursorColor(t *testing.T) {
	cfg := config.Default()
	cfg.Cursor.Color = "not-a-color"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected error for invalid cursor.color")
	}
}

func TestNewRejectsInvalidPaletteForeground(t *testing.T) {
	cfg := config.Default()
	cfg.Colors.Foreground = "bad"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected error for invalid colors.foreground")
	}
}

func TestNewRejectsInvalidSearchMatch(t *testing.T) {
	cfg := config.Default()
	cfg.UI.SearchMatch = "bad"
	if _, err := New(cfg); err == nil {
		t.Fatal("expected error for invalid ui.search_match")
	}
}

func TestNewCustomContentBracketsColor(t *testing.T) {
	cfg := config.Default()
	cfg.UI.ContentBrackets = "#aabbccdd"
	thm, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if thm.UI.ContentBrackets.A == 0 {
		t.Fatal("custom content_brackets color should apply")
	}
}

func TestNewExplicitContentBracketsOff(t *testing.T) {
	cfg := config.Default()
	cfg.UI.ContentBrackets = config.ContentBracketsOff
	thm, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if thm.UI.ContentBrackets != (color.NRGBA{}) {
		t.Fatalf("off sentinel should disable brackets, got %v", thm.UI.ContentBrackets)
	}
}

func TestNewCommandIndicatorFlags(t *testing.T) {
	cfg := config.Default()
	on := true
	cfg.UI.CommandBorderEnabled = &on
	cfg.UI.CommandDotEnabled = &on
	thm, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !thm.UI.CommandBorderEnabled || !thm.UI.CommandDotEnabled {
		t.Fatal("command indicator flags should propagate")
	}
}

func TestNewRejectsInvalidUIFields(t *testing.T) {
	fields := []struct {
		name string
		mut  func(*config.Config)
	}{
		{"scrollbar_track", func(c *config.Config) { c.UI.ScrollbarTrack = "bad" }},
		{"scrollbar_thumb", func(c *config.Config) { c.UI.ScrollbarThumb = "bad" }},
		{"url_underline", func(c *config.Config) { c.UI.URLUnderline = "bad" }},
		{"hint_label_bg", func(c *config.Config) { c.UI.HintLabelBG = "bad" }},
		{"hint_label_fg", func(c *config.Config) { c.UI.HintLabelFG = "bad" }},
		{"pane_focus_border", func(c *config.Config) { c.UI.PaneFocusBorder = "bad" }},
		{"command_running", func(c *config.Config) { c.UI.CommandRunning = "bad" }},
		{"command_success", func(c *config.Config) { c.UI.CommandSuccess = "bad" }},
		{"command_failed", func(c *config.Config) { c.UI.CommandFailed = "bad" }},
		{"content_brackets", func(c *config.Config) { c.UI.ContentBrackets = "bad" }},
	}
	for _, tc := range fields {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			tc.mut(cfg)
			if _, err := New(cfg); err == nil {
				t.Fatalf("expected error for invalid ui.%s", tc.name)
			}
		})
	}
}

func TestGlassFillClampsFactor(t *testing.T) {
	bg := color.NRGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff}
	if got := glassFill(bg, -1); got != glassFill(bg, 0) {
		t.Fatal("negative factor should clamp to 0")
	}
	if got := glassFill(bg, 2); got != glassFill(bg, 1) {
		t.Fatal("factor>1 should clamp to 1")
	}
}

func TestDimFGClampsTowardBG(t *testing.T) {
	fg := color.NRGBA{R: 0xff, A: 0xff}
	bg := color.NRGBA{B: 0x40, A: 0xff}
	if got := dimFG(fg, bg, -0.5); got != dimFG(fg, bg, 0) {
		t.Fatal("negative towardBG should clamp to 0")
	}
	if got := dimFG(fg, bg, 2); got != dimFG(fg, bg, 1) {
		t.Fatal("towardBG>1 should clamp to 1")
	}
}

func TestParseHexAlphaRejectsMalformed(t *testing.T) {
	for _, s := range []string{"rrggbb", "#gggggg", "11223344", "#112233gg", "#abcd"} {
		if _, err := parseHexAlpha(s); err == nil {
			t.Fatalf("parseHexAlpha(%q) should fail", s)
		}
	}
}

func TestXterm256CubeAndGray(t *testing.T) {
	if got := xterm256(16); got.R != 0 || got.G != 0 || got.B != 0 {
		t.Fatalf("cube base = %v", got)
	}
	if got := xterm256(232); got.R != 8 || got.G != 8 || got.B != 8 {
		t.Fatalf("gray ramp start = %v", got)
	}
	if got := xterm256(10); got.A != 0xff {
		t.Fatalf("low indices should still return opaque black, got %v", got)
	}
}
