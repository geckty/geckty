package chrome

import (
	"fmt"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"

	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/theme"
)

// TabBarWidget renders and dispatches the tab strip using geckty's neutral
// theme tokens. Detailed tab drag handling remains owned by the application
// controller, which is also responsible for session mutation.
type TabBarWidget struct {
	widget.WidgetBase
	Manager  *session.Manager
	Theme    theme.Theme
	OnNew    func()
	OnClose  func(int)
	OnSelect func(int)
}

// NewTabBarWidget constructs a tab-strip widget bound to manager and theme.
func NewTabBarWidget(manager *session.Manager, thm theme.Theme) *TabBarWidget {
	return &TabBarWidget{WidgetBase: *widget.NewWidgetBase(), Manager: manager, Theme: thm}
}

// Layout reserves Height logical pixels for the strip.
func (w *TabBarWidget) Layout(_ widget.Context, c geometry.Constraints) geometry.Size {
	return c.Constrain(geometry.Sz(c.MaxWidth, Height))
}

// Draw paints tab pills and the "+" affordance (retained-mode path).
func (w *TabBarWidget) Draw(_ widget.Context, canvas widget.Canvas) {
	b := w.Bounds()
	pal := w.Theme.Palette
	canvas.DrawRect(b, widget.RGBA8(pal.InactiveTabBG.R, pal.InactiveTabBG.G, pal.InactiveTabBG.B, pal.InactiveTabBG.A))
	if w.Manager == nil {
		return
	}
	tabs := w.Manager.Tabs()
	g := ComputeGeometry(int(b.Width()), len(tabs), MinTabWidth, PlusWidth, CloseZoneWidth)
	active := w.Manager.ActiveID()
	for i, tab := range tabs {
		x := b.Min.X + float32(i*g.TabWidth)
		r := geometry.NewRect(x, b.Min.Y+4, float32(g.TabWidth-3), b.Height()-8)
		bg := pal.InactiveTabBG
		if tab.ID == active {
			bg = pal.ActiveTabBG
		}
		canvas.DrawRoundRect(r, widget.RGBA8(bg.R, bg.G, bg.B, bg.A), 5)
		canvas.DrawText(fmt.Sprintf("Tab %d", tab.ID+1), r, 12,
			widget.RGBA8(pal.Foreground.R, pal.Foreground.G, pal.Foreground.B, pal.Foreground.A),
			false, widget.TextAlignCenter)
	}
	if g.PlusWidth > 0 {
		plus := geometry.NewRect(b.Min.X+float32(g.PlusX), b.Min.Y, float32(g.PlusWidth), b.Height())
		canvas.DrawText("+", plus, 18,
			widget.RGBA8(pal.Foreground.R, pal.Foreground.G, pal.Foreground.B, pal.Foreground.A),
			false, widget.TextAlignCenter)
	}
}

// Event handles left-clicks on tabs / "+" (drag remains app-owned).
func (w *TabBarWidget) Event(_ widget.Context, e event.Event) bool {
	mouse, ok := e.(*event.MouseEvent)
	if !ok || !mouse.IsPress() || !mouse.IsLeftButton() || w.Manager == nil {
		return false
	}
	b := w.Bounds()
	tabs := w.Manager.Tabs()
	g := ComputeGeometry(int(b.Width()), len(tabs), MinTabWidth, PlusWidth, CloseZoneWidth)
	x := int(mouse.Position.X)
	if IsPlusHit(x, g) {
		if w.OnNew != nil {
			w.OnNew()
		}
		return true
	}
	if index := TabAt(x, g, len(tabs)); index >= 0 && index < len(tabs) && w.OnSelect != nil {
		w.OnSelect(tabs[index].ID)
		return true
	}
	return false
}

var _ widget.Widget = (*TabBarWidget)(nil)
