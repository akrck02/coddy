package viewer

import (
	"strings"

	"github.com/akrck02/code-review/internal/markdown"
	"github.com/akrck02/go-renderer/draw2d"
)

// tablePadding is the room inside a table's cells.
const tablePadding = 10

// table lays a table out: its columns as wide as their words, shrunk to fit, the head bold on a
// shade, and a line between rows.
func (layout *formatter) table(block markdown.Block, base wordStyle) {
	widths := layout.columnWidths(block, base)
	total := float32(0)
	for _, width := range widths {
		total += width
	}
	top := layout.y
	for index, row := range block.Rows {
		style := base
		if index == 0 {
			style.weight = draw2d.Bold
		}
		rowTop, shapesBefore := layout.y, len(layout.shapes)
		bottom := layout.tableRow(row, widths, style)
		if index == 0 {
			shade := shape{area: draw2d.Rectangle{Y: rowTop, Width: total, Height: bottom - rowTop}, colour: layout.look.Hover}
			layout.shapes = append(layout.shapes[:shapesBefore], append([]shape{shade}, layout.shapes[shapesBefore:]...)...)
		}
		layout.box(draw2d.Rectangle{Y: bottom, Width: total, Height: 1}, layout.palette.Line, 0)
		layout.y = bottom + 1
	}
	layout.box(draw2d.Rectangle{Y: top, Width: total, Height: 1}, layout.palette.Line, 0)
}

// tableRow lays a row's cells out, each in its column, and tells where the tallest ends.
func (layout *formatter) tableRow(row [][]markdown.Span, widths []float32, style wordStyle) float32 {
	bottom, left := layout.y, float32(0)
	for column, width := range widths {
		if column < len(row) {
			end := layout.flow(row[column], left+tablePadding, left+width-tablePadding, layout.y+tablePadding/2, style)
			bottom = max(bottom, end+tablePadding/2)
		}
		left += width
	}
	return max(bottom, layout.y+layout.lineHeight(style)+tablePadding)
}

// columnWidths are each column's width: its widest cell's, shrunk when the table is wider than
// the text. What a column loses is taken from the room beyond its longest word first, so a word is
// cut only when the words alone do not fit.
func (layout *formatter) columnWidths(block markdown.Block, base wordStyle) []float32 {
	var widths, least []float32
	for _, row := range block.Rows {
		for column, cell := range row {
			for len(widths) <= column {
				widths, least = append(widths, 2*tablePadding), append(least, 2*tablePadding)
			}
			widths[column] = max(widths[column], layout.cellWidth(cell, base)+2*tablePadding)
			least[column] = max(least[column], layout.longestWord(cell, base)+2*tablePadding)
		}
	}
	return shrunk(widths, least, layout.width)
}

// cellWidth is how wide a cell's words are on one line.
func (layout *formatter) cellWidth(cell []markdown.Span, base wordStyle) float32 {
	width := float32(0)
	for _, span := range cell {
		style := layout.styleOf(span, base)
		for _, word := range layout.words(span, style, layout.width) {
			width += layout.measure(word, style)
			if style.code && strings.TrimSpace(word) != "" {
				width += 2 * codePadding
			}
		}
	}
	return width
}

// longestWord is how wide a cell's widest word is: the least its column can be without cutting it.
// Code counts by its words too, since it goes on to the next line at its spaces when its column is
// narrower than it.
func (layout *formatter) longestWord(cell []markdown.Span, base wordStyle) float32 {
	longest := float32(0)
	for _, span := range cell {
		style := layout.styleOf(span, base)
		for _, word := range strings.SplitAfter(span.Text, " ") {
			longest = max(longest, layout.wordWidth(word, style))
		}
	}
	return longest
}

// shrunk fits widths in room: each loses a share of what it has beyond its least, or, when the
// leasts alone do not fit, each is shrunk in proportion.
func shrunk(widths, least []float32, room float32) []float32 {
	total, leastTotal := float32(0), float32(0)
	for column := range widths {
		total, leastTotal = total+widths[column], leastTotal+least[column]
	}
	if total <= room {
		return widths
	}
	if leastTotal > room {
		for column := range widths {
			widths[column] *= room / total
		}
		return widths
	}
	share := (room - leastTotal) / (total - leastTotal)
	for column := range widths {
		widths[column] = least[column] + (widths[column]-least[column])*share
	}
	return widths
}
