package viewer

import (
	"fmt"
	"strconv"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/highlight"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// measure measures a character of the code, the width every column is, when its size changed.
func (view *Viewer) measure(context *ui.Context) {
	if view.measuredSize == view.codeSize && view.charWidth > 0 {
		return
	}
	view.measuredSize = view.codeSize
	metrics := context.Measurer().MeasureText("M", view.codeSize, draw2d.Mono, 1)
	view.charWidth = metrics.Width
	view.codeAscent, view.codeDescent = metrics.Ascent, metrics.Descent
}

// gutterMarkWidth is the bar beside a line's number that says it was added or changed.
const gutterMarkWidth float32 = 3

// paint paints the visible rows as the presentation shown paints them, the folds among them, and
// the marks along the scroll bar.
func (view *Viewer) paint(context *ui.Context, window Window, drawing *draw2d.List) {
	document := view.Document
	if document == nil {
		return
	}
	if document.Binary || (document.Error != nil && view.RowCount() == 0) {
		message := "A binary file: nothing to show"
		if !document.Binary {
			message = view.problem.Of(document, func(document *codebase.Document) string { return document.Error.Error() })
		}
		drawing.Text(message, view.List.Area.X+24, view.List.Area.Y+32, context.Theme.TextSize, draw2d.Regular, context.Theme.InkQuiet)
		return
	}
	painter := view.startPainting(context, window, drawing)
	view.List.Width = 0
	drawing.PushClip(painter.area)
	first, last := view.List.Visible()
	for index := first; index < last; index++ {
		top := view.List.RowTop(index)
		baseline := top + (view.rowHeight+view.codeAscent-view.codeDescent)/2
		if view.showsRows() && view.rows[index].IsFold() {
			view.paintFold(painter, view.rows[index], top, baseline)
			continue
		}
		view.shown.paintRow(view, painter, index, top, baseline)
	}
	view.List.PaintBar(drawing, painter.look.ScrollThumb)
	if !view.shown.marksAreGits() || view.showsStatus(painter.window) {
		view.paintScrollMarks(drawing, painter.look)
	}
	drawing.PopClip()
}

// painting is what every line painted in one frame shares: where the gutter and the code are.
type painting struct {
	context     *ui.Context
	window      Window
	drawing     *draw2d.List
	look        theme.Look
	area        draw2d.Rectangle
	codeArea    draw2d.Rectangle
	numberWidth float32
	gutter      float32
}

// startPainting measures the gutter, wide enough for the longest number, and the code beside it.
func (view *Viewer) startPainting(context *ui.Context, window Window, drawing *draw2d.List) painting {
	document := view.Document
	area := view.List.Area
	numberColumns := len(strconv.Itoa(max(len(document.NewLines), len(document.OldLines), 1)))
	numberWidth := float32(numberColumns)*view.charWidth + 16
	gutter := view.shown.gutterWidth(view, numberWidth)
	codeLeft := area.X + gutter + 10
	view.codeLeft, view.codeWidth = codeLeft, max(area.Right()-codeLeft-pane.ScrollBarWidth, 0)
	return painting{
		context: context, window: window, drawing: drawing, look: window.Look(), area: area,
		codeArea:    draw2d.Rectangle{X: codeLeft, Y: area.Y, Width: max(area.Right()-codeLeft-pane.ScrollBarWidth, 0), Height: area.Height},
		numberWidth: numberWidth, gutter: gutter,
	}
}

// paintFileLine paints a line of the whole file: its number, its mark and its code.
func (view *Viewer) paintFileLine(painter painting, index int, top, baseline float32) {
	look, area, drawing := painter.look, painter.area, painter.drawing
	number := index + 1
	editing := view.Edit != nil && painter.window.Editing()
	numberInk := look.LineNumber
	if number == view.MarkedLine || (editing && index == view.Edit.Caret().Line) {
		drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}, 0, look.Hover, draw2d.Color{}, 0)
		numberInk = look.ActiveLineNumber
	}
	view.paintNumber(drawing, number, area.X, painter.numberWidth, baseline, numberInk)
	if view.showsStatus(painter.window) {
		view.paintMark(drawing, look, view.Marks[number], area.X+painter.numberWidth+4, top)
	}
	if view.Edit != nil {
		view.paintSelection(painter, index, top)
	}
	line := view.fileLine(index)
	view.paintCode(drawing, painter.look, line, painter.codeArea, baseline)
	view.paintComment(painter, number, line.Columns, area.X+painter.gutter-20, baseline)
	if editing {
		view.paintCaret(painter, index, top)
	}
}

// paintFold paints the row that stands for lines folded away.
func (view *Viewer) paintFold(painter painting, row codebase.Row, top, baseline float32) {
	palette, area := painter.context.Theme, painter.area
	rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}
	painter.drawing.Rectangle(rowArea, 0, painter.look.Hover.WithAlpha(0.6), draw2d.Color{}, 0)
	painter.drawing.Icon("unfold-vertical", area.X+painter.gutter-20, top+(view.rowHeight-12)/2, 12, palette.InkQuiet)
	label := view.foldWordsOf(foldLabel{row.Folded, view.shown.foldedLines()})
	painter.drawing.Text(label, painter.codeArea.X, baseline, palette.SmallTextSize, draw2d.Regular, palette.InkQuiet)
}

// foldLabel is what a fold's words say: how many lines it holds, and what lines they are.
type foldLabel struct {
	lines int
	what  string
}

// foldWordsOf are a fold's words, made the first time a fold of that size shows.
func (view *Viewer) foldWordsOf(label foldLabel) string {
	if words, made := view.foldWords[label]; made {
		return words
	}
	if view.foldWords == nil {
		view.foldWords = map[foldLabel]string{}
	}
	words := fmt.Sprintf("%d %s", label.lines, label.what)
	view.foldWords[label] = words
	return words
}

// paintMark paints a line's mark in the file view's gutter: a bar for a line added (green) or
// changed (amber), a small wedge where lines were removed (red).
func (view *Viewer) paintMark(drawing *draw2d.List, look theme.Look, mark codebase.Mark, x, top float32) {
	switch mark {
	case codebase.AddedMark:
		drawing.Rectangle(draw2d.Rectangle{X: x, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, look.Created, draw2d.Color{}, 0)
	case codebase.ModifiedMark:
		drawing.Rectangle(draw2d.Rectangle{X: x, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, look.Modified, draw2d.Color{}, 0)
	case codebase.RemovedAbove:
		drawing.Rectangle(draw2d.Rectangle{X: x - 1, Y: top - 2, Width: 8, Height: 4}, 2, look.Deleted, draw2d.Color{}, 0)
	case codebase.RemovedBelow:
		drawing.Rectangle(draw2d.Rectangle{X: x - 1, Y: top + view.rowHeight - 2, Width: 8, Height: 4}, 2, look.Deleted, draw2d.Color{}, 0)
	}
}

// scrollMarkWidth is how wide a mark beside the scroll bar is, and scrollMarkHeight how short it
// may be at least, in pixels, so a single line of a long file still shows.
const (
	scrollMarkWidth  float32 = 3
	scrollMarkHeight float32 = 2
)

// paintScrollMarks marks where the changes or the uses are along the scroll bar, as a map of the
// file: one mark per run of rows, worked out when the view was laid out.
func (view *Viewer) paintScrollMarks(drawing *draw2d.List, look theme.Look) {
	marks := view.scrollMarks
	if marks.rows == 0 {
		return
	}
	bar := view.List.BarArea()
	rowHeight := bar.Height / float32(marks.rows)
	for _, run := range marks.runs {
		y := bar.Y + float32(run.first)*rowHeight
		height := float32(run.count-1)*rowHeight + max(rowHeight, scrollMarkHeight)
		area := draw2d.Rectangle{X: bar.X - scrollMarkWidth, Y: y, Width: scrollMarkWidth, Height: height}
		drawing.Rectangle(area, 0, colourOf(run.kind, look), draw2d.Color{}, 0)
	}
}
func lineAt(lines []highlight.Line, number int) highlight.Line {
	if number < 1 || number > len(lines) {
		return highlight.Line{}
	}
	return lines[number-1]
}

func (view *Viewer) paintNumber(drawing *draw2d.List, number int, left, width, baseline float32, colour draw2d.Color) {
	if number <= 0 {
		return
	}
	text := view.numberText(number)
	x := left + width - 8 - float32(len(text))*view.charWidth
	drawing.Text(text, x, baseline, view.codeSize, draw2d.Mono, colour)
}

// numberText is a line's number in words, each made once, the first time its line shows.
func (view *Viewer) numberText(number int) string {
	if text, made := view.numbers[number]; made {
		return text
	}
	if view.numbers == nil {
		view.numbers = map[int]string{}
	}
	text := strconv.Itoa(number)
	view.numbers[number] = text
	return text
}

// paintCode paints a line's coloured pieces, shifted by the sideways scroll and cut at the area.
func (view *Viewer) paintCode(drawing *draw2d.List, look theme.Look, line highlight.Line, area draw2d.Rectangle, baseline float32) {
	width := float32(line.Columns) * view.charWidth
	view.List.Width = max(view.List.Width, width)
	drawing.PushClip(area)
	for _, span := range line.Spans {
		x := area.X + float32(span.Column)*view.charWidth - view.List.OffsetX
		if x > area.Right() {
			break
		}
		if x+float32(len(span.Text))*view.charWidth < area.X || isBlank(span.Text) {
			continue
		}
		drawing.Text(span.Text, x, baseline, view.codeSize, draw2d.Mono, look.Syntax.Colour(span.Kind))
	}
	drawing.PopClip()
}

func isBlank(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] != ' ' {
			return false
		}
	}
	return true
}
