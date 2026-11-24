package viewer

import (
	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/text"
)

// RevealLine scrolls to a line of the file as it is now and marks it, opening the fold it is in
// when the diff is shown.
func (view *Viewer) RevealLine(number int) {
	view.GoTo = 0
	view.MarkedLine = number
	if view.Edit != nil { // the caret goes there too, at the column asked for
		view.Edit.Place(text.Position{Line: number - 1, Column: view.GoToColumn}, false)
	}
	view.GoToColumn = 0
	if !view.showsRows() {
		view.List.RevealNearTop(max(min(number-1, view.RowCount()-1), 0))
		return
	}
	for attempt := 0; attempt < 2; attempt++ {
		for index, row := range view.rows {
			if !row.IsFold() && row.Line.Kind != git.Removal && row.Line.NewNumber == number {
				view.List.RevealNearTop(index)
				return
			}
		}
		// the line is folded away: open its fold and look again
		for _, row := range view.rows {
			if !row.IsFold() {
				continue
			}
			for offset := 0; offset < row.Folded; offset++ {
				if line := view.diff[row.FoldStart+offset]; line.NewNumber == number {
					view.openedFolds[row.FoldStart] = true
				}
			}
		}
		view.layOut()
	}
}

// StepHunk jumps to the next (or previous) run of changes.
func (view *Viewer) StepHunk(step int) {
	if len(view.Hunks) == 0 {
		return
	}
	marker := view.List.RowOf(view.List.Area.Y + view.List.Area.Height/3)
	target := -1
	if step > 0 {
		for _, start := range view.Hunks {
			if start > marker {
				target = start
				break
			}
		}
	} else {
		for index := len(view.Hunks) - 1; index >= 0; index-- {
			if view.Hunks[index] < marker {
				target = view.Hunks[index]
				break
			}
		}
	}
	if target >= 0 {
		view.List.RevealNearTop(target)
	}
}

// ScrollLines scrolls by step lines; a pane.PageStep is a page, a pane.EndStep the top or the bottom.
func (view *Viewer) ScrollLines(step int) {
	list := &view.List
	page := list.Area.Height - list.RowHeight*2
	switch {
	case step >= pane.EndStep:
		list.Offset = list.ContentHeight()
	case step <= -pane.EndStep:
		list.Offset = 0
	case step >= pane.PageStep:
		list.Offset += page
	case step <= -pane.PageStep:
		list.Offset -= page
	default:
		list.Offset += float32(step) * list.RowHeight
	}
	list.Clamp()
}

// ScrollSideways scrolls by step times four characters.
func (view *Viewer) ScrollSideways(step int) {
	view.List.OffsetX += float32(step) * view.charWidth * sidewaysCharacters
	view.List.Clamp()
}

// sidewaysCharacters are how many characters one press of ← or → scrolls.
const sidewaysCharacters = 4
