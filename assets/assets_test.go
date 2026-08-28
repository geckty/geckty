package assets

import (
	"io/fs"
	"testing"
)

func TestEmbeddedThemesContainGlass(t *testing.T) {
	data, err := fs.ReadFile(Themes, "themes/glass.toml")
	if err != nil {
		t.Fatalf("read glass theme: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("glass.toml is empty")
	}
}

func TestEmbeddedFontsPresent(t *testing.T) {
	entries, err := fs.ReadDir(Fonts, "fonts/mono")
	if err != nil {
		t.Fatalf("read fonts/mono: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected bundled mono fonts")
	}
}

func TestEmbeddedIconPresent(t *testing.T) {
	if len(Icon) < 8 {
		t.Fatalf("icon PNG too small: %d bytes", len(Icon))
	}
	// PNG magic
	if Icon[0] != 0x89 || Icon[1] != 'P' || Icon[2] != 'N' || Icon[3] != 'G' {
		t.Fatal("Icon is not a PNG")
	}
}
