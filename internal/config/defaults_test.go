package config

import (
	"runtime"
	"testing"
)

func TestDefaultPopulatesCoreSections(t *testing.T) {
	cfg := Default()
	if cfg.Window.Width != 1000 || cfg.Window.Height != 650 {
		t.Fatalf("window defaults: %+v", cfg.Window)
	}
	if cfg.Colors.Foreground == "" || cfg.Colors.Background == "" {
		t.Fatal("colors should be populated from embedded glass theme")
	}
	if cfg.UI.VisualBell == "" {
		t.Fatal("ui tokens should be populated")
	}
	if len(cfg.Keybindings) == 0 {
		t.Fatal("expected default keybindings")
	}
	if cfg.LogLevel != "error" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
}

func TestDefaultKeybindingsPlatformShape(t *testing.T) {
	cfg := Default()
	var newTab *Keybinding
	for i := range cfg.Keybindings {
		if cfg.Keybindings[i].Action == ActionNewTab {
			newTab = &cfg.Keybindings[i]
			break
		}
	}
	if newTab == nil {
		t.Fatal("missing new tab binding")
	}
	if runtime.GOOS == "darwin" {
		if len(newTab.Mods) != 1 || newTab.Mods[0] != ModCmd {
			t.Fatalf("darwin new tab mods = %v", newTab.Mods)
		}
		return
	}
	if len(newTab.Mods) != 2 {
		t.Fatalf("non-darwin new tab mods = %v", newTab.Mods)
	}
}
