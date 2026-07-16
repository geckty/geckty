package gogpu

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
)

const osWindows = "windows"

// fontBundle holds the primary cell face plus its measured metrics.
type fontBundle struct {
	face   font.Face
	cellW  int
	cellH  int
	ascent int
}

var (
	fontCandidatesOnce  sync.Once
	fontCandidatesCache [][]byte
	fontCandidatesFor   string
)

// systemMonoCandidates returns font file bytes to try, in preference order:
// a configured family (best-effort filename match — see the package's known
// limitation in comparison to gio's real system-font matching), then the
// platform default monospace face, then the embedded Go Mono as a universal
// last resort.
func systemMonoCandidates(configuredFamily string) [][]byte {
	fontCandidatesOnce.Do(func() {
		fontCandidatesFor = configuredFamily
		fontCandidatesCache = loadFontCandidates(configuredFamily)
	})
	if fontCandidatesFor != configuredFamily {
		// Config changed since the cache was built (e.g. in tests) — rebuild.
		fontCandidatesFor = configuredFamily
		fontCandidatesCache = loadFontCandidates(configuredFamily)
	}
	return fontCandidatesCache
}

func loadFontCandidates(configuredFamily string) [][]byte {
	home, _ := os.UserHomeDir()
	var paths []string

	if f := strings.TrimSpace(configuredFamily); f != "" && !strings.EqualFold(f, "monospace") {
		paths = append(paths, configuredFamilyPaths(f, home)...)
	}

	switch runtime.GOOS {
	case osWindows:
		winFonts := filepath.Join(os.Getenv("SystemRoot"), "Fonts")
		if winFonts == `\Fonts` {
			winFonts = `C:\Windows\Fonts`
		}
		paths = append(paths,
			filepath.Join(winFonts, "consola.ttf"),  // Consolas regular
			filepath.Join(winFonts, "consolab.ttf"), // Consolas bold
			filepath.Join(winFonts, "lucon.ttf"),    // Lucida Console fallback
		)
	case "darwin":
		paths = append(paths,
			"/System/Library/Fonts/Menlo.ttc",
			"/System/Library/Fonts/SFNSMono.ttf",
		)
	default:
		paths = append(paths,
			filepath.Join(home, ".local/share/fonts/DejaVuSansMono.ttf"),
			"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf",
			"/usr/share/fonts/TTF/DejaVuSansMono.ttf",
		)
	}

	var out [][]byte
	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p) //nolint:gosec // G304: hardcoded system/user font directories
		if err != nil || len(data) == 0 {
			continue
		}
		out = append(out, data)
	}
	// Go Mono is always available as the embedded universal fallback.
	out = append(out, gomono.TTF)
	return out
}

// configuredFamilyPaths guesses filenames for a user-configured font family
// name — best-effort, since (unlike gio's system-font matching) there is no
// real font-enumeration API used here, only common naming conventions.
func configuredFamilyPaths(family, home string) []string {
	stripped := strings.ReplaceAll(family, " ", "")
	names := []string{
		family + ".ttf", family + ".ttc",
		stripped + ".ttf", stripped + ".ttc",
		stripped + "-Regular.ttf", stripped + "-Medium.ttf",
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

// loadFontBundle loads the best available monospace face and measures its
// cell metrics. configuredFamily is geckty's cfg.Font.Family (may be empty
// or "monospace" for the platform default).
func loadFontBundle(configuredFamily string, size, scaleFactor float64) fontBundle {
	dpi := 72.0 * scaleFactor
	var primary font.Face

	for _, data := range systemMonoCandidates(configuredFamily) {
		f, err := openFace(data, size, dpi)
		if err != nil || f == nil {
			continue
		}
		primary = f
		break
	}
	if primary == nil {
		f, err := openFace(gomono.TTF, size, dpi)
		if err != nil {
			panic("gogpu: failed to load any monospace font: " + err.Error())
		}
		primary = f
	}

	m := primary.Metrics()
	b := fontBundle{face: primary, cellH: m.Height.Ceil(), ascent: m.Ascent.Ceil()}
	if adv, ok := primary.GlyphAdvance('M'); ok {
		b.cellW = adv.Ceil()
	} else {
		b.cellW = b.cellH / 2
	}
	return b
}
