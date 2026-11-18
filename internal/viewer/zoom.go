package viewer

import (
	"math"

	"github.com/akrck02/code-review/internal/pane"
	ui "github.com/akrck02/go-ui"
)

// The code's size: 13 pixels to start with, and how far a zoom may take it.
const (
	defaultCodeSize float32 = 13
	smallestCode    float32 = 8
	largestCode     float32 = 32
)

// Zoom makes the code a pixel larger (step 1) or smaller (−1), or brings it back to its first size
// (0); the line at the top stays at the top.
func (view *Viewer) Zoom(step int) {
	size := defaultCodeSize
	if step != 0 {
		size = min(max(view.codeSize+float32(step), smallestCode), largestCode)
	}
	if size == view.codeSize {
		return
	}
	previous := view.rowHeight
	view.codeSize = size
	// lines keep the room the first size has: 22 pixels for 13
	view.rowHeight = float32(math.Round(float64(size * codeRowHeight / defaultCodeSize)))
	view.List.Offset *= view.rowHeight / previous
	view.List.OffsetX *= view.rowHeight / previous
	view.List.RowHeight = view.rowHeight
}

// CodeSize is the size the code is painted at, in pixels.
func (view *Viewer) CodeSize() float32 { return view.codeSize }

// followZoom zooms while the wheel turns with Command held over the code, a step for each notch:
// a trackpad's small movements add up until they make one.
func (view *Viewer) followZoom(context *ui.Context, keys pane.Keys) {
	x, y := context.Pointer()
	if !keys.Command() || !view.List.Area.Contains(x, y) {
		view.zoomWheel = 0
		return
	}
	_, vertical := keys.Wheel()
	view.zoomWheel += vertical
	for view.zoomWheel >= 1 {
		view.zoomWheel--
		view.Zoom(1)
	}
	for view.zoomWheel <= -1 {
		view.zoomWheel++
		view.Zoom(-1)
	}
}
