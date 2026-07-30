package gogpu

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

func TestLoadFontBundleEmptyFamilyUsesEmbeddedFontsForEveryStyle(t *testing.T) {
	for _, role := range []fontRole{roleMono, roleUI} {
		b := loadFontBundle("", 13, 1, role)
		if b.regular == nil || b.bold == nil || b.italic == nil || b.boldItalic == nil {
			t.Fatalf("role %v: loadFontBundle with empty Family should resolve all 4 styles from the embedded fonts, got %+v", role, b)
		}
		if b.cellW <= 0 || b.cellH <= 0 {
			t.Fatalf("role %v: loadFontBundle should measure cell metrics from the regular face", role)
		}
	}
}

func TestFontBundleFaceFallsBackToRegularWhenStyleMissing(t *testing.T) {
	regular := loadFontBundle("", 13, 1, roleMono).regular
	b := fontBundle{regular: regular} // bold/italic/boldItalic deliberately unset

	for _, tc := range []struct{ bold, italic bool }{
		{true, false}, {false, true}, {true, true},
	} {
		if got, idx := b.face(tc.bold, tc.italic); got != regular || idx != 0 {
			t.Fatalf("face(%v,%v) = (%p, %d), want (regular, 0) when that style has no dedicated face", tc.bold, tc.italic, got, idx)
		}
	}
}

func TestFontBundleFaceUsesDedicatedStyleWhenPresent(t *testing.T) {
	b := loadFontBundle("", 13, 1, roleMono) // embedded fonts provide all 4 styles

	cases := []struct {
		bold, italic bool
		want         font.Face
		wantIdx      int
	}{
		{false, false, b.regular, 0},
		{true, false, b.bold, 1},
		{false, true, b.italic, 2},
		{true, true, b.boldItalic, 3},
	}
	for _, tc := range cases {
		got, idx := b.face(tc.bold, tc.italic)
		if got != tc.want || idx != tc.wantIdx {
			t.Fatalf("face(%v,%v) = (%p, %d), want (%p, %d)", tc.bold, tc.italic, got, idx, tc.want, tc.wantIdx)
		}
	}
}
