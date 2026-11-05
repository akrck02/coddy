package app

import (
	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// finderState is the fuzzy finder: type part of a file's name or path, from any project, and open
// it with Enter.
type finderState struct {
	open      bool
	focusNext bool
	query     string
	asked     string // the query the matches are for
	stale     bool   // the projects changed since the matches were found
	matches   []review.Match
	selected  int
	list      scroller
}

const (
	finderLimit     = 200
	finderRowHeight = 30
	finderRows      = 14
)

func (finder *finderState) show() {
	finder.open = true
	finder.focusNext = true
	finder.query = ""
	finder.stale = true
	finder.selected = 0
	finder.list.offset = 0
}

func (finder *finderState) refresh(application *App) {
	if !finder.stale && finder.asked == finder.query {
		return
	}
	finder.matches = review.FuzzyFind(application.candidates, finder.query, finderLimit)
	finder.asked = finder.query
	finder.stale = false
	finder.selected = 0
	finder.list.offset = 0
}

func (finder *finderState) choose(application *App) {
	if finder.selected < 0 || finder.selected >= len(finder.matches) {
		return
	}
	candidate := finder.matches[finder.selected].Candidate
	reference := fileReference{candidate.Project, candidate.Path}
	finder.open = false
	application.openFile(reference)
	application.sidebar.revealFile(application, reference)
	application.focus = editorPane
}

func (finder *finderState) build(context *ui.Context, application *App) {
	if !finder.open {
		return
	}
	keys := application.keys
	switch {
	case keys.typed(input.KeyDown):
		finder.selected++
	case keys.typed(input.KeyUp):
		finder.selected--
	case keys.typed(input.KeyPageDown):
		finder.selected += finderRows
	case keys.typed(input.KeyPageUp):
		finder.selected -= finderRows
	}
	theme := context.Theme
	options := ui.LayerOptions{AlignX: ui.Center, AlignY: ui.Start, Padding: ui.Padding{Top: 72}, Width: ui.Fixed(720)}
	dismissed := context.Modal("finder", options, func() {
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(10), Gap: 8},
			ui.Look{Fill: theme.Surface, Border: theme.Line, BorderWidth: 1, Radius: theme.PanelRadius, Shadow: true}, func() {
				if finder.focusNext {
					context.FocusField("query")
					finder.focusNext = false
				}
				submitted := context.EntryField("query", &finder.query, ui.TextFieldOptions{Placeholder: "Find a file in any project (changed files first)"})
				finder.refresh(application)
				finder.selected = min(max(finder.selected, 0), max(len(finder.matches)-1, 0))
				finder.list.rowHeight = finderRowHeight
				finder.list.count = len(finder.matches)
				finder.list.reveal(finder.selected)
				onBar := finder.list.follow(context, keys, theme.ScrollStep)
				x, y := context.Pointer()
				if index := finder.list.rowAt(x, y); index >= 0 && !onBar && keys.clicked() {
					finder.selected = index
					submitted = true
				}
				height := float32(min(max(len(finder.matches), 1), finderRows)) * finderRowHeight
				context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(height)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
					finder.list.place(area)
					finder.paint(context, application, drawing)
				})
				context.Caption("↑↓ choose · Enter open · Esc close")
				if submitted {
					finder.choose(application)
				}
			})
	})
	if dismissed {
		finder.open = false
	}
}

func (finder *finderState) paint(context *ui.Context, application *App, drawing *draw2d.List) {
	theme := context.Theme
	area := finder.list.area
	measurer := context.Measurer()
	size := theme.TextSize
	metrics := measurer.MeasureText("Ag", size, draw2d.Regular, 1)
	drawing.PushClip(area)
	if len(finder.matches) == 0 {
		drawing.Text("No file matches", area.X+8, area.Y+20, size, draw2d.Regular, theme.InkQuiet)
	}
	first, last := finder.list.visible()
	for index := first; index < last; index++ {
		match := finder.matches[index]
		top := finder.list.rowTop(index)
		rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width - scrollBarWidth, Height: finderRowHeight}
		if index == finder.selected {
			drawing.Rectangle(rowArea, theme.ControlRadius, theme.PrimaryTint, draw2d.Color{}, 0)
		}
		baseline := top + (finderRowHeight+metrics.Ascent-metrics.Descent)/2
		candidate := match.Candidate
		project := application.projects[candidate.Project]
		change, changed := project.ChangeOf(candidate.Path)
		x := area.X + 8
		status := " "
		statusInk := theme.InkQuiet
		if changed {
			status, statusInk = change.Status.Letter(), statusColour(theme, change.Status)
		}
		drawing.Text(status, x, baseline, size, draw2d.Mono, statusInk)
		x += 20
		icon, iconInk := fileIcon(candidate.Path, application.look)
		drawing.Icon(icon, x, top+(finderRowHeight-15)/2, 15, iconInk)
		x += 22
		paintMatched(drawing, measurer, candidate.Label, match.Positions, x, baseline, size, theme, changed, change.Status)
	}
	finder.list.paintBar(drawing, theme.InkQuiet.WithAlpha(0.4))
	drawing.PopClip()
}

// paintMatched paints a label with the characters that matched the query in bold.
func paintMatched(drawing *draw2d.List, measurer draw2d.Measurer, label string, positions []int, x, baseline, size float32,
	theme ui.Theme, changed bool, status git.Status) {
	ink := theme.Ink
	if changed {
		ink = statusColour(theme, status)
	}
	matched := map[int]bool{}
	for _, position := range positions {
		matched[position] = true
	}
	start := 0
	for start < len(label) {
		end := start + 1
		for end < len(label) && matched[end] == matched[start] {
			end++
		}
		piece := label[start:end]
		weight, colour := draw2d.Regular, ink
		if matched[start] {
			weight, colour = draw2d.Bold, theme.Primary
		}
		drawing.Text(piece, x, baseline, size, weight, colour)
		x += measurer.MeasureText(piece, size, weight, 1).Width
		start = end
	}
}
