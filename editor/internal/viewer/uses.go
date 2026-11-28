package viewer

import (
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/highlight"
	"github.com/akrck02/go-renderer/draw2d"
)

// usesPresentation shows the file as it is now with the uses of a name washed and the lines away
// from them folded, as the changes fold the lines away from a change.
type usesPresentation struct{}

// layOut folds the lines away from the uses and finds where each run of uses starts.
func (usesPresentation) layOut(view *Viewer) {
	focus := map[int]bool{}
	for number := range view.Uses {
		focus[number] = true
	}
	view.rows, view.diff = codebase.FocusRows(len(view.lines()), focus, view.foldContext(), view.openedFolds)
	view.Hunks = codebase.FocusStarts(view.rows, focus)
}

func (usesPresentation) showsRows() bool { return true }

func (usesPresentation) gutterWidth(view *Viewer, numberWidth float32) float32 {
	return numberWidth + wholeFileGutterRoom
}

func (usesPresentation) paintRow(view *Viewer, painter painting, index int, top, baseline float32) {
	view.paintUseLine(painter, index, top, baseline)
}

// barMarks marks each run of lines that use the name.
func (usesPresentation) barMarks(view *Viewer) barMarks {
	marks := barMarks{rows: len(view.rows)}
	for index, row := range view.rows {
		if !row.IsFold() && len(view.Uses[row.Line.NewNumber]) > 0 {
			marks.add(index, useBarMark)
		}
	}
	return marks
}

// marksAreGits is false: the uses' marks are not git's, so they show whatever git's status shows.
func (usesPresentation) marksAreGits() bool { return false }

func (usesPresentation) foldedLines() string { return "lines without uses" }

// paintUseLine paints a line of the uses view: its number and code, washed when it uses the name,
// more strongly behind each use.
func (view *Viewer) paintUseLine(painter painting, index int, top, baseline float32) {
	look, area, drawing := painter.look, painter.area, painter.drawing
	number := view.rows[index].Line.NewNumber
	rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}
	numberInk := look.LineNumber
	if spans := view.Uses[number]; len(spans) > 0 {
		drawing.Rectangle(rowArea, 0, look.Theme.PrimaryTint, draw2d.Color{}, 0)
		drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, look.Theme.Primary, draw2d.Color{}, 0)
		numberInk = look.ActiveLineNumber
		line := lineAt(view.lines(), number).Text()
		drawing.PushClip(painter.codeArea)
		for _, span := range spans {
			start, end := highlight.Column(line, span.Start), highlight.Column(line, span.End)
			x := painter.codeArea.X + float32(start)*view.charWidth - view.List.OffsetX
			word := draw2d.Rectangle{X: x, Y: top + 2, Width: float32(end-start) * view.charWidth, Height: view.rowHeight - 4}
			drawing.Rectangle(word, 3, look.Theme.Selection, draw2d.Color{}, 0)
		}
		drawing.PopClip()
	}
	if number == view.MarkedLine && view.MarkedLine > 0 {
		drawing.Rectangle(rowArea, 0, look.Hover.WithAlpha(0.8), draw2d.Color{}, 0)
		numberInk = look.ActiveLineNumber
	}
	view.paintNumber(drawing, number, area.X, painter.numberWidth, baseline, numberInk)
	view.paintCode(drawing, painter.look, lineAt(view.lines(), number), painter.codeArea, baseline)
}
