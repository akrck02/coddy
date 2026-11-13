package viewer

import (
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

// columnWidths are each column's width: its widest cell's, shrunk in proportion when the table is
// wider than the text.
func (layout *formatter) columnWidths(block markdown.Block, base wordStyle) []float32 {
	var widths []float32
	for _, row := range block.Rows {
		for column, cell := range row {
			for len(widths) <= column {
				widths = append(widths, 2*tablePadding)
			}
			width := float32(2 * tablePadding)
			for _, span := range cell {
				width += layout.measure(span.Text, layout.styleOf(span, base))
			}
			widths[column] = max(widths[column], width+8)
		}
	}
	total := float32(0)
	for _, width := range widths {
		total += width
	}
	if total > layout.width {
		for column := range widths {
			widths[column] *= layout.width / total
		}
	}
	return widths
}
