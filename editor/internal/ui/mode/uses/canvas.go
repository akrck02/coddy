package uses

import (
	"math"

	"github.com/akrck02/coddy/editor/internal/ui/mode"
	"github.com/akrck02/coddy/editor/internal/ui/pane"
	"github.com/akrck02/coddy/editor/internal/ui/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// A graph is drawn on a canvas: boxes and the arrows between them, laid out in the canvas's own
// units, shown zoomed and moved. The wheel zooms around the pointer, dragging moves it, 0 fits it
// in the window again, and a click on a box tells which.

// boxKind is how a box is drawn.
type boxKind int

const (
	plainBox   boxKind = iota
	groupBox           // a project holding files: a frame, its name on top
	centreBox          // the name searched
	changedBox         // a project with changes
	litBox             // a project, or a file, that uses the name
	litChanged         // both
)

// box is a box of a graph, in the canvas's units; target is what a click on it means to the
// graph, -1 for nothing. A box that reveals opens its use instead.
type box struct {
	x, y, width, height float32
	label, detail       string
	kind                boxKind
	target              int
	reveals             bool
	use                 mode.Use
}

// arrow goes from a box to another, by their places in the scene.
type arrow struct{ from, to int }

// scene is what a graph draws.
type scene struct {
	boxes  []box
	arrows []arrow
}

// canvas is where a graph is shown and how: its zoom and where it was moved to.
type canvas struct {
	zoom, panX, panY float32
	fitted           bool
	area             draw2d.Rectangle
	dragging         bool
	lastX, lastY     float32
	hovered          int
	pieces           map[pieceKey]*draw2d.Image // the arrows' pieces drawn, by zoom level (arrows.go)
}

const (
	boxText     = 12 // the size of a box's label, in the canvas's units
	fitMargin   = 40
	largestZoom = 2
	wheelZoom   = 1.12
)

// textSizes are the sizes a box's text is drawn at, whatever the zoom: the renderer draws each
// letter once per size and keeps it, so a size for every step of the wheel drew every letter
// again on each step, and sent the letters' picture to the graphics card again, megabytes a frame.
var textSizes = []float32{6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 18, 20, 22, 24}

// textSize is the size a box's text is drawn at for a zoom: the largest of textSizes that is not
// larger than the label scaled, so it stays in its box; 0 when it would be too small to read.
func textSize(zoom float32) float32 {
	scaled, chosen := boxText*zoom, float32(0)
	for _, size := range textSizes {
		if size <= scaled {
			chosen = size
		}
	}
	return chosen
}

// textWidth is how wide a label is drawn, roughly, in the canvas's units.
func textWidth(text string) float32 { return float32(len([]rune(text))) * 7.2 }

// fit zooms and moves the canvas so the whole scene is in sight.
func (canvas *canvas) fit(graph scene) {
	if len(graph.boxes) == 0 || canvas.area.Width <= 0 {
		return
	}
	left, top := float32(math.MaxFloat32), float32(math.MaxFloat32)
	right, bottom := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for _, each := range graph.boxes {
		left, top = min(left, each.x), min(top, each.y)
		right, bottom = max(right, each.x+each.width), max(bottom, each.y+each.height)
	}
	width, height := max(right-left, 1), max(bottom-top, 1)
	canvas.zoom = min((canvas.area.Width-2*fitMargin)/width, (canvas.area.Height-2*fitMargin)/height, largestZoom)
	canvas.panX = canvas.area.Width/2 - (left+width/2)*canvas.zoom
	canvas.panY = canvas.area.Height/2 - (top+height/2)*canvas.zoom
	canvas.fitted = true
}

// toScreen is where a point of the canvas is on the screen.
func (canvas *canvas) toScreen(x, y float32) (float32, float32) {
	return canvas.area.X + canvas.panX + x*canvas.zoom, canvas.area.Y + canvas.panY + y*canvas.zoom
}

// boxAt is the box under a point of the screen, by its place in the scene; -1 for none.
func (canvas *canvas) boxAt(graph scene, x, y float32) int {
	for index := len(graph.boxes) - 1; index >= 0; index-- {
		each := graph.boxes[index]
		left, top := canvas.toScreen(each.x, each.y)
		if x >= left && y >= top && x < left+each.width*canvas.zoom && y < top+each.height*canvas.zoom && each.kind != groupBox {
			return index
		}
	}
	return -1
}

// follow zooms with the wheel, moves with a drag, and tells which box was clicked; -1 for none.
func (canvas *canvas) follow(context *ui.Context, keys pane.Keys, graph scene) int {
	x, y := context.Pointer()
	inside := canvas.area.Contains(x, y)
	canvas.hovered = -1
	if inside {
		canvas.hovered = canvas.boxAt(graph, x, y)
	}
	if _, wheel := keys.Wheel(); inside && wheel != 0 {
		factor := float32(math.Pow(wheelZoom, float64(wheel)))
		zoom := min(max(canvas.zoom*factor, 0.05), largestZoom*2)
		localX, localY := x-canvas.area.X, y-canvas.area.Y
		canvas.panX = localX - (localX-canvas.panX)*zoom/canvas.zoom
		canvas.panY = localY - (localY-canvas.panY)*zoom/canvas.zoom
		canvas.zoom = zoom
	}
	clicked := -1
	switch {
	case keys.Clicked() && inside:
		clicked = canvas.hovered
		canvas.dragging, canvas.lastX, canvas.lastY = clicked < 0, x, y
	case canvas.dragging && keys.Holding():
		canvas.panX, canvas.panY = canvas.panX+x-canvas.lastX, canvas.panY+y-canvas.lastY
		canvas.lastX, canvas.lastY = x, y
	default:
		canvas.dragging = false
	}
	return clicked
}

// build shows a scene on the canvas, filling the room left, and tells which box was clicked.
func (canvas *canvas) build(context *ui.Context, keys pane.Keys, look theme.Look, graph scene) int {
	clicked := -1
	if canvas.area.Width > 0 {
		if !canvas.fitted {
			canvas.fit(graph)
		}
		clicked = canvas.follow(context, keys, graph)
	}
	context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		if area.Width != canvas.area.Width || area.Height != canvas.area.Height {
			canvas.fitted = false // a new size: fitted again next frame
		}
		canvas.area = area
		drawing.PushClip(area)
		canvas.paint(drawing, look, graph)
		drawing.PopClip()
	})
	return clicked
}

// paint draws the groups, then the arrows, then the boxes over them.
func (canvas *canvas) paint(drawing *draw2d.List, look theme.Look, graph scene) {
	for index, each := range graph.boxes {
		if each.kind == groupBox {
			canvas.paintBox(drawing, look, each, index == canvas.hovered)
		}
	}
	for _, each := range graph.arrows {
		canvas.paintArrow(drawing, look, graph.boxes[each.from], graph.boxes[each.to])
	}
	for index, each := range graph.boxes {
		if each.kind != groupBox {
			canvas.paintBox(drawing, look, each, index == canvas.hovered)
		}
	}
}

// paintBox draws a box: its fill and edge by its kind, its label and its detail.
func (canvas *canvas) paintBox(drawing *draw2d.List, look theme.Look, each box, hovered bool) {
	palette := look.Theme
	left, top := canvas.toScreen(each.x, each.y)
	area := draw2d.Rectangle{X: left, Y: top, Width: each.width * canvas.zoom, Height: each.height * canvas.zoom}
	fill, edge, ink := palette.Surface, palette.Line, palette.Ink
	switch each.kind {
	case groupBox:
		fill, ink = look.Editor, palette.InkSoft
	case centreBox:
		fill, edge, ink = palette.Primary, palette.Primary, palette.OnPrimary
	case changedBox:
		edge = look.Modified
	case litBox:
		fill, edge = palette.PrimaryTint, palette.Primary
	case litChanged:
		fill, edge = palette.PrimaryTint, look.Modified
	}
	if hovered {
		fill = look.Hover
	}
	drawing.Rectangle(area, 6*canvas.zoom, fill, edge, max(canvas.zoom, 1))
	size := textSize(canvas.zoom)
	if size == 0 {
		return // too small to read
	}
	baseline := top + (each.height*canvas.zoom+size*0.7)/2
	if each.kind == groupBox {
		baseline = top + size*1.6
	}
	drawing.Text(each.label, left+10*canvas.zoom, baseline, size, draw2d.Medium, ink)
	if each.detail != "" {
		x := left + area.Width - (textWidth(each.detail)+10)*canvas.zoom
		drawing.Text(each.detail, x, baseline, size, draw2d.Mono, palette.InkQuiet)
	}
}

// edgeToward is where the line from a box's centre toward a point leaves the box.
func edgeToward(from box, x, y float32) (float32, float32) {
	centreX, centreY := from.x+from.width/2, from.y+from.height/2
	dx, dy := x-centreX, y-centreY
	if dx == 0 && dy == 0 {
		return centreX, centreY
	}
	scale := min(abs(from.width/2/nonZero(dx)), abs(from.height/2/nonZero(dy)))
	return centreX + dx*scale, centreY + dy*scale
}

func abs(value float32) float32 { return float32(math.Abs(float64(value))) }

// nonZero keeps a divisor from being 0.
func nonZero(value float32) float32 {
	if value == 0 {
		return 1e-6
	}
	return value
}
