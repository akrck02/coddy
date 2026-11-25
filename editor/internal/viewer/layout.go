package viewer

import (
	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/highlight"
	"github.com/akrck02/coddy/editor/internal/workspace"
)

// layOut chooses the presentation of the mode and works out the rows it shows, and the marks
// along the scroll bar, so painting a frame only reads them.
func (view *Viewer) layOut() {
	view.rows, view.diff, view.Hunks, view.Inline, view.Marks = nil, nil, nil, nil, nil
	view.scrollMarks = barMarks{}
	view.shown = view.presentationFor()
	defer func() { view.List.Count = view.RowCount() }()
	if view.Document == nil {
		return
	}
	view.shown.layOut(view)
	view.scrollMarks = view.shown.barMarks(view)
}

// countChanges counts the lines added and gone, and marks the lines of the file that changed.
func (view *Viewer) countChanges() {
	document := view.Document
	if document.Changed {
		view.Added, view.Gone = workspace.Counts(document.Diff)
		view.Marks = workspace.LineMarks(document.Diff)
	}
}

// showsRows tells whether the view shows rows, folds among them, rather than the file's lines.
func (view *Viewer) showsRows() bool { return view.shown.showsRows() }

// lines are the lines shown in the file view: as it is now, or as it was for a deleted file.
func (view *Viewer) lines() []highlight.Line {
	document := view.Document
	if document.Changed && document.Change.Status == git.Deleted {
		return document.OldLines
	}
	return document.NewLines
}

// RowCount is how many rows the file takes as shown.
func (view *Viewer) RowCount() int {
	if view.Document == nil {
		return 0
	}
	if view.showsRows() {
		return len(view.rows)
	}
	return view.fileLineCount()
}

// SetMode shows the file as its changes or whole; only a changed file has changes to show.
func (view *Viewer) SetMode(mode Mode) {
	if view.Document == nil || mode == view.Mode || (mode == DiffView && !view.Document.Changed) ||
		(mode == UsesView && len(view.Uses) == 0) {
		return
	}
	view.Mode = mode
	view.layOut()
	view.List.Offset = 0
	if mode != FileView && len(view.Hunks) > 0 {
		view.List.RevealNearTop(view.Hunks[0])
	}
}

// foldContext is how many unchanged lines the view keeps around a change: -1 for every one.
func (view *Viewer) foldContext() int {
	if view.unfolded[view.Mode] {
		return -1
	}
	return foldContext
}

// Unfolded tells whether the view shows every unchanged line.
func (view *Viewer) Unfolded() bool { return view.unfolded[view.Mode] }

// ToggleContext shows every unchanged line, or folds them again.
func (view *Viewer) ToggleContext() {
	view.unfolded[view.Mode] = !view.unfolded[view.Mode]
	view.layOut()
	view.List.Clamp()
}

// OpenFoldInSight opens the first fold that can be seen.
func (view *Viewer) OpenFoldInSight() {
	first, last := view.List.Visible()
	for index := first; index < last && index < len(view.rows); index++ {
		if view.rows[index].IsFold() {
			view.openFold(index)
			return
		}
	}
}

func (view *Viewer) openFold(index int) {
	view.openedFolds[view.rows[index].FoldStart] = true
	view.layOut()
}
