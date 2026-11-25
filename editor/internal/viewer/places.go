package viewer

import (
	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/highlight"
	"github.com/akrck02/coddy/editor/internal/text"
)

// Place is a place in the file open, as it is now: a line and a character of it, from 0.
type Place struct{ Line, Column int }

// PlaceAt is the place of the file under a point of the code: in the file view any line, in the
// diff any line the file still has (a removed line is not on disk, where servers read).
func (view *Viewer) PlaceAt(x, y float32) (Place, bool) {
	if !view.showing || view.Document == nil || x < view.codeLeft {
		return Place{}, false
	}
	line := view.fileLineOfRow(view.List.RowAt(x, y))
	if line < 0 || line >= view.fileLineCount() {
		return Place{}, false
	}
	column := (x - view.codeLeft + view.List.OffsetX) / max(view.charWidth, 1)
	return Place{Line: line, Column: highlight.CharacterAt(view.fileLine(line).Text(), column)}, true
}

// fileLineOfRow is the line of the file a row shows, from 0; -1 when it shows none.
func (view *Viewer) fileLineOfRow(index int) int {
	if index < 0 {
		return -1
	}
	if !view.showsRows() {
		return index
	}
	if index >= len(view.rows) || view.rows[index].IsFold() || view.rows[index].Line.Kind == git.Removal {
		return -1
	}
	return view.rows[index].Line.NewNumber - 1
}

// Caret is the place of the caret, while the file is being edited.
func (view *Viewer) Caret() (Place, bool) {
	if !view.Editable() {
		return Place{}, false
	}
	caret := view.Edit.Caret()
	return Place{Line: caret.Line, Column: caret.Column}, true
}

// PlaceAsked is the place a command about a name asks about: the caret while editing, otherwise
// the place clicked last.
func (view *Viewer) PlaceAsked() (Place, bool) {
	if place, editing := view.Caret(); editing {
		return place, true
	}
	return view.clicked, view.clickedSome && view.showing
}

// LineText is the text of a line of the file as it is now, edits included.
func (view *Viewer) LineText(index int) string {
	if index < 0 || index >= view.fileLineCount() {
		return ""
	}
	return view.fileLine(index).Text()
}

// LineInSight is the line, from 1, to come back to: the caret's while editing, otherwise the
// marked line, or the first in sight.
func (view *Viewer) LineInSight() int {
	if place, editing := view.Caret(); editing {
		return place.Line + 1
	}
	if view.MarkedLine > 0 {
		return view.MarkedLine
	}
	if view.List.RowHeight <= 0 {
		return 1
	}
	return view.fileLineOfRow(max(view.List.RowOf(view.List.Area.Y), 0)) + 1
}

// PlaceCaret puts the caret at a place of the file being edited, and keeps it in sight.
func (view *Viewer) PlaceCaret(place Place) {
	if view.Editable() {
		view.Edit.Place(text.Position(place), false)
		view.revealCaret()
	}
}

// PointOf is the middle of a place of the file view, where it was drawn last: what a test points
// at, and where a card about a name goes.
func (view *Viewer) PointOf(place Place) (x, y float32) {
	x = view.codeLeft + (float32(place.Column)+0.5)*view.charWidth - view.List.OffsetX
	y = view.List.RowTop(place.Line) + view.List.RowHeight/2
	return x, y
}
