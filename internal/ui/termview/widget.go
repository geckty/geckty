package termview

import (
	"image"
	"math"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"

	"github.com/geckty/geckty/internal/session"
	"github.com/geckty/geckty/internal/ui/theme"
)

// TerminalWidget is the retained-mode bridge for the terminal grid. Painter
// deliberately remains the terminal renderer: it rasterizes to an RGBA image
// on the CPU, which the ui canvas then composites in the widget tree.
type TerminalWidget struct {
	widget.WidgetBase
	Session    *session.Session
	Painter    *Painter
	Theme      theme.Theme
	BlinkOn    func() bool
	Selection  Selection
	Placements []Placement

	image *image.RGBA
}

// NewTerminalWidget constructs a visible, enabled terminal leaf.
func NewTerminalWidget(sess *session.Session, painter *Painter, thm theme.Theme) *TerminalWidget {
	return &TerminalWidget{
		WidgetBase: *widget.NewWidgetBase(),
		Session:    sess,
		Painter:    painter,
		Theme:      thm,
	}
}

// Layout expands to the largest size the parent allows.
func (w *TerminalWidget) Layout(_ widget.Context, c geometry.Constraints) geometry.Size {
	return c.Biggest()
}

// Draw rasterizes the session grid via Painter into an RGBA leaf image.
func (w *TerminalWidget) Draw(_ widget.Context, canvas widget.Canvas) {
	if w.Session == nil || w.Painter == nil {
		return
	}
	bounds := w.Bounds()
	width, height := int(math.Ceil(float64(bounds.Width()))), int(math.Ceil(float64(bounds.Height())))
	if width <= 0 || height <= 0 {
		return
	}
	if w.image == nil || w.image.Bounds().Dx() != width || w.image.Bounds().Dy() != height {
		w.image = image.NewRGBA(image.Rect(0, 0, width, height))
	}
	blinkOn := true
	if w.BlinkOn != nil {
		blinkOn = w.BlinkOn()
	}
	// Clear to theme background then paint the grid at (0,0) of this leaf.
	bg := ToRGBA(w.Theme.Palette.Background)
	for i := 0; i+3 < len(w.image.Pix); i += 4 {
		w.image.Pix[i] = bg.R
		w.image.Pix[i+1] = bg.G
		w.image.Pix[i+2] = bg.B
		w.image.Pix[i+3] = bg.A
	}
	w.Painter.Paint(w.image.Pix, width, height, 0, 0, w.Session.Term,
		w.Session.ScrollOffset(), w.Selection, w.Placements, blinkOn, nil, 0, 0)
	canvas.DrawImage(w.image, bounds.Min)
}

// Event keeps terminal leaves focusable. Keyboard dispatch remains in app so
// platform-specific gogpu EventSource handling stays centralized.
func (w *TerminalWidget) Event(ctx widget.Context, e event.Event) bool {
	if _, ok := e.(*event.MouseEvent); ok {
		ctx.RequestFocus(w)
		return true
	}
	return false
}

var _ widget.Widget = (*TerminalWidget)(nil)
