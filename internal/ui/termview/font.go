package termview

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"

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

// FontRole picks which bundled family (assets.Fonts) backs a style this
// package can't find configured or installed anywhere else.
type FontRole int

const (
	// RoleMono is the terminal grid face (bundled IBM Plex Mono).
	RoleMono FontRole = iota
	// RoleUI is the tab bar / chrome face (bundled PT Sans).
	RoleUI
)

// embeddedFontPaths maps (role, style) to its assets.Fonts path.
var embeddedFontPaths = map[FontRole]map[fontStyle]string{
	RoleMono: {
		styleRegular:    "fonts/mono/IBMPlexMono-Regular.ttf",
		styleBold:       "fonts/mono/IBMPlexMono-Bold.ttf",
		styleItalic:     "fonts/mono/IBMPlexMono-Italic.ttf",
		styleBoldItalic: "fonts/mono/IBMPlexMono-BoldItalic.ttf",
	},
	RoleUI: {
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
func embeddedFontData(role FontRole, style fontStyle) []byte {
	data, err := assets.Fonts.ReadFile(embeddedFontPaths[role][style])
	if err != nil {
		panic("geckty: missing embedded font asset: " + err.Error())
	}
	return data
}

// FontBundle holds a family's four style faces plus the Regular face's
// measured cell metrics. Bold/Italic/BoldItalic don't get their own metrics
// — for a monospace family every style shares the same advance width by
// definition, and the UI-font role's chrome text doesn't grid-align at all.
type FontBundle struct {
	Regular, Bold, Italic, BoldItalic font.Face
	CellW, CellH, Ascent              int
}

// face returns b's face for the given bold/italic combination plus its
// styleIndex (see painter.go), falling back to Regular (index 0) for any
// variant that failed to load (e.g. a configured system family with no
// dedicated Bold file on disk) rather than a nil Face that would silently
// paint nothing. Painter.faceAndAtlas uses the returned index to look up
// the matching cached glyph atlas.
func (b FontBundle) face(bold, italic bool) (font.Face, int) {
	idx := styleIndex(bold, italic)
	faces := [4]font.Face{b.Regular, b.Bold, b.Italic, b.BoldItalic}
	if faces[idx] == nil {
		idx = 0
	}
	return faces[idx], idx
}

var (
	fontPathsOnce  sync.Once
	fontPathsCache map[fontStyle][]string
	fontPathsFor   string
	fontPathsRole  FontRole
)

// fontCandidatePaths returns filesystem paths to try for each style, in
// preference order. An empty configuredFamily uses embedded bundled fonts
// only (no platform scan). "monospace" skips the family guess and uses the
// platform default for role. The embedded fallback is opened separately —
// it is not duplicated in the path list.
func fontCandidatePaths(configuredFamily string, role FontRole) map[fontStyle][]string {
	fontPathsOnce.Do(func() {
		fontPathsFor, fontPathsRole = configuredFamily, role
		fontPathsCache = loadFontCandidatePaths(configuredFamily, role)
	})
	if fontPathsFor != configuredFamily || fontPathsRole != role {
		fontPathsFor, fontPathsRole = configuredFamily, role
		fontPathsCache = loadFontCandidatePaths(configuredFamily, role)
	}
	return fontPathsCache
}

func loadFontCandidatePaths(configuredFamily string, role FontRole) map[fontStyle][]string {
	family := strings.TrimSpace(configuredFamily)
	out := make(map[fontStyle][]string, 4)
	for _, style := range []fontStyle{styleRegular, styleBold, styleItalic, styleBoldItalic} {
		if family == "" {
			// Default config: embedded IBM Plex / PT Sans only — avoids
			// loading Menlo/Consolas/etc. into RAM when the user did not
			// request a custom family.
			out[style] = nil
			continue
		}
		var paths []string
		if !strings.EqualFold(family, "monospace") {
			home, _ := os.UserHomeDir()
			paths = append(paths, configuredFamilyStylePaths(family, home, style)...)
		}
		paths = append(paths, platformStyleCandidates(style, role)...)
		out[style] = paths
	}
	return out
}

// platformStyleCandidates returns platform-conventional file paths for a
// well-known system font of the given role/style — role's bundled font is
// tried after these regardless, so an incomplete or wrong guess here just
// falls through rather than breaking anything.
func platformStyleCandidates(style fontStyle, role FontRole) []string {
	switch runtime.GOOS {
	case osWindows:
		winFonts := filepath.Join(os.Getenv("SystemRoot"), "Fonts")
		if winFonts == `\Fonts` {
			winFonts = `C:\Windows\Fonts`
		}
		if role == RoleUI {
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
		if role == RoleUI {
			names := map[fontStyle]string{
				styleRegular: "Helvetica.ttc", styleBold: "Helvetica.ttc",
				styleItalic: "Helvetica.ttc", styleBoldItalic: "Helvetica.ttc",
			}
			return []string{"/System/Library/Fonts/" + names[style]}
		}
		return []string{"/System/Library/Fonts/Menlo.ttc", "/System/Library/Fonts/SFNSMono.ttf"}
	default:
		home, _ := os.UserHomeDir()
		if role == RoleUI {
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
// name's regular weight — best-effort, since (unlike a full system-font
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

func openFace(data []byte, size, dpi float64, role FontRole) (font.Face, error) {
	return openFaceStyle(data, size, dpi, styleRegular, role)
}

func openFaceStyle(data []byte, size, dpi float64, style fontStyle, role FontRole) (font.Face, error) {
	hinting := font.HintingNone
	if role == RoleMono {
		// Full grid hinting: crisp stems on HiDPI like JetBrains Terminal /
		// iTerm — HintingNone looked soft/thin next to IDE chrome.
		hinting = font.HintingFull
	}
	opts := &opentype.FaceOptions{
		Size:    size,
		DPI:     dpi,
		Hinting: hinting,
	}
	if col, err := opentype.ParseCollection(data); err == nil && col.NumFonts() > 0 {
		idx := pickCollectionIndex(data, style, col.NumFonts())
		fnt, err := col.Font(idx)
		if err != nil {
			return nil, err
		}
		return opentype.NewFace(fnt, opts)
	}
	fnt, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(fnt, opts)
}

// pickCollectionIndex chooses a face inside a .ttc for the requested style
// (Menlo ships Regular/Bold/Italic/Bold Italic in one file — always taking
// Font(0) made every style render as Regular).
func pickCollectionIndex(data []byte, style fontStyle, n int) int {
	if n <= 1 {
		return 0
	}
	col, err := sfnt.ParseCollection(data)
	if err != nil {
		return 0
	}
	want := collectionSubfamilyWant(style)
	var buf sfnt.Buffer
	best, bestScore := 0, -1
	for i := 0; i < n && i < col.NumFonts(); i++ {
		f, err := col.Font(i)
		if err != nil {
			continue
		}
		sub, _ := f.Name(&buf, sfnt.NameIDSubfamily)
		score := collectionSubfamilyScore(strings.ToLower(sub), want, style)
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	return best
}

func collectionSubfamilyWant(style fontStyle) []string {
	switch style {
	case styleBold:
		return []string{"bold"}
	case styleItalic:
		return []string{"italic", "oblique"}
	case styleBoldItalic:
		return []string{"bold italic", "bold oblique"}
	default:
		return []string{"medium", "semibold", "regular", "book", "roman"}
	}
}

func collectionSubfamilyScore(sub string, want []string, style fontStyle) int {
	switch style {
	case styleBold:
		if strings.Contains(sub, "italic") || strings.Contains(sub, "oblique") {
			return 0
		}
	case styleItalic:
		if strings.Contains(sub, "bold") {
			return 0
		}
	}
	for i, w := range want {
		if sub == w {
			return 100 - i
		}
	}
	for i, w := range want {
		if strings.Contains(sub, w) {
			return 50 - i
		}
	}
	return 0
}

// openBestFaceStyle tries each path in order, reading font bytes transiently
// (not retained after the face opens) before falling back to the embedded
// bundled font for role/style.
func openBestFaceStyle(paths []string, role FontRole, style fontStyle, size, dpi float64) font.Face {
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p) //nolint:gosec // G304: hardcoded system/user font directories
		if err != nil || len(b) == 0 {
			continue
		}
		if f, err := openFaceStyle(b, size, dpi, style, role); err == nil && f != nil {
			return f
		}
	}
	if f, err := openFaceStyle(embeddedFontData(role, style), size, dpi, style, role); err == nil && f != nil {
		return f
	}
	return nil
}

// LoadFontBundle loads role's four style faces (falling back through
// configured family -> platform default -> embedded bundled font per style,
// see fontCandidatePaths) and measures cell metrics from the Regular
// face. configuredFamily is the relevant FontConfig/UIFontConfig.Family
// (empty selects the embedded bundled font directly; "monospace" skips
// straight to the platform default search).
func LoadFontBundle(configuredFamily string, size, scaleFactor float64, role FontRole) FontBundle {
	dpi := 72.0 * scaleFactor
	paths := fontCandidatePaths(configuredFamily, role)

	b := FontBundle{}

	b.Regular = openBestFaceStyle(paths[styleRegular], role, styleRegular, size, dpi)
	b.Bold = openBestFaceStyle(paths[styleBold], role, styleBold, size, dpi)
	b.Italic = openBestFaceStyle(paths[styleItalic], role, styleItalic, size, dpi)
	b.BoldItalic = openBestFaceStyle(paths[styleBoldItalic], role, styleBoldItalic, size, dpi)
	if b.Regular == nil {
		f, err := openFaceStyle(embeddedFontData(role, styleRegular), size, dpi, styleRegular, role)
		if err != nil {
			panic("geckty: failed to load embedded fallback font: " + err.Error())
		}
		b.Regular = f
	}
	m := b.Regular.Metrics()
	b.CellH = m.Height.Ceil()
	b.Ascent = m.Ascent.Ceil()
	if adv, ok := b.Regular.GlyphAdvance('M'); ok {
		b.CellW = adv.Ceil()
	} else {
		b.CellW = b.CellH / 2
	}

	return b
}

// platformSymbolFontPaths lists outline symbol/emoji-capable fonts to try
// when the primary monospace face has no glyph for a rune. Color emoji
// fonts (Apple Color Emoji, Noto Color Emoji) are skipped — golang.org/x/
// image/font only rasterizes outline glyphs.
func platformSymbolFontPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts/Apple Symbols.ttf",
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			"/Library/Fonts/Arial Unicode.ttf",
			"/System/Library/Fonts/LastResort.otf",
		}
	case osWindows:
		winFonts := filepath.Join(os.Getenv("SystemRoot"), "Fonts")
		if winFonts == `\Fonts` {
			winFonts = `C:\Windows\Fonts`
		}
		return []string{
			filepath.Join(winFonts, "seguisym.ttf"),
			filepath.Join(winFonts, "arialuni.ttf"),
			filepath.Join(winFonts, "seguili.ttf"),
		}
	default:
		return []string{
			"/usr/share/fonts/truetype/noto/NotoSansSymbols2-Regular.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansSymbols-Regular.ttf",
			"/usr/share/fonts/truetype/ancient-scripts/Symbola_hint.ttf",
			"/usr/share/fonts/TTF/Symbola.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		}
	}
}

// LoadSymbolFallbackFace opens the first usable platform symbol font at
// the given size/scale. Returns nil when none are installed.
func LoadSymbolFallbackFace(size, scaleFactor float64) font.Face {
	dpi := 72.0 * scaleFactor
	for _, p := range platformSymbolFontPaths() {
		b, err := os.ReadFile(p) //nolint:gosec // G304: hardcoded system font paths
		if err != nil || len(b) == 0 {
			continue
		}
		if f, err := openFace(b, size, dpi, RoleMono); err == nil && f != nil {
			return f
		}
	}
	return nil
}
