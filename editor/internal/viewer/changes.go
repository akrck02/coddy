package viewer

import (
	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/highlight"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/coddy/editor/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
)

// changesPresentation shows a changed file as its diff: both sides interleaved, each line with its
// two numbers, washed in green or red, the unchanged lines far from a change folded away.
type changesPresentation struct{}

// layOut lays the diff out in rows, finds where each run of changes starts, and the words that
// changed inside each replaced line.
func (changesPresentation) layOut(view *Viewer) {
	view.countChanges()
	document := view.Document
	view.rows, view.diff = workspace.DiffRows(document.Diff, view.foldContext(), view.openedFolds), document.Diff
	view.Hunks = workspace.HunkStarts(view.rows)
	view.Inline = workspace.InlineChanges(view.rows, func(line git.DiffLine) []rune {
		if line.Kind == git.Removal {
			return []rune(lineAt(document.OldLines, line.OldNumber).Text())
		}
		return []rune(lineAt(document.NewLines, line.NewNumber).Text())
	})
}

func (changesPresentation) showsRows() bool { return true }

// gutterWidth holds two numbers, the old and the new, and the sign of the change.
func (changesPresentation) gutterWidth(view *Viewer, numberWidth float32) float32 {
	return numberWidth*2 + view.charWidth*2 + changesGutterRoom
}

// changesGutterRoom is the room after the sign of a change, in pixels.
const changesGutterRoom float32 = 6

func (changesPresentation) paintRow(view *Viewer, painter painting, index int, top, baseline float32) {
	view.paintDiffLine(painter, index, top, baseline)
}

// barMarks marks each run of lines added (green) or removed (red).
func (changesPresentation) barMarks(view *Viewer) barMarks {
	marks := barMarks{rows: len(view.rows)}
	for index, row := range view.rows {
		switch {
		case row.IsFold() || row.Line.Kind == git.Context:
		case row.Line.Kind == git.Removal:
			marks.add(index, deletedBarMark)
		default:
			marks.add(index, createdBarMark)
		}
	}
	return marks
}

func (changesPresentation) marksAreGits() bool { return true }

func (changesPresentation) foldedLines() string { return "unchanged lines" }

// lineStyle is how a line of the diff is drawn: its code, its sign and the colours of a change.
type lineStyle struct {
	code       highlight.Line
	sign       string
	signColour draw2d.Color
	lineWash   draw2d.Color
	wordWash   draw2d.Color
	edge       draw2d.Color
}

// styleOf picks a diff line's code from the side it is on, and its colours from how it changed.
func (view *Viewer) styleOf(line git.DiffLine, look theme.Look) lineStyle {
	document := view.Document
	switch line.Kind {
	case git.Insertion:
		return lineStyle{code: lineAt(document.NewLines, line.NewNumber), sign: "+", signColour: look.Created,
			lineWash: look.CreatedLine, wordWash: look.CreatedWord, edge: look.Created}
	case git.Removal:
		return lineStyle{code: lineAt(document.OldLines, line.OldNumber), sign: "−", signColour: look.Deleted,
			lineWash: look.DeletedLine, wordWash: look.DeletedWord, edge: look.Deleted}
	}
	return lineStyle{code: lineAt(document.NewLines, line.NewNumber)} // an unchanged line has no sign
}

// paintDiffLine paints a line of the diff: its wash, its two numbers, its sign and its code.
func (view *Viewer) paintDiffLine(painter painting, index int, top, baseline float32) {
	look, area, drawing := painter.look, painter.area, painter.drawing
	line := view.rows[index].Line
	style := view.styleOf(line, look)
	rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}
	if style.lineWash.IsVisible() {
		view.paintWash(painter, index, style, rowArea)
	}
	numberInk := look.LineNumber
	if line.Kind != git.Removal && line.NewNumber == view.MarkedLine && view.MarkedLine > 0 {
		drawing.Rectangle(rowArea, 0, look.Hover.WithAlpha(0.8), draw2d.Color{}, 0)
		numberInk = look.ActiveLineNumber
	}
	if line.Kind != git.Context {
		numberInk = style.signColour.WithAlpha(0.75)
	}
	view.paintNumber(drawing, line.OldNumber, area.X, painter.numberWidth, baseline, numberInk)
	view.paintNumber(drawing, line.NewNumber, area.X+painter.numberWidth, painter.numberWidth, baseline, numberInk)
	if style.sign != "" {
		drawing.Text(style.sign, area.X+painter.numberWidth*2+view.charWidth/2, baseline, view.codeSize, draw2d.Mono, style.signColour)
	}
	view.paintCode(drawing, painter.look, style.code, painter.codeArea, baseline)
	if line.Kind != git.Removal {
		view.paintComment(painter, line.NewNumber, style.code.Columns, area.X+painter.gutter-20, baseline)
	}
}

// paintWash paints a changed line's background, its edge, and a stronger wash behind the words that
// changed.
func (view *Viewer) paintWash(painter painting, index int, style lineStyle, rowArea draw2d.Rectangle) {
	drawing, top := painter.drawing, rowArea.Y
	drawing.Rectangle(rowArea, 0, style.lineWash, draw2d.Color{}, 0)
	drawing.Rectangle(draw2d.Rectangle{X: rowArea.X, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, style.edge, draw2d.Color{}, 0)
	span, found := view.Inline[index]
	if !found || span.End <= span.Start {
		return
	}
	x := painter.codeArea.X + float32(span.Start)*view.charWidth - view.List.OffsetX
	word := draw2d.Rectangle{X: x, Y: top + 2, Width: float32(span.End-span.Start) * view.charWidth, Height: view.rowHeight - 4}
	drawing.PushClip(painter.codeArea)
	drawing.Rectangle(word, 3, style.wordWash, draw2d.Color{}, 0)
	drawing.PopClip()
}
