package pane

import (
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// Scroller is a list of rows of one height painted by the program, which keeps its own scroll so
// it can move it from the keyboard (go-ui's lists only scroll by hand). Input is read against
// where the list was painted in the last frame, as go-ui's own controls do.
type Scroller struct {
	Offset    float32 // how far down the content is scrolled
	OffsetX   float32 // how far right, for long lines
	RowHeight float32
	Count     int
	Width     float32 // the content's width, for scrolling sideways; 0 = no sideways scroll

	Area draw2d.Rectangle // where it was painted last

	// a row to reveal once the list has been laid out (asked for before its size was known), as
	// its index plus one, and whether near the top
	pendingReveal  int
	pendingNearTop bool

	draggingThumb bool
	dragStart     float32 // the pointer's y when the thumb was taken, less the thumb's top
}

const ScrollBarWidth float32 = 8

func (list *Scroller) ContentHeight() float32 { return float32(list.Count) * list.RowHeight }

// Clamp keeps the offsets inside the content.
func (list *Scroller) Clamp() {
	list.Offset = min(list.Offset, list.ContentHeight()-list.Area.Height)
	list.Offset = max(list.Offset, 0)
	list.OffsetX = min(list.OffsetX, list.Width-list.Area.Width+40)
	list.OffsetX = max(list.OffsetX, 0)
}

// Visible is the range of rows that can be seen.
func (list *Scroller) Visible() (first, last int) {
	if list.RowHeight <= 0 {
		return 0, 0
	}
	first = max(int(list.Offset/list.RowHeight), 0)
	last = min(int((list.Offset+list.Area.Height)/list.RowHeight)+1, list.Count)
	return first, max(first, last)
}

// RowTop is where a row is painted.
func (list *Scroller) RowTop(index int) float32 {
	return list.Area.Y + float32(index)*list.RowHeight - list.Offset
}

// RowAt is the row under a point, or -1.
func (list *Scroller) RowAt(x, y float32) int {
	if !list.Area.Contains(x, y) || x >= list.Area.Right()-ScrollBarWidth {
		return -1
	}
	index := int((y - list.Area.Y + list.Offset) / list.RowHeight)
	if index < 0 || index >= list.Count {
		return -1
	}
	return index
}

// PageRows is how many rows fill the view.
func (list *Scroller) PageRows() int {
	return max(int(list.Area.Height/list.RowHeight)-1, 1)
}

// Place records where the list was laid out, and does a reveal asked for before that was known.
func (list *Scroller) Place(area draw2d.Rectangle) {
	list.Area = area
	if list.pendingReveal > 0 && list.RowHeight > 0 && area.Height > 0 {
		index := list.pendingReveal - 1
		list.pendingReveal = 0
		if list.pendingNearTop {
			list.RevealNearTop(index)
		} else {
			list.Reveal(index)
		}
	}
	list.Clamp()
}

// unplaced tells whether the list's size is not known yet, and keeps a reveal for when it is.
func (list *Scroller) unplaced(index int, nearTop bool) bool {
	if list.Area.Height > 0 && list.RowHeight > 0 && list.Count > index {
		return false
	}
	list.pendingReveal, list.pendingNearTop = index+1, nearTop
	return true
}

// Reveal scrolls as little as possible to show a row.
func (list *Scroller) Reveal(index int) {
	if list.unplaced(index, false) {
		return
	}
	top := float32(index) * list.RowHeight
	if top < list.Offset {
		list.Offset = top
	}
	if bottom := top + list.RowHeight; bottom > list.Offset+list.Area.Height {
		list.Offset = bottom - list.Area.Height
	}
	list.Clamp()
}

// RevealNearTop scrolls a row to a third of the way down the view, with what comes before it in
// sight: where a jump to a change lands.
func (list *Scroller) RevealNearTop(index int) {
	if list.unplaced(index, true) {
		return
	}
	list.Offset = float32(index)*list.RowHeight - list.Area.Height/3
	list.Clamp()
}

// Follow moves the list with the wheel when the pointer is over it, and with its thumb when that is
// dragged. It tells whether the pointer is on the scroll bar, so a click there selects nothing.
func (list *Scroller) Follow(context *ui.Context, keys Keys, step float32) (onBar bool) {
	x, y := context.Pointer()
	// the wheel with Command zooms, where a pane zooms, and scrolls nothing
	if list.Area.Contains(x, y) && !keys.Command() {
		horizontal, vertical := keys.Wheel()
		list.Offset -= vertical * step
		list.OffsetX -= horizontal * step
	}
	bar := list.BarArea()
	onBar = bar.Contains(x, y)
	thumb, hasThumb := list.thumbArea()
	if keys.Clicked() && hasThumb && onBar {
		list.draggingThumb = true
		list.dragStart = y - thumb.Y
		if !thumb.Contains(x, y) {
			list.dragStart = thumb.Height / 2 // a click beside the thumb brings it under the pointer
		}
	}
	if !keys.Holding() {
		list.draggingThumb = false
	}
	if list.draggingThumb && hasThumb {
		list.moveThumbTo(y-list.dragStart, thumb.Height)
		onBar = true
	}
	list.Clamp()
	return onBar
}

// moveThumbTo scrolls so the thumb's top is at a height.
func (list *Scroller) moveThumbTo(top, thumbHeight float32) {
	bar := list.BarArea()
	span := bar.Height - thumbHeight
	if span <= 0 {
		return
	}
	list.Offset = (top - bar.Y) / span * (list.ContentHeight() - list.Area.Height)
}

// BarArea is the strip the scroll bar runs in, along the right edge.
func (list *Scroller) BarArea() draw2d.Rectangle {
	return draw2d.Rectangle{X: list.Area.Right() - ScrollBarWidth, Y: list.Area.Y, Width: ScrollBarWidth, Height: list.Area.Height}
}

func (list *Scroller) thumbArea() (draw2d.Rectangle, bool) {
	content := list.ContentHeight()
	if content <= list.Area.Height || list.Area.Height <= 0 {
		return draw2d.Rectangle{}, false
	}
	bar := list.BarArea()
	height := max(bar.Height*list.Area.Height/content, 24)
	top := bar.Y + (bar.Height-height)*list.Offset/(content-list.Area.Height)
	return draw2d.Rectangle{X: bar.X + 2, Y: top, Width: bar.Width - 4, Height: height}, true
}

// PaintBar paints the thumb, when the content is taller than the view.
func (list *Scroller) PaintBar(drawing *draw2d.List, colour draw2d.Color) {
	if thumb, shown := list.thumbArea(); shown {
		drawing.Rectangle(thumb, thumb.Width/2, colour, draw2d.Color{}, 0)
	}
}
