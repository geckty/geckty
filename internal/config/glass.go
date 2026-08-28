package config

// Glass blend factors — populated in init() from assets/themes/glass.toml
// (via loadEmbeddedTheme). Hardcoded seeds below are used only if embed
// load fails; they must match glass.toml's [ui.glass] section.
var (
	GlassBarLift   = 0.06
	GlassInactive  = 0.05
	GlassHover     = 0.14
	GlassActive    = 0.30
	GlassDrag      = 0.22
	GlassPlusHover = 0.08
	GlassRim       = 0.70
	GlassRimAlpha  = 0.35
	GlassFillAlpha = 0.78
)

func init() {
	applyGlassFromEmbedded()
}

// applyGlassFromEmbedded overwrites Glass* from the embedded glass theme
// when present. Safe to call from tests that want to re-sync after mutation.
func applyGlassFromEmbedded() {
	tf, ok := loadEmbeddedTheme(ThemeGlass)
	if !ok {
		return
	}
	g := tf.UI.Glass
	setGlassFloat(&GlassBarLift, g.BarLift)
	setGlassFloat(&GlassInactive, g.Inactive)
	setGlassFloat(&GlassHover, g.Hover)
	setGlassFloat(&GlassActive, g.Active)
	setGlassFloat(&GlassDrag, g.Drag)
	setGlassFloat(&GlassPlusHover, g.PlusHover)
	setGlassFloat(&GlassRim, g.Rim)
	setGlassFloat(&GlassRimAlpha, g.RimAlpha)
	setGlassFloat(&GlassFillAlpha, g.FillAlpha)
}

func setGlassFloat(dst *float64, src *float64) {
	if src != nil {
		*dst = *src
	}
}

// ContentBracketsOff is the explicit "disable content brackets" sentinel in
// [ui] content_brackets (also accepted: "none", "false"). Empty means
// "use soft default", not off.
const ContentBracketsOff = "off"

// ThemeGlass is the built-in theme name shipped as assets/themes/glass.toml.
const ThemeGlass = "glass"
