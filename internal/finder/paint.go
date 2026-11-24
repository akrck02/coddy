package finder

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// finderRowRadius is how round a result's background is, in pixels.
const finderRowRadius = 8

func (finder *Finder) paint(context *ui.Context, window Window, drawing *draw2d.List) {
	palette := context.Theme
	area := finder.list.Area
	drawing.PushClip(area)
	if finder.count() == 0 {
		message := finder.from.nothingFound(finder)
		drawing.Text(message, area.X+14, area.Y+24, palette.TextSize, draw2d.Regular, palette.InkQuiet)
	}
	pointerX, pointerY := context.Pointer()
	hovered := finder.list.RowAt(pointerX, pointerY)
	first, last := finder.list.Visible()
	for index := first; index < last; index++ {
		top := finder.list.RowTop(index)
		rowArea := draw2d.Rectangle{X: area.X, Y: top + 1, Width: area.Width - pane.ScrollBarWidth - 2, Height: finder.from.rowHeight() - 2}
		state := theme.RowPlain
		switch index {
		case finder.selected:
			state = theme.RowChosen
		case hovered:
			state = theme.RowHovered
		}
		theme.PaintRowBackground(drawing, window.Look(), rowArea, finderRowRadius, state)
		finder.from.paintRow(finder, context, window, drawing, index, rowArea)
	}
	finder.list.PaintBar(drawing, window.Look().ScrollThumb)
	drawing.PopClip()
}

// paintName paints a file found by its name: its icon, its name and, quieter, its project and
// folder, with the letters that matched in the accent; its status on the right.
func (finder *Finder) paintName(context *ui.Context, window Window, drawing *draw2d.List, match Match, row draw2d.Rectangle) {
	palette := context.Theme
	measurer := context.Measurer()
	size := palette.TextSize
	metrics := measurer.MeasureText("Ag", size, draw2d.Regular, 1)
	baseline := row.Y + (row.Height+metrics.Ascent-metrics.Descent)/2
	candidate := match.Candidate
	project := window.Projects()[candidate.Project]
	change, changed := project.ChangeOf(candidate.Path)
	theme.DrawIcon(drawing, window.Look(), theme.FileIcon(candidate.Path, window.Look()), row.X+12, row.Y+(row.Height-16)/2, 16)
	nameInk := palette.Ink
	if changed {
		nameInk = theme.StatusColour(palette, change.Status)
		letter := change.Status.Letter()
		width := measurer.MeasureText(letter, palette.SmallTextSize, draw2d.Mono, 1).Width
		drawing.Text(letter, row.Right()-14-width, baseline, palette.SmallTextSize, draw2d.Mono, nameInk)
	}
	// the label is "project/folder/name": the name first and plain, then where it is, quiet
	label := candidate.Label
	slash := strings.LastIndexByte(label, '/')
	x := row.X + 40
	drawing.PushClip(draw2d.Rectangle{X: row.X, Y: row.Y, Width: row.Width - 40, Height: row.Height})
	x = paintMatched(drawing, measurer, label, match.Positions, slash+1, len(label), x, baseline, size, nameInk, palette.Primary)
	paintMatched(drawing, measurer, label, match.Positions, 0, max(slash, 0), x+10, baseline, palette.SmallTextSize, palette.InkQuiet, palette.Primary)
	drawing.PopClip()
}

// paintHit paints a line found by its content: the line itself in the code's face, the text found
// washed in the accent, and under it the file and the line's number.
func (finder *Finder) paintHit(context *ui.Context, window Window, drawing *draw2d.List, hit ContentHit, row draw2d.Rectangle) {
	palette := context.Theme
	measurer := context.Measurer()
	const codeSize = 13
	code := measurer.MeasureText("M", codeSize, draw2d.Mono, 1)
	small := measurer.MeasureText("Ag", palette.SmallTextSize, draw2d.Regular, 1)
	theme.DrawIcon(drawing, window.Look(), theme.FileIcon(hit.Path, window.Look()), row.X+12, row.Y+9, 16)
	left := row.X + 40
	drawing.PushClip(draw2d.Rectangle{X: row.X, Y: row.Y, Width: row.Width - 8, Height: row.Height})
	// the line from its first letter, or from a little before what was found in a long line
	before := hit.Text[:min(hit.Column, len(hit.Text))]
	text := hit.text
	column := len(before) + 3*strings.Count(before, "\t")
	start := excerptStart(text, column)
	shown := text[start:]
	codeBaseline := row.Y + 8 + code.Ascent
	found := utf8.RuneCountInString(strings.TrimSpace(finder.asked))
	if hit.length > 0 {
		found = hit.length
	}
	matchX := left + float32(utf8.RuneCountInString(text[start:min(column, len(text))]))*code.Width
	wash := draw2d.Rectangle{X: matchX - 1, Y: row.Y + 6, Width: float32(found)*code.Width + 2, Height: code.Ascent + code.Descent + 4}
	drawing.Rectangle(wash, 3, palette.Primary.WithAlpha(0.3), draw2d.Color{}, 0)
	drawing.Text(shown, left, codeBaseline, codeSize, draw2d.Mono, window.Look().Syntax.RoleColour(highlight.Plain))
	where := hit.where
	whereInk := palette.InkQuiet
	if hit.changed {
		whereInk = theme.StatusColour(palette, hit.status).WithAlpha(0.85)
	}
	drawing.Text(where, left, row.Bottom()-8-small.Descent, palette.SmallTextSize, draw2d.Regular, whereInk)
	drawing.PopClip()
}

// paintMatched paints the part of a label from start to end, with the characters that matched the
// query in bold and in the accent, and returns where it ends.
func paintMatched(drawing *draw2d.List, measurer draw2d.Measurer, label string, positions []int, start, end int, x, baseline, size float32,
	ink, accent draw2d.Color) float32 {
	matched := func(index int) bool { // positions grow, so a search finds one without a map
		found, at := slices.BinarySearch(positions, index)
		return at && found >= 0
	}
	for start < end {
		next := start + 1
		for next < end && matched(next) == matched(start) {
			next++
		}
		piece := label[start:next]
		weight, colour := draw2d.Regular, ink
		if matched(start) {
			weight, colour = draw2d.Bold, accent
		}
		drawing.Text(piece, x, baseline, size, weight, colour)
		x += measurer.MeasureText(piece, size, weight, 1).Width
		start = next
	}
	return x
}

// A long line is shown from a little before its match: when the match lies further than
// longLineLead bytes from the line's first letter, the line is shown from matchLead bytes before it.
const (
	longLineLead = 48
	matchLead    = 24
)

// excerptStart is where the shown part of a hit's line starts: its first letter, or a little before
// the match in a long line. It never starts on a space, inside a character or after the match, so
// the line shows no indentation and the match stays in sight.
func excerptStart(text string, column int) int {
	column = min(column, len(text))
	start := len(text) - len(strings.TrimLeft(text, " "))
	if column-start > longLineLead {
		start = column - matchLead
	}
	for start < column && (text[start] == ' ' || !utf8.RuneStart(text[start])) {
		start++
	}
	return min(start, column)
}
