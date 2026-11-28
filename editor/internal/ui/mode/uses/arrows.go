package uses

import (
	"image"
	"math"

	"github.com/akrck02/coddy/editor/internal/ui/theme"
	"github.com/akrck02/go-renderer/draw2d"
)

// An arrow's lines are drawn as pictures of short pieces of them, each drawn once for a level of
// zoom and kept. The renderer's Stroke drew a whole line into a new picture on every frame, as
// large as the box around the line at the zoom, and sent it to the graphics card: zoomed in, tens
// of megabytes a frame. A piece is small, a level holds for a range of zooms, and a piece out of
// sight is not drawn at all.

const (
	pieceSide   = 96   // how long a piece is, in pixels of its picture
	arrowWidth  = 1    // how wide a line is, in the canvas's units
	thinnest    = 1.2  // how wide a line is at least, in pixels of its picture
	keptPieces  = 2000 // how many pictures of pieces are kept; past that they are drawn again
	arrowShade  = 0.55 // how much of the quiet ink an arrow takes over the editor's colour
	headLength  = 9
	headOpening = 0.45
)

// pieceKey tells a piece apart: the line it is of, the level of zoom and its place in the line.
type pieceKey struct {
	from, to draw2d.Point
	level    int
	piece    int
}

// zoomLevel is the level a line is drawn at for a zoom, a power of √2, and the scale it means: the
// nearest at or above the zoom, so a picture is shrunk, not blown up.
func zoomLevel(zoom float32) (int, float32) {
	level := int(math.Ceil(2 * math.Log2(float64(max(zoom, 0.01)))))
	return level, float32(math.Pow(2, float64(level)/2))
}

// paintArrow draws an arrow from a box's edge to another's, with its head at the second; opaque,
// so the pieces of a line, which overlap, show as one.
func (canvas *canvas) paintArrow(drawing *draw2d.List, look theme.Look, from, to box) {
	startX, startY := edgeToward(from, to.x+to.width/2, to.y+to.height/2)
	endX, endY := edgeToward(to, from.x+from.width/2, from.y+from.height/2)
	colour := look.Editor.Mix(look.Theme.InkQuiet, arrowShade)
	end := draw2d.Point{X: endX, Y: endY}
	canvas.paintLine(drawing, draw2d.Point{X: startX, Y: startY}, end, colour)
	angle := math.Atan2(float64(endY-startY), float64(endX-startX))
	for _, side := range []float64{-headOpening, headOpening} {
		head := draw2d.Point{X: endX - headLength*float32(math.Cos(angle+side)), Y: endY - headLength*float32(math.Sin(angle+side))}
		canvas.paintLine(drawing, end, head, colour)
	}
}

// paintLine draws a straight line of the canvas, piece by piece, leaving out the pieces out of
// sight.
func (canvas *canvas) paintLine(drawing *draw2d.List, from, to draw2d.Point, colour draw2d.Color) {
	level, scale := zoomLevel(canvas.zoom)
	length := float32(math.Hypot(float64(to.X-from.X), float64(to.Y-from.Y)))
	count := max(int(math.Ceil(float64(length*scale/pieceSide))), 1)
	for piece := range count {
		start, end := along(from, to, float32(piece)/float32(count)), along(from, to, float32(piece+1)/float32(count))
		picture, corner := canvas.piece(pieceKey{from: from, to: to, level: level, piece: piece}, start, end, scale)
		width, height := picture.Size()
		left, top := canvas.toScreen(corner.X, corner.Y)
		area := draw2d.Rectangle{X: left, Y: top, Width: float32(width) / scale * canvas.zoom, Height: float32(height) / scale * canvas.zoom}
		if overlaps(area, canvas.area) {
			drawing.Image(picture, area, 0, colour)
		}
	}
}

// along is the point of a line a share of the way from its start.
func along(from, to draw2d.Point, share float32) draw2d.Point {
	return draw2d.Point{X: from.X + (to.X-from.X)*share, Y: from.Y + (to.Y-from.Y)*share}
}

// overlaps tells whether two rectangles share some room.
func overlaps(one, other draw2d.Rectangle) bool {
	return one.X < other.X+other.Width && other.X < one.X+one.Width && one.Y < other.Y+other.Height && other.Y < one.Y+one.Height
}

// piece is the picture of a piece of a line, drawn the first time it is wanted, and the canvas
// point its picture's corner is at.
func (canvas *canvas) piece(key pieceKey, start, end draw2d.Point, scale float32) (*draw2d.Image, draw2d.Point) {
	half := max(arrowWidth*scale, thinnest) / 2
	margin := (half + 1) / scale
	corner := draw2d.Point{X: min(start.X, end.X) - margin, Y: min(start.Y, end.Y) - margin}
	if picture, kept := canvas.pieces[key]; kept {
		return picture, corner
	}
	if canvas.pieces == nil || len(canvas.pieces) >= keptPieces {
		canvas.pieces = map[pieceKey]*draw2d.Image{}
	}
	width := int(math.Ceil(float64((abs(end.X-start.X) + 2*margin) * scale)))
	height := int(math.Ceil(float64((abs(end.Y-start.Y) + 2*margin) * scale)))
	picture := draw2d.NewBlankImage(max(width, 1), max(height, 1))
	place := func(point draw2d.Point) draw2d.Point {
		return draw2d.Point{X: (point.X - corner.X) * scale, Y: (point.Y - corner.Y) * scale}
	}
	coverLine(picture.Pixels, place(start), place(end), half)
	canvas.pieces[key] = picture
	return picture, corner
}

// coverLine covers the pixels of a mask within half a line's width of a straight piece, its edge
// smoothed over a pixel, its ends round.
func coverLine(mask *image.NRGBA, from, to draw2d.Point, half float32) {
	bounds := mask.Rect
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			centre := draw2d.Point{X: float32(column) + 0.5, Y: float32(row) + 0.5}
			coverage := min(max(half+0.5-distanceTo(centre, from, to), 0), 1)
			if alpha := uint8(coverage * 255); alpha > 0 {
				offset := mask.PixOffset(column, row)
				mask.Pix[offset], mask.Pix[offset+1], mask.Pix[offset+2], mask.Pix[offset+3] = 255, 255, 255, alpha
			}
		}
	}
}

// distanceTo is how far a point is from a straight piece between two others.
func distanceTo(point, from, to draw2d.Point) float32 {
	alongX, alongY := to.X-from.X, to.Y-from.Y
	length := alongX*alongX + alongY*alongY
	share := float32(0)
	if length > 0 {
		share = min(max(((point.X-from.X)*alongX+(point.Y-from.Y)*alongY)/length, 0), 1)
	}
	return float32(math.Hypot(float64(from.X+share*alongX-point.X), float64(from.Y+share*alongY-point.Y)))
}
