package gogpu

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
