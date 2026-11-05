package app

import (
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// scroller is a list of rows of one height painted by the program, which keeps its own scroll so
// it can move it from the keyboard (go-ui's lists only scroll by hand). Input is read against
// where the list was painted in the last frame, as go-ui's own controls do.
type scroller struct {
	offset    float32 // how far down the content is scrolled
	offsetX   float32 // how far right, for long lines
	rowHeight float32
	count     int
	width     float32 // the content's width, for scrolling sideways; 0 = no sideways scroll

	area draw2d.Rectangle // where it was painted last

	// a row to reveal once the list has been laid out (asked for before its size was known), as
	// its index plus one, and whether near the top
	pendingReveal  int
	pendingNearTop bool

	draggingThumb bool
	dragStart     float32 // the pointer's y when the thumb was taken, less the thumb's top
}

const scrollBarWidth float32 = 8

func (list *scroller) contentHeight() float32 { return float32(list.count) * list.rowHeight }

// clamp keeps the offsets inside the content.
func (list *scroller) clamp() {
	list.offset = min(list.offset, list.contentHeight()-list.area.Height)
	list.offset = max(list.offset, 0)
	list.offsetX = min(list.offsetX, list.width-list.area.Width+40)
	list.offsetX = max(list.offsetX, 0)
}

// visible is the range of rows that can be seen.
func (list *scroller) visible() (first, last int) {
	if list.rowHeight <= 0 {
		return 0, 0
	}
	first = max(int(list.offset/list.rowHeight), 0)
	last = min(int((list.offset+list.area.Height)/list.rowHeight)+1, list.count)
	return first, max(first, last)
}

// rowTop is where a row is painted.
func (list *scroller) rowTop(index int) float32 {
	return list.area.Y + float32(index)*list.rowHeight - list.offset
}

// rowAt is the row under a point, or -1.
func (list *scroller) rowAt(x, y float32) int {
	if !list.area.Contains(x, y) || x >= list.area.Right()-scrollBarWidth {
		return -1
	}
	index := int((y - list.area.Y + list.offset) / list.rowHeight)
	if index < 0 || index >= list.count {
		return -1
	}
	return index
}

// pageRows is how many rows fill the view.
func (list *scroller) pageRows() int {
	return max(int(list.area.Height/list.rowHeight)-1, 1)
}

// place records where the list was laid out, and does a reveal asked for before that was known.
func (list *scroller) place(area draw2d.Rectangle) {
	list.area = area
	if list.pendingReveal > 0 && list.rowHeight > 0 && area.Height > 0 {
		index := list.pendingReveal - 1
		list.pendingReveal = 0
		if list.pendingNearTop {
			list.revealNearTop(index)
		} else {
			list.reveal(index)
		}
	}
	list.clamp()
}

// unplaced tells whether the list's size is not known yet, and keeps a reveal for when it is.
func (list *scroller) unplaced(index int, nearTop bool) bool {
	if list.area.Height > 0 && list.rowHeight > 0 && list.count > index {
		return false
	}
	list.pendingReveal, list.pendingNearTop = index+1, nearTop
	return true
}

// reveal scrolls as little as possible to show a row.
func (list *scroller) reveal(index int) {
	if list.unplaced(index, false) {
		return
	}
	top := float32(index) * list.rowHeight
	if top < list.offset {
		list.offset = top
	}
	if bottom := top + list.rowHeight; bottom > list.offset+list.area.Height {
		list.offset = bottom - list.area.Height
	}
	list.clamp()
}

// revealNearTop scrolls a row to a third of the way down the view, with what comes before it in
// sight: where a jump to a change lands.
func (list *scroller) revealNearTop(index int) {
	if list.unplaced(index, true) {
		return
	}
	list.offset = float32(index)*list.rowHeight - list.area.Height/3
	list.clamp()
}

// follow moves the list with the wheel when the pointer is over it, and with its thumb when that is
// dragged. It tells whether the pointer is on the scroll bar, so a click there selects nothing.
func (list *scroller) follow(context *ui.Context, keys keys, step float32) (onBar bool) {
	x, y := context.Pointer()
	if list.area.Contains(x, y) {
		horizontal, vertical := keys.wheel()
		list.offset -= vertical * step
		list.offsetX -= horizontal * step
	}
	bar := list.barArea()
	onBar = bar.Contains(x, y)
	thumb, hasThumb := list.thumbArea()
	if keys.clicked() && hasThumb && onBar {
		list.draggingThumb = true
		list.dragStart = y - thumb.Y
		if !thumb.Contains(x, y) {
			list.dragStart = thumb.Height / 2 // a click beside the thumb brings it under the pointer
		}
	}
	if !keys.holding() {
		list.draggingThumb = false
	}
	if list.draggingThumb && hasThumb {
		list.moveThumbTo(y-list.dragStart, thumb.Height)
		onBar = true
	}
	list.clamp()
	return onBar
}

// moveThumbTo scrolls so the thumb's top is at a height.
func (list *scroller) moveThumbTo(top, thumbHeight float32) {
	bar := list.barArea()
	span := bar.Height - thumbHeight
	if span <= 0 {
		return
	}
	list.offset = (top - bar.Y) / span * (list.contentHeight() - list.area.Height)
}

func (list *scroller) barArea() draw2d.Rectangle {
	return draw2d.Rectangle{X: list.area.Right() - scrollBarWidth, Y: list.area.Y, Width: scrollBarWidth, Height: list.area.Height}
}

func (list *scroller) thumbArea() (draw2d.Rectangle, bool) {
	content := list.contentHeight()
	if content <= list.area.Height || list.area.Height <= 0 {
		return draw2d.Rectangle{}, false
	}
	bar := list.barArea()
	height := max(bar.Height*list.area.Height/content, 24)
	top := bar.Y + (bar.Height-height)*list.offset/(content-list.area.Height)
	return draw2d.Rectangle{X: bar.X + 2, Y: top, Width: bar.Width - 4, Height: height}, true
}

// paintBar paints the thumb, when the content is taller than the view.
func (list *scroller) paintBar(drawing *draw2d.List, colour draw2d.Color) {
	if thumb, shown := list.thumbArea(); shown {
		drawing.Rectangle(thumb, thumb.Width/2, colour, draw2d.Color{}, 0)
	}
}
