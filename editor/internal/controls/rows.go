package controls

import (
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
)

// RowState is how a row of a list stands, which its background shows.
type RowState int

const (
	// RowPlain is a row with nothing to show: no background.
	RowPlain RowState = iota
	// RowHovered is a row under the pointer.
	RowHovered
	// RowChosenQuietly is the chosen row of a list without the keyboard, or one open elsewhere.
	RowChosenQuietly
	// RowChosen is the chosen row of the list that has the keyboard.
	RowChosen
)

// PaintRowBackground paints the background of a list's row, rounded by radius pixels, as its
// state asks. The sidebar and the finder share it, so a chosen row looks the same.
func PaintRowBackground(drawing *draw2d.List, look theme.Look, area draw2d.Rectangle, radius float32, state RowState) {
	switch state {
	case RowChosen:
		drawing.Rectangle(area, radius, look.Chosen, look.FocusEdge, 1)
	case RowChosenQuietly:
		drawing.Rectangle(area, radius, look.Hover.Mix(look.Chosen, 0.5), draw2d.Color{}, 0)
	case RowHovered:
		drawing.Rectangle(area, radius, look.Hover, draw2d.Color{}, 0)
	}
}
