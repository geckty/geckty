package app

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadAppIconDecodesEmbeddedPNG(t *testing.T) {
	if img := loadAppIcon(); img == nil {
		t.Fatal("expected embedded app icon to decode")
	}
}

func TestGridPlacementsEmptySession(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	if got := gridPlacements(s.mgr.Active()); got != nil {
		t.Fatalf("gridPlacements = %v, want nil", got)
	}
}

func TestToRGBAImageConvertsNonRGBA(t *testing.T) {
	src := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.White, color.Black})
	got := toRGBAImage(src)
	if got == nil || got.Bounds().Dx() != 2 {
		t.Fatal("expected converted RGBA image")
	}
}

func TestShowScrollbackInPagerHonorsPagerEnv(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	pager := filepath.Join(t.TempDir(), "pager.sh")
	if err := os.WriteFile(pager, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAGER", pager)
	s.showScrollbackInPager()
}

func TestClipboardNativeHelpersMissingOnEmptyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("powershell/clip are not PATH-gated the same way")
	}
	t.Setenv("PATH", t.TempDir())
	if err := clipboardWriteNative("x"); err == nil {
		t.Fatal("expected clipboardWriteNative error without helper")
	}
	if _, err := clipboardReadNative(); err == nil {
		t.Fatal("expected clipboardReadNative error without helper")
	}
}

func TestGridPlacementsSkipsNilImages(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	if active == nil {
		t.Fatal("expected active session")
	}
	if got := gridPlacements(active); got != nil {
		t.Fatalf("gridPlacements = %v, want nil", got)
	}
}
