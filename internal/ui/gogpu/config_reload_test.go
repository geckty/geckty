package gogpu

import (
	"testing"

	"github.com/gogpu/gpucontext"

	"golang.org/x/image/font/basicfont"

	"github.com/geckty/geckty/internal/config"
)

func TestApplyConfigUpdatesCfgPaletteAndKeymap(t *testing.T) {
	s, _ := testUIState(t)
	s.painter.Fonts.regular = basicfont.Face7x13

	newCfg := config.Default()
	newCfg.Colors.Preset = ""
	newCfg.Colors.Foreground = "#ffffff"
	newCfg.Colors.Background = "#000000"
	newCfg.Colors.ANSI = [16]string{
		"#000000", "#ff0000", "#00ff00", "#ffff00",
		"#0000ff", "#ff00ff", "#00ffff", "#ffffff",
		"#000000", "#ff0000", "#00ff00", "#ffff00",
		"#0000ff", "#ff00ff", "#00ffff", "#ffffff",
	}
	newCfg.Keybindings = []config.Keybinding{
		{Key: "N", Mods: []string{"ctrl", "shift"}, Action: string(ActionNewTab)},
	}

	s.applyConfig(newCfg)

	if s.cfg != newCfg {
		t.Fatal("applyConfig should replace s.cfg")
	}
	if s.painter.Fonts.regular != nil {
		t.Fatal("applyConfig should clear the cached font face so ensureFonts reloads it")
	}
	if s.palette.Foreground.R != 0xff || s.palette.Foreground.G != 0xff || s.palette.Foreground.B != 0xff {
		t.Fatalf("applyConfig should have rebuilt the palette from the new config, got %+v", s.palette.Foreground)
	}
	if a, ok := s.keymap.Match(gpucontext.KeyN, gpucontext.ModControl|gpucontext.ModShift); !ok || a != ActionNewTab {
		t.Fatalf("applyConfig should have rebuilt the keymap, Match = %q, %v", a, ok)
	}
}

func TestApplyConfigKeepsPreviousPaletteOnInvalidColors(t *testing.T) {
	s, _ := testUIState(t)
	before := s.palette

	newCfg := config.Default()
	newCfg.Colors.Preset = ""
	newCfg.Colors.Foreground = "not-a-color"

	s.applyConfig(newCfg)

	if s.cfg != newCfg {
		t.Fatal("applyConfig should still swap s.cfg even when colors are invalid")
	}
	if s.palette != before {
		t.Fatal("applyConfig should keep the previous palette when the new colors fail to parse")
	}
}

func TestApplyPendingConfigIsNoopWhenNothingPending(t *testing.T) {
	s, _ := testUIState(t)
	before := s.cfg
	s.applyPendingConfig()
	if s.cfg != before {
		t.Fatal("applyPendingConfig should not change s.cfg when nothing is pending")
	}
}

func TestApplyPendingConfigDrainsStoredConfig(t *testing.T) {
	s, _ := testUIState(t)
	newCfg := config.Default()
	newCfg.Font.Size = 42
	s.pendingCfg.Store(newCfg)

	s.applyPendingConfig()

	if s.cfg != newCfg {
		t.Fatal("applyPendingConfig should apply the stored config")
	}
	if s.pendingCfg.Load() != nil {
		t.Fatal("applyPendingConfig should drain pendingCfg")
	}
}

func TestWireConfigReloadNoopWhenHotReloadDisabled(t *testing.T) {
	s, app := testUIState(t)
	s.cfg = config.Default()
	s.cfg.HotReload = false

	// s.cfg has no source path in this test setup either, so
	// config.Config.Watch itself would already be a no-op; this exercises
	// the HotReload=false short-circuit in wireConfigReload specifically.
	stop := s.wireConfigReload(app)
	stop()
}
