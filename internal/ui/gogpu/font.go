package gogpu

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/geckty/geckty/assets"
)

const osWindows = "windows"

// fontStyle selects one of a family's four static weight/slant variants.
// Static, not variable-font-axis-driven: golang.org/x/image/font/sfnt (which
// opentype.Parse uses) has no fvar/gvar support, so a variable font would
// always rasterize at its neutral instance regardless of requested weight —
// geckty's bundled fonts (see assets.Fonts) ship four separate static files
// instead, and system-font lookup below does the same.
type fontStyle int

const (
	styleRegular fontStyle = iota
	styleBold
	styleItalic
	styleBoldItalic
)

// fontRole picks which bundled family (assets.Fonts) backs a style this
// package can't find configured or installed anywhere else.
type fontRole int

const (
	roleMono fontRole = iota // terminal grid: IBM Plex Mono
	roleUI                   // tab bar/chrome: PT Sans
)

// embeddedFontPaths maps (role, style) to its assets.Fonts path.
var embeddedFontPaths = map[fontRole]map[fontStyle]string{
	roleMono: {
		styleRegular:    "fonts/mono/IBMPlexMono-Regular.ttf",
		styleBold:       "fonts/mono/IBMPlexMono-Bold.ttf",
		styleItalic:     "fonts/mono/IBMPlexMono-Italic.ttf",
		styleBoldItalic: "fonts/mono/IBMPlexMono-BoldItalic.ttf",
	},
	roleUI: {
		styleRegular:    "fonts/ui/PTSans-Regular.ttf",
		styleBold:       "fonts/ui/PTSans-Bold.ttf",
		styleItalic:     "fonts/ui/PTSans-Italic.ttf",
		styleBoldItalic: "fonts/ui/PTSans-BoldItalic.ttf",
	},
}

// embeddedFontData reads role's bundled file for style — panics on error
// since assets.Fonts is compiled into the binary (a missing/corrupt entry
// here is a build-time packaging bug, not a runtime condition to recover
// from).
func embeddedFontData(role fontRole, style fontStyle) []byte {
	data, err := assets.Fonts.ReadFile(embeddedFontPaths[role][style])
	if err != nil {
		panic("gogpu: missing embedded font asset: " + err.Error())
	}
	return data
}

// fontBundle holds a family's four style faces plus the Regular face's
// measured cell metrics. Bold/Italic/BoldItalic don't get their own metrics
// — for a monospace family every style shares the same advance width by
// definition, and the UI-font role's chrome text doesn't grid-align at all.
type fontBundle struct {
	regular, bold, italic, boldItalic font.Face
	cellW, cellH, ascent              int
}

// face returns b's face for the given bold/italic combination plus its
// styleIndex (see painter.go), falling back to Regular (index 0) for any
// variant that failed to load (e.g. a configured system family with no
// dedicated Bold file on disk) rather than a nil Face that would silently
// paint nothing. Painter.faceAndAtlas uses the returned index to look up
// the matching cached glyph atlas.
func (b fontBundle) face(bold, italic bool) (font.Face, int) {
	idx := styleIndex(bold, italic)
	faces := [4]font.Face{b.regular, b.bold, b.italic, b.boldItalic}
	if faces[idx] == nil {
		idx = 0
	}
	return faces[idx], idx
}

var (
	fontCandidatesOnce  sync.Once
	fontCandidatesCache map[fontStyle][][]byte
	fontCandidatesFor   string
	fontCandidatesRole  fontRole
)

// systemFontCandidates returns, per style, font file bytes to try in
// preference order: a configured family (best-effort filename match — see
// the package's known limitation compared to gio's real system-font
// matching), then the platform default for role, then role's embedded
// bundled font (assets.Fonts) as a universal last resort that's always
// present regardless of style.
func systemFontCandidates(configuredFamily string, role fontRole) map[fontStyle][][]byte {
	fontCandidatesOnce.Do(func() {
		fontCandidatesFor, fontCandidatesRole = configuredFamily, role
		fontCandidatesCache = loadFontCandidates(configuredFamily, role)
	})
	if fontCandidatesFor != configuredFamily || fontCandidatesRole != role {
		// Config changed since the cache was built (e.g. in tests, or a
		// hot-reloaded font family/UI font) — rebuild.
		fontCandidatesFor, fontCandidatesRole = configuredFamily, role
		fontCandidatesCache = loadFontCandidates(configuredFamily, role)
	}
	return fontCandidatesCache
}

func loadFontCandidates(configuredFamily string, role fontRole) map[fontStyle][][]byte {
	home, _ := os.UserHomeDir()
	out := make(map[fontStyle][][]byte, 4)
	for _, style := range []fontStyle{styleRegular, styleBold, styleItalic, styleBoldItalic} {
		var paths []string
		if f := strings.TrimSpace(configuredFamily); f != "" && !strings.EqualFold(f, "monospace") {
			paths = append(paths, configuredFamilyStylePaths(f, home, style)...)
		}
		paths = append(paths, platformStyleCandidates(style, role)...)

		var data [][]byte
		for _, p := range paths {
			if p == "" {
				continue
			}
			b, err := os.ReadFile(p) //nolint:gosec // G304: hardcoded system/user font directories
			if err != nil || len(b) == 0 {
				continue
			}
			data = append(data, b)
		}
		// The embedded bundled font is always available as the universal
		// fallback, so every style resolves to *something* even when
		// nothing on disk matched.
		data = append(data, embeddedFontData(role, style))
		out[style] = data
	}
	return out
}

// platformStyleCandidates returns platform-conventional file paths for a
// well-known system font of the given role/style — role's bundled font is
// tried after these regardless, so an incomplete or wrong guess here just
// falls through rather than breaking anything.
func platformStyleCandidates(style fontStyle, role fontRole) []string {
	switch runtime.GOOS {
	case osWindows:
		winFonts := filepath.Join(os.Getenv("SystemRoot"), "Fonts")
		if winFonts == `\Fonts` {
			winFonts = `C:\Windows\Fonts`
		}
		if role == roleUI {
			names := map[fontStyle]string{
				styleRegular: "segoeui.ttf", styleBold: "segoeuib.ttf",
				styleItalic: "segoeuii.ttf", styleBoldItalic: "segoeuiz.ttf",
			}
			return []string{filepath.Join(winFonts, names[style])}
		}
		names := map[fontStyle]string{
			styleRegular: "consola.ttf", styleBold: "consolab.ttf",
			styleItalic: "consolai.ttf", styleBoldItalic: "consolaz.ttf",
		}
		return []string{filepath.Join(winFonts, names[style]), filepath.Join(winFonts, "lucon.ttf")}
	case "darwin":
		if role == roleUI {
			names := map[fontStyle]string{
				styleRegular: "Helvetica.ttc", styleBold: "Helvetica.ttc",
				styleItalic: "Helvetica.ttc", styleBoldItalic: "Helvetica.ttc",
			}
			return []string{"/System/Library/Fonts/" + names[style]}
		}
		return []string{"/System/Library/Fonts/Menlo.ttc", "/System/Library/Fonts/SFNSMono.ttf"}
	default:
		home, _ := os.UserHomeDir()
		if role == roleUI {
			names := map[fontStyle]string{
				styleRegular: "DejaVuSans.ttf", styleBold: "DejaVuSans-Bold.ttf",
				styleItalic: "DejaVuSans-Oblique.ttf", styleBoldItalic: "DejaVuSans-BoldOblique.ttf",
			}
			n := names[style]
			return []string{
				filepath.Join(home, ".local/share/fonts", n),
				"/usr/share/fonts/truetype/dejavu/" + n,
			}
		}
		names := map[fontStyle]string{
			styleRegular: "DejaVuSansMono.ttf", styleBold: "DejaVuSansMono-Bold.ttf",
			styleItalic: "DejaVuSansMono-Oblique.ttf", styleBoldItalic: "DejaVuSansMono-BoldOblique.ttf",
		}
		n := names[style]
		return []string{
			filepath.Join(home, ".local/share/fonts", n),
			"/usr/share/fonts/truetype/dejavu/" + n,
			"/usr/share/fonts/TTF/" + n,
		}
	}
}

// configuredFamilyPaths guesses filenames for a user-configured font family
// name's regular weight — best-effort, since (unlike gio's system-font
// matching) there is no real font-enumeration API used here, only common
// naming conventions.
func configuredFamilyPaths(family, home string) []string {
	return configuredFamilyStylePaths(family, home, styleRegular)
}

// configuredFamilyStylePaths is configuredFamilyPaths generalized to a
// specific style's filename conventions (e.g. "Family-Bold.ttf" for
// styleBold).
func configuredFamilyStylePaths(family, home string, style fontStyle) []string {
	stripped := strings.ReplaceAll(family, " ", "")
	var names []string
	switch style {
	case styleBold:
		names = []string{stripped + "-Bold.ttf", stripped + "Bold.ttf", stripped + "-Bold.ttc"}
	case styleItalic:
		names = []string{stripped + "-Italic.ttf", stripped + "Italic.ttf", stripped + "-Oblique.ttf"}
	case styleBoldItalic:
		names = []string{stripped + "-BoldItalic.ttf", stripped + "BoldItalic.ttf", stripped + "-BoldOblique.ttf"}
	default:
		names = []string{
			family + ".ttf", family + ".ttc",
			stripped + ".ttf", stripped + ".ttc",
			stripped + "-Regular.ttf", stripped + "-Medium.ttf",
		}
	}
	var dirs []string
	switch runtime.GOOS {
	case osWindows:
		winFonts := filepath.Join(os.Getenv("SystemRoot"), "Fonts")
		if winFonts == `\Fonts` {
			winFonts = `C:\Windows\Fonts`
		}
		if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
			dirs = append(dirs, filepath.Join(lad, "Microsoft", "Windows", "Fonts"))
		}
		dirs = append(dirs, winFonts)
	case "darwin":
		dirs = append(dirs, filepath.Join(home, "Library", "Fonts"), "/Library/Fonts")
	default:
		dirs = append(dirs, filepath.Join(home, ".local/share/fonts"), "/usr/share/fonts/truetype")
	}
	var out []string
	for _, dir := range dirs {
		for _, n := range names {
			out = append(out, filepath.Join(dir, n))
		}
	}
	return out
}

func openFace(data []byte, size, dpi float64) (font.Face, error) {
	if col, err := opentype.ParseCollection(data); err == nil && col.NumFonts() > 0 {
		fnt, err := col.Font(0)
		if err != nil {
			return nil, err
		}
		return opentype.NewFace(fnt, &opentype.FaceOptions{
			Size: size,
			DPI:  dpi,
			// None: sharper on HiDPI; Full greyscale AA looks mushy at small sizes.
			Hinting: font.HintingNone,
		})
	}
	fnt, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    size,
		DPI:     dpi,
		Hinting: font.HintingNone,
	})
}

// openBestFace tries each candidate in order, returning the first that
// parses successfully.
func openBestFace(candidates [][]byte, size, dpi float64) font.Face {
	for _, data := range candidates {
		if f, err := openFace(data, size, dpi); err == nil && f != nil {
			return f
		}
	}
	return nil
}

// loadFontBundle loads role's four style faces (falling back through
// configured family -> platform default -> embedded bundled font per style,
// see systemFontCandidates) and measures cell metrics from the Regular
// face. configuredFamily is the relevant FontConfig/UIFontConfig.Family
// (empty selects the embedded bundled font directly; "monospace" skips
// straight to the platform default search).
func loadFontBundle(configuredFamily string, size, scaleFactor float64, role fontRole) fontBundle {
	dpi := 72.0 * scaleFactor
	candidates := systemFontCandidates(configuredFamily, role)

	b := fontBundle{
		regular:    openBestFace(candidates[styleRegular], size, dpi),
		bold:       openBestFace(candidates[styleBold], size, dpi),
		italic:     openBestFace(candidates[styleItalic], size, dpi),
		boldItalic: openBestFace(candidates[styleBoldItalic], size, dpi),
	}
	if b.regular == nil {
		// Can't happen in practice (the embedded bundled font is always a
		// candidate and ships in the binary), but fail loudly rather than
		// paint nothing if it ever does.
		f, err := openFace(embeddedFontData(role, styleRegular), size, dpi)
		if err != nil {
			panic("gogpu: failed to load embedded fallback font: " + err.Error())
		}
		b.regular = f
	}

	m := b.regular.Metrics()
	b.cellH = m.Height.Ceil()
	b.ascent = m.Ascent.Ceil()
	if adv, ok := b.regular.GlyphAdvance('M'); ok {
		b.cellW = adv.Ceil()
	} else {
		b.cellW = b.cellH / 2
	}
	return b
}
