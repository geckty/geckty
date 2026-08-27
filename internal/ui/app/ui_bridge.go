package app

import (
	"image"

	"github.com/gogpu/gpucontext"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"

	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/chrome"
	"github.com/geckty/geckty/internal/ui/termview"
	"github.com/geckty/geckty/internal/ui/theme"
)

// shellRoot is the retained widget tree: tab strip, terminal panes, and a
// full-window overlay for search/hints/bell/confirm.
type shellRoot struct {
	widget.WidgetBase
	tabBar  *chrome.TabBarWidget
	panes   *paneHost
	overlay *overlayWidget
	tabH    float32
}

func newShellRoot(mgr *session.Manager, thm theme.Theme) *shellRoot {
	return &shellRoot{
		WidgetBase: *widget.NewWidgetBase(),
		tabBar:     chrome.NewTabBarWidget(mgr, thm),
		panes:      newPaneHost(),
		overlay:    newOverlayWidget(),
		tabH:       float32(chrome.Height),
	}
}

func (w *shellRoot) Children() []widget.Widget {
	return []widget.Widget{w.tabBar, w.panes, w.overlay}
}

func (w *shellRoot) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	sz := c.Biggest()
	w.SetBounds(geometry.NewRect(0, 0, sz.Width, sz.Height))
	tabH := w.tabH
	if tabH > sz.Height {
		tabH = sz.Height
	}
	w.tabBar.Layout(ctx, geometry.Tight(geometry.Sz(sz.Width, tabH)))
	w.tabBar.SetBounds(geometry.NewRect(0, 0, sz.Width, tabH))
	contentH := sz.Height - tabH
	if contentH < 0 {
		contentH = 0
	}
	w.panes.Layout(ctx, geometry.Tight(geometry.Sz(sz.Width, contentH)))
	w.panes.SetBounds(geometry.NewRect(0, tabH, sz.Width, contentH))
	w.overlay.Layout(ctx, geometry.Tight(geometry.Sz(sz.Width, sz.Height)))
	w.overlay.SetBounds(geometry.NewRect(0, 0, sz.Width, sz.Height))
	return sz
}

func (w *shellRoot) Draw(ctx widget.Context, canvas widget.Canvas) {
	w.tabBar.Draw(ctx, canvas)
	w.panes.Draw(ctx, canvas)
	w.overlay.Draw(ctx, canvas)
}

func (w *shellRoot) Event(ctx widget.Context, e event.Event) bool {
	if w.overlay.Event(ctx, e) {
		return true
	}
	if w.tabBar.Event(ctx, e) {
		return true
	}
	return w.panes.Event(ctx, e)
}

var _ widget.Widget = (*shellRoot)(nil)

type paneHost struct {
	widget.WidgetBase
	widgets []*termview.TerminalWidget
	rects   []session.PaneRect
}

func newPaneHost() *paneHost {
	return &paneHost{WidgetBase: *widget.NewWidgetBase()}
}

func (h *paneHost) Children() []widget.Widget {
	out := make([]widget.Widget, len(h.widgets))
	for i, w := range h.widgets {
		out[i] = w
	}
	return out
}

func (h *paneHost) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	sz := c.Biggest()
	h.SetBounds(geometry.NewRect(0, 0, sz.Width, sz.Height))
	for i, tw := range h.widgets {
		if i >= len(h.rects) {
			break
		}
		r := h.rects[i]
		tw.Layout(ctx, geometry.Tight(geometry.Sz(float32(r.W), float32(r.H))))
		tw.SetBounds(geometry.NewRect(float32(r.X), float32(r.Y), float32(r.W), float32(r.H)))
	}
	return sz
}

func (h *paneHost) Draw(ctx widget.Context, canvas widget.Canvas) {
	for _, tw := range h.widgets {
		tw.Draw(ctx, canvas)
	}
}

func (h *paneHost) Event(ctx widget.Context, e event.Event) bool {
	for _, tw := range h.widgets {
		if tw.Event(ctx, e) {
			return true
		}
	}
	return false
}

var _ widget.Widget = (*paneHost)(nil)

type overlayWidget struct {
	widget.WidgetBase
	image *image.RGBA
}

func newOverlayWidget() *overlayWidget {
	return &overlayWidget{WidgetBase: *widget.NewWidgetBase()}
}

func (w *overlayWidget) Layout(_ widget.Context, c geometry.Constraints) geometry.Size {
	return c.Biggest()
}

func (w *overlayWidget) Draw(_ widget.Context, canvas widget.Canvas) {
	if w.image != nil {
		canvas.DrawImage(w.image, w.Bounds().Min)
	}
}

func (*overlayWidget) Event(widget.Context, event.Event) bool { return false }

var _ widget.Widget = (*overlayWidget)(nil)

// Widget tree helpers below are retained for a future gogpu/ui present path.
// The live Backend paints via paintFrame + PresentTexture (see app.go).

func newUIApp(host gpucontext.WindowProvider, mgr *session.Manager, thm theme.Theme) (*uiapp.App, *shellRoot) {
	root := newShellRoot(mgr, thm)
	app := uiapp.New(uiapp.WithWindowProvider(host))
	app.SetRoot(root)
	return app, root
}
