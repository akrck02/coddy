package viewer

import (
	"time"

	ui "github.com/akrck02/go-ui"
)

// hoverPause is how long the pointer rests on a name before its type and documentation are asked.
const hoverPause = 500 * time.Millisecond

// resting is the pointer resting on a place of the code: since when, and whether the window was
// asked about it.
type resting struct {
	place Place
	since time.Time
	on    bool
	asked bool
}

// hoverAt follows the pointer resting on the code: after the pause the window is asked what the
// server says of the name under it, and told when the pointer leaves it.
func (view *Viewer) hoverAt(context *ui.Context, window Window, x, y float32) {
	place, found := view.PlaceAt(x, y)
	found = found && view.List.Area.Contains(x, y) && !window.Keys().Holding()
	rest := &view.rest
	if !found || !rest.on || place != rest.place {
		if rest.asked {
			window.Hover(rest.place, false)
		}
		*rest = resting{place: place, since: time.Now(), on: found}
	}
	if !rest.on || rest.asked {
		return
	}
	if waited := time.Since(rest.since); waited < view.HoverPause {
		context.RequestFrame(view.HoverPause - waited) // a window that draws on demand wakes to ask
		return
	}
	rest.asked = true
	window.Hover(place, true)
}
