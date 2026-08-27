package config

import "github.com/BurntSushi/toml"

type colorStringField struct {
	key string
	get func(ColorsConfig) string
	set func(*ColorsConfig, string)
}

var colorStringFields = []colorStringField{
	{"foreground", func(c ColorsConfig) string { return c.Foreground }, func(c *ColorsConfig, v string) { c.Foreground = v }},
	{"background", func(c ColorsConfig) string { return c.Background }, func(c *ColorsConfig, v string) { c.Background = v }},
	{"selection", func(c ColorsConfig) string { return c.Selection }, func(c *ColorsConfig, v string) { c.Selection = v }},
	{"selection_background", func(c ColorsConfig) string { return c.SelectionBackground }, func(c *ColorsConfig, v string) { c.SelectionBackground = v }},
	{"selection_foreground", func(c ColorsConfig) string { return c.SelectionForeground }, func(c *ColorsConfig, v string) { c.SelectionForeground = v }},
	{"cursor", func(c ColorsConfig) string { return c.Cursor }, func(c *ColorsConfig, v string) { c.Cursor = v }},
	{"active_tab_foreground", func(c ColorsConfig) string { return c.ActiveTabForeground }, func(c *ColorsConfig, v string) { c.ActiveTabForeground = v }},
	{"active_tab_background", func(c ColorsConfig) string { return c.ActiveTabBackground }, func(c *ColorsConfig, v string) { c.ActiveTabBackground = v }},
	{"inactive_tab_foreground", func(c ColorsConfig) string { return c.InactiveTabForeground }, func(c *ColorsConfig, v string) { c.InactiveTabForeground = v }},
	{"inactive_tab_background", func(c ColorsConfig) string { return c.InactiveTabBackground }, func(c *ColorsConfig, v string) { c.InactiveTabBackground = v }},
	{"tab_bar_background", func(c ColorsConfig) string { return c.TabBarBackground }, func(c *ColorsConfig, v string) { c.TabBarBackground = v }},
	{"hover_tab_background", func(c ColorsConfig) string { return c.HoverTabBackground }, func(c *ColorsConfig, v string) { c.HoverTabBackground = v }},
	{"plus_button_background", func(c ColorsConfig) string { return c.PlusButtonBackground }, func(c *ColorsConfig, v string) { c.PlusButtonBackground = v }},
}

type uiStringField struct {
	key string
	get func(UIConfig) string
	set func(*UIConfig, string)
}

var uiStringFields = []uiStringField{
	{"visual_bell", func(u UIConfig) string { return u.VisualBell }, func(u *UIConfig, v string) { u.VisualBell = v }},
	{"scrollbar_track", func(u UIConfig) string { return u.ScrollbarTrack }, func(u *UIConfig, v string) { u.ScrollbarTrack = v }},
	{"scrollbar_thumb", func(u UIConfig) string { return u.ScrollbarThumb }, func(u *UIConfig, v string) { u.ScrollbarThumb = v }},
	{"url_underline", func(u UIConfig) string { return u.URLUnderline }, func(u *UIConfig, v string) { u.URLUnderline = v }},
	{"search_match", func(u UIConfig) string { return u.SearchMatch }, func(u *UIConfig, v string) { u.SearchMatch = v }},
	{"hint_label_bg", func(u UIConfig) string { return u.HintLabelBG }, func(u *UIConfig, v string) { u.HintLabelBG = v }},
	{"hint_label_fg", func(u UIConfig) string { return u.HintLabelFG }, func(u *UIConfig, v string) { u.HintLabelFG = v }},
	{"pane_focus_border", func(u UIConfig) string { return u.PaneFocusBorder }, func(u *UIConfig, v string) { u.PaneFocusBorder = v }},
	{"command_running", func(u UIConfig) string { return u.CommandRunning }, func(u *UIConfig, v string) { u.CommandRunning = v }},
	{"command_success", func(u UIConfig) string { return u.CommandSuccess }, func(u *UIConfig, v string) { u.CommandSuccess = v }},
	{"command_failed", func(u UIConfig) string { return u.CommandFailed }, func(u *UIConfig, v string) { u.CommandFailed = v }},
}

type uiBoolField struct {
	key string
	get func(UIConfig) *bool
	set func(*UIConfig, *bool)
}

var uiBoolFields = []uiBoolField{
	{"command_border_enabled", func(u UIConfig) *bool { return u.CommandBorderEnabled }, func(u *UIConfig, v *bool) { u.CommandBorderEnabled = v }},
	{"command_dot_enabled", func(u UIConfig) *bool { return u.CommandDotEnabled }, func(u *UIConfig, v *bool) { u.CommandDotEnabled = v }},
}

type glassFloatField struct {
	key string
	get func(GlassConfig) *float64
	set func(*GlassConfig, *float64)
}

var glassFloatFields = []glassFloatField{
	{"bar_lift", func(g GlassConfig) *float64 { return g.BarLift }, func(g *GlassConfig, v *float64) { g.BarLift = v }},
	{"inactive", func(g GlassConfig) *float64 { return g.Inactive }, func(g *GlassConfig, v *float64) { g.Inactive = v }},
	{"hover", func(g GlassConfig) *float64 { return g.Hover }, func(g *GlassConfig, v *float64) { g.Hover = v }},
	{"active", func(g GlassConfig) *float64 { return g.Active }, func(g *GlassConfig, v *float64) { g.Active = v }},
	{"drag", func(g GlassConfig) *float64 { return g.Drag }, func(g *GlassConfig, v *float64) { g.Drag = v }},
	{"plus_hover", func(g GlassConfig) *float64 { return g.PlusHover }, func(g *GlassConfig, v *float64) { g.PlusHover = v }},
	{"rim", func(g GlassConfig) *float64 { return g.Rim }, func(g *GlassConfig, v *float64) { g.Rim = v }},
	{"rim_alpha", func(g GlassConfig) *float64 { return g.RimAlpha }, func(g *GlassConfig, v *float64) { g.RimAlpha = v }},
	{"fill_alpha", func(g GlassConfig) *float64 { return g.FillAlpha }, func(g *GlassConfig, v *float64) { g.FillAlpha = v }},
}

func mergeColors(base, over ColorsConfig) ColorsConfig {
	out := base
	for _, f := range colorStringFields {
		if v := f.get(over); v != "" {
			f.set(&out, v)
		}
	}
	if ansiNonEmpty(over.ANSI) {
		out.ANSI = over.ANSI
	}
	out.Preset = ""
	return out
}

func mergeUI(base, over UIConfig) UIConfig {
	out := base
	for _, f := range uiStringFields {
		if v := f.get(over); v != "" {
			f.set(&out, v)
		}
	}
	for _, f := range uiBoolFields {
		if p := f.get(over); p != nil {
			v := *p
			f.set(&out, &v)
		}
	}
	if over.ContentBrackets != "" {
		out.ContentBrackets = over.ContentBrackets
	}
	out.Glass = mergeGlass(base.Glass, over.Glass)
	return out
}

func mergeGlass(base, over GlassConfig) GlassConfig {
	out := base
	for _, f := range glassFloatFields {
		if p := f.get(over); p != nil {
			v := *p
			f.set(&out, &v)
		}
	}
	return out
}

func colorsOverridesFrom(md toml.MetaData, decoded ColorsConfig) ColorsConfig {
	var over ColorsConfig
	defined := func(key string) bool { return md.IsDefined("colors", key) }
	for _, f := range colorStringFields {
		if defined(f.key) {
			f.set(&over, f.get(decoded))
		}
	}
	if defined("ansi") {
		over.ANSI = decoded.ANSI
	}
	return over
}

func uiOverridesFrom(md toml.MetaData, decoded UIConfig) UIConfig {
	var over UIConfig
	defined := func(key string) bool { return md.IsDefined("ui", key) }
	for _, f := range uiStringFields {
		if defined(f.key) {
			f.set(&over, f.get(decoded))
		}
	}
	for _, f := range uiBoolFields {
		if defined(f.key) {
			f.set(&over, f.get(decoded))
		}
	}
	if defined("content_brackets") {
		if decoded.ContentBrackets == "" {
			over.ContentBrackets = ContentBracketsOff
		} else {
			over.ContentBrackets = decoded.ContentBrackets
		}
	}
	for _, f := range glassFloatFields {
		if md.IsDefined("ui", "glass", f.key) {
			f.set(&over.Glass, f.get(decoded.Glass))
		}
	}
	return over
}
