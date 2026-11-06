package sidebar

import (
	"fmt"
	"sort"
	"strings"

	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
)

// serverBadge is what a project's badge says of the files language servers wrote in it: each
// server with how many, "jdtls · 4"; "" when they wrote none.
func serverBadge(project *workspace.Project) string {
	counts := map[string]int{}
	for _, file := range project.ServerFiles {
		counts[file.Server]++
	}
	var parts []string
	for server, count := range counts {
		parts = append(parts, fmt.Sprintf("%s · %d", server, count))
	}
	sort.Strings(parts)
	return strings.Join(parts, "  ")
}

// paintServerBadge draws the quiet, dashed badge of what servers wrote in a project, keeps where it
// is for a click, and moves the row's right side left of it.
func (sidebar *Sidebar) paintServerBadge(side *rightSide, row Row, index int, top float32) {
	words := serverBadge(side.painter.window.Projects()[row.Project])
	if words == "" {
		return
	}
	palette := side.painter.context.Theme
	small := palette.SmallTextSize
	width := side.painter.context.Measurer().MeasureText(words, small, draw2d.Mono, 1).Width + 12
	box := draw2d.Rectangle{X: side.right - width, Y: top + 4, Width: width, Height: RowHeight - 8}
	paintDashedBox(side.painter.drawing, box, palette.InkQuiet.WithAlpha(0.7))
	side.painter.drawing.Text(words, box.X+6, side.baseline, small, draw2d.Mono, palette.InkQuiet)
	sidebar.badges[index] = [2]float32{box.X, box.Right()}
	side.right = box.X - 8
}

// paintDashedBox draws a rectangle's outline in dashes, one pixel thick.
func paintDashedBox(drawing *draw2d.List, box draw2d.Rectangle, colour draw2d.Color) {
	const dash, gap = 3, 2
	for x := box.X; x < box.Right(); x += dash + gap {
		length := min(dash, box.Right()-x)
		drawing.Rectangle(draw2d.Rectangle{X: x, Y: box.Y, Width: length, Height: 1}, 0, colour, draw2d.Color{}, 0)
		drawing.Rectangle(draw2d.Rectangle{X: x, Y: box.Bottom() - 1, Width: length, Height: 1}, 0, colour, draw2d.Color{}, 0)
	}
	for y := box.Y; y < box.Bottom(); y += dash + gap {
		length := min(dash, box.Bottom()-y)
		drawing.Rectangle(draw2d.Rectangle{X: box.X, Y: y, Width: 1, Height: length}, 0, colour, draw2d.Color{}, 0)
		drawing.Rectangle(draw2d.Rectangle{X: box.Right() - 1, Y: y, Width: 1, Height: length}, 0, colour, draw2d.Color{}, 0)
	}
}

// onBadge tells whether a click at x on a row fell on its project's badge of server files.
func (sidebar *Sidebar) onBadge(index int, x float32) bool {
	span, shown := sidebar.badges[index]
	return shown && sidebar.Rows[index].Kind == ProjectRow && x >= span[0] && x <= span[1]
}
