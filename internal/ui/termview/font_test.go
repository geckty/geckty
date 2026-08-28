package termview

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/image/font"
)

func TestConfiguredFamilyPathsIncludesNameVariants(t *testing.T) {
	paths := configuredFamilyPaths("My Font", "/home/user")
	joined := strings.Join(paths, "\n")
	for _, want := range []string{"My Font.ttf", "My Font.ttc", "MyFont.ttf", "MyFont-Regular.ttf", "MyFont-Medium.ttf"} {
		if !strings.Contains(joined, want) {
			t.Errorf("configuredFamilyPaths(%q) missing filename variant %q; got %v", "My Font", want, paths)
		}
	}
}

func TestConfiguredFamilyPathsUsesPlatformFontDir(t *testing.T) {
	paths := configuredFamilyPaths("Foo", "/home/user")
	if len(paths) == 0 {
		t.Fatal("expected at least one candidate path")
	}

	var wantDir string
	switch runtime.GOOS {
	case "darwin":
		wantDir = filepath.Join("/home/user", "Library", "Fonts")
	case osWindows:
		// Falls back to C:\Windows\Fonts when SystemRoot isn't set — either
		// way every candidate is rooted at some *\Fonts directory.
		wantDir = "Fonts"
	default:
		wantDir = filepath.Join("/home/user", ".local/share/fonts")
	}

	found := false
	for _, p := range paths {
		if strings.Contains(p, wantDir) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("configuredFamilyPaths on %s = %v, want a path containing %q", runtime.GOOS, paths, wantDir)
	}
}

func TestConfiguredFamilyPathsEmptyFamilyStillProducesDirCandidates(t *testing.T) {
	paths := configuredFamilyPaths("", "/home/user")
	if len(paths) == 0 {
		t.Fatal("expected candidate paths even for an empty family name")
	}
}

func TestConfiguredFamilyStylePathsBoldItalic(t *testing.T) {
	bold := configuredFamilyStylePaths("JetBrains Mono", "/home/u", styleBold)
	italic := configuredFamilyStylePaths("JetBrains Mono", "/home/u", styleItalic)
	bi := configuredFamilyStylePaths("JetBrains Mono", "/home/u", styleBoldItalic)
	for _, paths := range [][]string{bold, italic, bi} {
		if len(paths) == 0 {
			t.Fatal("expected style-specific candidate paths")
		}
	}
	joined := strings.Join(bold, "\n")
	if !strings.Contains(joined, "JetBrainsMono-Bold.ttf") {
		t.Fatalf("bold paths missing Bold variant: %v", bold)
	}
}

func TestPlatformStyleCandidatesNonEmpty(t *testing.T) {
	for _, role := range []FontRole{RoleMono, RoleUI} {
		for _, style := range []fontStyle{styleRegular, styleBold, styleItalic, styleBoldItalic} {
			paths := platformStyleCandidates(style, role)
			if len(paths) == 0 {
				t.Fatalf("platformStyleCandidates(%v,%v) empty", style, role)
			}
		}
	}
}

func TestLoadFontCandidatesIncludesEmbeddedFallback(t *testing.T) {
	cands := loadFontCandidates("Nonexistent Font Family XYZ", RoleMono)
	for _, style := range []fontStyle{styleRegular, styleBold, styleItalic, styleBoldItalic} {
		if len(cands[style]) == 0 {
			t.Fatalf("style %v: expected candidates including embedded fallback", style)
		}
		// Last entry is always the embedded font bytes.
		last := cands[style][len(cands[style])-1]
		if len(last) == 0 {
			t.Fatalf("style %v: embedded fallback empty", style)
		}
	}
}

func TestLoadFontBundleEmptyFamilyUsesEmbeddedFontsForEveryStyle(t *testing.T) {
	for _, role := range []FontRole{RoleMono, RoleUI} {
		b := LoadFontBundle("", 13, 1, role)
		if b.Regular == nil || b.Bold == nil || b.Italic == nil || b.BoldItalic == nil {
			t.Fatalf("role %v: LoadFontBundle with empty Family should resolve all 4 styles from the embedded fonts, got %+v", role, b)
		}
		if b.CellW <= 0 || b.CellH <= 0 {
			t.Fatalf("role %v: LoadFontBundle should measure cell metrics from the regular face", role)
		}
	}
}

func TestFontBundleFaceFallsBackToRegularWhenStyleMissing(t *testing.T) {
	regular := LoadFontBundle("", 13, 1, RoleMono).Regular
	b := FontBundle{Regular: regular} // bold/italic/boldItalic deliberately unset

	for _, tc := range []struct{ bold, italic bool }{
		{true, false}, {false, true}, {true, true},
	} {
		if got, idx := b.face(tc.bold, tc.italic); got != regular || idx != 0 {
			t.Fatalf("face(%v,%v) = (%p, %d), want (regular, 0) when that style has no dedicated face", tc.bold, tc.italic, got, idx)
		}
	}
}

func TestFontBundleFaceUsesDedicatedStyleWhenPresent(t *testing.T) {
	b := LoadFontBundle("", 13, 1, RoleMono) // embedded fonts provide all 4 styles

	cases := []struct {
		bold, italic bool
		want         font.Face
		wantIdx      int
	}{
		{false, false, b.Regular, 0},
		{true, false, b.Bold, 1},
		{false, true, b.Italic, 2},
		{true, true, b.BoldItalic, 3},
	}
	for _, tc := range cases {
		got, idx := b.face(tc.bold, tc.italic)
		if got != tc.want || idx != tc.wantIdx {
			t.Fatalf("face(%v,%v) = (%p, %d), want (%p, %d)", tc.bold, tc.italic, got, idx, tc.want, tc.wantIdx)
		}
	}
}
