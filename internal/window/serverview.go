package window

import (
	"github.com/akrck02/code-review/internal/servers"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// dotColour is the colour of a server's state: green when ready, the accent while it works, quiet
// while it starts or rests, the warning's when it cannot run.
func dotColour(palette ui.Theme, state servers.State) draw2d.Color {
	switch state {
	case servers.Ready:
		return palette.Positive
	case servers.Working:
		return palette.Primary
	case servers.Broken:
		return palette.Warning
	}
	return palette.InkQuiet
}

// buildDot is a server's state as a small round mark.
func buildDot(context *ui.Context, state servers.State, size float32) {
	colour := dotColour(context.Theme, state)
	context.Paint(ui.Layout{Width: ui.Fixed(size), Height: ui.Fixed(size)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		if state == servers.Resting {
			drawing.Rectangle(area, size/2, draw2d.Color{}, colour, 1.5)
			return
		}
		drawing.Rectangle(area, size/2, colour, draw2d.Color{}, 0)
	})
}

// buildBar is how far a server is, as a thin bar across its card.
func (application *Window) buildBar(context *ui.Context, fraction float32, colour draw2d.Color) {
	palette := context.Theme
	context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(4)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		drawing.Rectangle(area, 2, application.look.Hover.Mix(palette.Ink, 0.12), draw2d.Color{}, 0)
		if fraction > 0 {
			done := area
			done.Width = area.Width * min(fraction, 1)
			drawing.Rectangle(done, 2, colour, draw2d.Color{}, 0)
		}
	})
}
