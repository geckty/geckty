package theme

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/geckty/geckty/internal/config"
	"github.com/geckty/geckty/internal/vt/emu"
)

// TestGlassThemeParses loads the built-in glass theme via deprecated preset alias.
func TestGlassThemeParses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"glass\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	p, err := NewPalette(cfg.Colors)
	if err != nil {
		t.Fatalf("NewPalette(glass theme): %v", err)
	}
	if p.Background != (color.NRGBA{R: 0x1d, G: 0x1f, B: 0x22, A: 0xff}) { // #1d1f22
		t.Fatalf("glass theme Background = %v, want #1d1f22", p.Background)
	}
	if p.Foreground != (color.NRGBA{R: 0xf4, G: 0xf4, B: 0xf4, A: 0xff}) {
		t.Fatalf("glass theme Foreground = %v, want #f4f4f4", p.Foreground)
	}
	if p.Selection != (color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff}) {
		t.Fatalf("glass theme Selection = %v, want #525252", p.Selection)
	}
	if p.Cursor != p.Foreground {
		t.Fatalf("default Cursor = %v, want Foreground", p.Cursor)
	}
	if p.ActiveTabBG.A == 0 || p.TabBarBG.A == 0 {
		t.Fatal("chrome slots should be derived when unset")
	}
}

func testPalette(t *testing.T) Palette {
	t.Helper()
	p, err := NewPalette(config.Default().Colors)
	if err != nil {
		t.Fatalf("NewPalette: %v", err)
	}
	return p
}

func TestResolveDefaultAndANSI(t *testing.T) {
	p := testPalette(t)

	if got := p.Resolve(emu.DefaultFG); got != p.Foreground {
		t.Fatalf("DefaultFG = %v, want %v", got, p.Foreground)
	}
	if got := p.Resolve(emu.DefaultBG); got != p.Background {
		t.Fatalf("DefaultBG = %v, want %v", got, p.Background)
	}
	if got := p.Resolve(emu.Red); got != p.ANSI[1] {
		t.Fatalf("Red = %v, want %v", got, p.ANSI[1])
	}
	if got := p.Resolve(emu.ANSIColor(15)); got != p.ANSI[15] {
		t.Fatalf("ANSIColor(15) = %v, want %v", got, p.ANSI[15])
	}
}

func TestResolveXterm256(t *testing.T) {
	p := testPalette(t)
	if got := p.Resolve(emu.XTermColor(196)); got != xterm256(196) {
		t.Fatalf("XTermColor(196) = %v, want %v", got, xterm256(196))
	}
}

func TestResolveTrueColor(t *testing.T) {
	// cy/emu is the reason geckty regained truecolor support — vt10x
	// (this project's VT library for a time) had no SGR 38/48;2;r;g;b
	// parsing at all. Verifying it end-to-end here, not just trusting
	// the library.
	p := testPalette(t)
	want := color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}
	if got := p.Resolve(emu.RGBColor(0x12, 0x34, 0x56)); got != want {
		t.Fatalf("RGBColor(0x12,0x34,0x56) = %v, want %v", got, want)
	}
}

func TestXterm256Cube(t *testing.T) {
	// Index 16 is the cube's black corner (r=g=b=0).
	if got := xterm256(16); got != (color.NRGBA{A: 0xff}) {
		t.Fatalf("xterm256(16) = %v, want black", got)
	}
	// Index 21 = r=0,g=0,b=5 -> pure blue at max cube level (0xff).
	if got := xterm256(21); got != (color.NRGBA{B: 0xff, A: 0xff}) {
		t.Fatalf("xterm256(21) = %v, want pure blue", got)
	}
	// Index 231 is the cube's white corner (r=g=b=5).
	want := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	if got := xterm256(231); got != want {
		t.Fatalf("xterm256(231) = %v, want %v", got, want)
	}
}

func TestXterm256Grayscale(t *testing.T) {
	// Index 232 is the darkest grayscale step.
	if got := xterm256(232); got != (color.NRGBA{R: 8, G: 8, B: 8, A: 0xff}) {
		t.Fatalf("xterm256(232) = %v", got)
	}
	// Index 255 is the lightest grayscale step.
	if got := xterm256(255); got != (color.NRGBA{R: 238, G: 238, B: 238, A: 0xff}) {
		t.Fatalf("xterm256(255) = %v", got)
	}
}

func TestParseHex(t *testing.T) {
	cases := map[string]color.NRGBA{
		"#000000": {A: 0xff},
		"#ffffff": {R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		"#2e3440": {R: 0x2e, G: 0x34, B: 0x40, A: 0xff},
	}
	for in, want := range cases {
		got, err := parseHex(in)
		if err != nil {
			t.Fatalf("parseHex(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("parseHex(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseHexRejectsInvalid(t *testing.T) {
	for _, in := range []string{"", "red", "#fff", "#gggggg", "000000"} {
		if _, err := parseHex(in); err == nil {
			t.Fatalf("parseHex(%q) expected error", in)
		}
	}
}

func TestNewPaletteRejectsInvalidConfig(t *testing.T) {
	cfg := config.Default().Colors
	cfg.Foreground = "not-a-color"
	if _, err := NewPalette(cfg); err == nil {
		t.Fatal("expected error for invalid foreground")
	}
}

func TestNewPaletteRejectsInvalidBackground(t *testing.T) {
	cfg := config.Default().Colors
	cfg.Background = "not-a-color"
	if _, err := NewPalette(cfg); err == nil {
		t.Fatal("expected error for invalid background")
	}
}

func TestNewPaletteRejectsInvalidSelection(t *testing.T) {
	cfg := config.Default().Colors
	cfg.Selection = "not-a-color"
	cfg.SelectionBackground = ""
	if _, err := NewPalette(cfg); err == nil {
		t.Fatal("expected error for invalid selection")
	}
}

func TestNewPaletteDefaultsSelectionWhenEmpty(t *testing.T) {
	cfg := config.Default().Colors
	cfg.Selection = ""
	cfg.SelectionBackground = ""
	p, err := NewPalette(cfg)
	if err != nil {
		t.Fatalf("NewPalette: %v", err)
	}
	want := color.NRGBA{R: 0x52, G: 0x52, B: 0x52, A: 0xff}
	if p.Selection != want {
		t.Fatalf("Selection = %v, want fallback mid-grey %v", p.Selection, want)
	}
}

func TestNewPaletteExplicitChromeAndCursor(t *testing.T) {
	cfg := config.Default().Colors
	cfg.Cursor = "#ff0000"
	cfg.ActiveTabBackground = "#00ff00"
	cfg.TabBarBackground = "#0000ff"
	cfg.SelectionBackground = "#112233"
	cfg.SelectionForeground = "#445566"
	p, err := NewPalette(cfg)
	if err != nil {
		t.Fatalf("NewPalette: %v", err)
	}
	if p.Cursor != (color.NRGBA{R: 0xff, A: 0xff}) {
		t.Fatalf("Cursor = %v, want #ff0000", p.Cursor)
	}
	if p.ActiveTabBG != (color.NRGBA{G: 0xff, A: 0xff}) {
		t.Fatalf("ActiveTabBG = %v, want #00ff00", p.ActiveTabBG)
	}
	if p.TabBarBG != (color.NRGBA{B: 0xff, A: 0xff}) {
		t.Fatalf("TabBarBG = %v, want #0000ff", p.TabBarBG)
	}
	if p.Selection != (color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}) {
		t.Fatalf("Selection = %v", p.Selection)
	}
	if p.SelectionFG != (color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff}) {
		t.Fatalf("SelectionFG = %v", p.SelectionFG)
	}
	if fill := p.TabFill(true, false, false); fill != p.ActiveTabBG {
		t.Fatalf("TabFill(active) = %v, want ActiveTabBG", fill)
	}
}

func TestApplyCursorColor(t *testing.T) {
	p := testPalette(t)
	if err := ApplyCursorColor(&p, "#00ff00"); err != nil {
		t.Fatal(err)
	}
	if p.Cursor != (color.NRGBA{G: 0xff, A: 0xff}) {
		t.Fatalf("Cursor = %v after ApplyCursorColor", p.Cursor)
	}
	if err := ApplyCursorColor(&p, ""); err != nil {
		t.Fatal(err)
	}
	if err := ApplyCursorColor(&p, "bad"); err == nil {
		t.Fatal("expected error for bad cursor.color")
	}
}

func TestNewPaletteRejectsInvalidANSI(t *testing.T) {
	cfg := config.Default().Colors
	cfg.ANSI[3] = "not-a-color"
	if _, err := NewPalette(cfg); err == nil {
		t.Fatal("expected error for invalid ANSI color")
	}
}
