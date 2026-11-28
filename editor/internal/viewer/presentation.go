package viewer

import (
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
)

// presentation is one way the view shows a file: whole, as its changes, or as the uses of a name.
// The view picks one when its mode or its file changes (presentationFor), never while painting,
// so a frame only asks the one chosen. A new way of showing a file is one more of these.
type presentation interface {
	// layOut works out the rows to show and the runs ] and [ step to, once the file's counts and
	// marks are known.
	layOut(view *Viewer)
	// showsRows tells whether it shows rows of a diff, folds among them, rather than the file's lines.
	showsRows() bool
	// gutterWidth is how wide the gutter is beside the code, for numbers numberWidth wide.
	gutterWidth(view *Viewer, numberWidth float32) float32
	// paintRow paints a row that is not a fold: its number, its marks and its code.
	paintRow(view *Viewer, painter painting, index int, top, baseline float32)
	// barMarks are the marks along the scroll bar, a map of where the changes or the uses are.
	barMarks(view *Viewer) barMarks
	// marksAreGits tells whether those marks are git's, shown only while git's status is.
	marksAreGits() bool
	// foldedLines names the lines a fold holds, as its words say: "unchanged lines".
	foldedLines() string
}

// The view's presentations; each is a value without fields, so choosing one costs nothing.
var (
	wholeFile presentation = wholeFilePresentation{}
	changes   presentation = changesPresentation{}
	uses      presentation = usesPresentation{}
)

// presentationFor is the presentation of the view's mode: the changes only for a file that changed.
func (view *Viewer) presentationFor() presentation {
	switch {
	case view.Mode == UsesView:
		return uses
	case view.Mode == DiffView && view.Document != nil && view.Document.Changed:
		return changes
	}
	return wholeFile
}

// wholeFilePresentation shows the file as it is now, marking in the gutter the lines that changed.
type wholeFilePresentation struct{}

// layOut steps ] and [ from one run of marked lines to the next.
func (wholeFilePresentation) layOut(view *Viewer) {
	view.countChanges()
	for number := 1; number <= len(view.lines()); number++ {
		if view.Marks[number] != codebase.NoMark && view.Marks[number-1] == codebase.NoMark {
			view.Hunks = append(view.Hunks, number-1)
		}
	}
}

func (wholeFilePresentation) showsRows() bool { return false }

func (wholeFilePresentation) gutterWidth(view *Viewer, numberWidth float32) float32 {
	return numberWidth + wholeFileGutterRoom
}

// wholeFileGutterRoom is the room after the line numbers, in pixels, for the marks of a change.
const wholeFileGutterRoom float32 = 14

func (wholeFilePresentation) paintRow(view *Viewer, painter painting, index int, top, baseline float32) {
	view.paintFileLine(painter, index, top, baseline)
}

// barMarks marks each run of lines added, changed or with lines removed beside them.
func (wholeFilePresentation) barMarks(view *Viewer) barMarks {
	count := len(view.lines())
	marks := barMarks{rows: count}
	for number := 1; number <= count; number++ {
		switch view.Marks[number] {
		case codebase.NoMark:
		case codebase.AddedMark:
			marks.add(number-1, createdBarMark)
		case codebase.ModifiedMark:
			marks.add(number-1, modifiedBarMark)
		default:
			marks.add(number-1, deletedBarMark)
		}
	}
	return marks
}

func (wholeFilePresentation) marksAreGits() bool { return true }

func (wholeFilePresentation) foldedLines() string { return "unchanged lines" }

// barMarkKind is what a mark along the scroll bar stands for, which gives its colour.
type barMarkKind int

const (
	createdBarMark barMarkKind = iota
	modifiedBarMark
	deletedBarMark
	useBarMark
)

// barMark is a run of rows marked the same along the scroll bar.
type barMark struct {
	first, count int
	kind         barMarkKind
}

// barMarks are the runs of marked rows along the scroll bar, out of rows rows. They are worked out
// when the view is laid out, so a frame paints one mark per run, however long the file.
type barMarks struct {
	rows int
	runs []barMark
}

// add marks a row, joining it to the run before it when they touch and are of the same kind.
func (marks *barMarks) add(row int, kind barMarkKind) {
	if last := len(marks.runs) - 1; last >= 0 {
		run := &marks.runs[last]
		if run.kind == kind && run.first+run.count == row {
			run.count++
			return
		}
	}
	marks.runs = append(marks.runs, barMark{first: row, count: 1, kind: kind})
}

// colourOf is the colour of a mark of a kind.
func colourOf(kind barMarkKind, look theme.Look) draw2d.Color {
	switch kind {
	case createdBarMark:
		return look.Created
	case modifiedBarMark:
		return look.Modified
	case deletedBarMark:
		return look.Deleted
	}
	return look.Theme.Primary
}
