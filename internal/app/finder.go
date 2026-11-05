package app

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// finderScope is what the finder searches: the names of the files, or what they hold.
type finderScope int

const (
	namesScope   finderScope = iota // fuzzy, over every file's project and path
	contentScope                    // the lines that hold the text typed, in every file
)

// finderState is the finder: type part of a file's name, or a text it holds, from any project,
// and open it with Enter, at the line found.
type finderState struct {
	open      bool
	focusNext bool
	scope     finderScope
	query     string
	asked     string // the query the matches are for
	stale     bool   // the projects changed since the matches were found
	matches   []review.Match
	selected  int
	list      pane.Scroller

	// the content search runs on other goroutines; search counts the searches asked for, so the
	// answer to one overtaken by more typing is dropped
	hits      []contentHit
	searching bool
	truncated bool
}

// contentHit is a line found by the content search.
type contentHit struct {
	project int
	changed bool
	status  git.Status
	git.Hit
}

const (
	finderLimit       = 200
	finderRowHeight   = 36
	contentRowHeight  = 50
	finderRows        = 12
	contentRows       = 9
	contentMinLength  = 2   // shorter texts match nearly every line
	contentHitLimit   = 400 // lines kept from a search across every project
	contentPerFile    = 6   // lines kept from each file
	contentPause      = 180 * time.Millisecond
	contentSearch     = "content search" // the key of the search running, which a newer one replaces
	finderPanelRadius = 14
)

func (finder *finderState) show(scope finderScope) {
	finder.open = true
	finder.focusNext = true
	finder.scope = scope
	finder.query = ""
	finder.stale = true
	finder.selected = 0
	finder.list.Offset = 0
	finder.hits, finder.searching = nil, false
}

func (finder *finderState) setScope(scope finderScope) {
	finder.scope = scope
	finder.stale = true
	finder.focusNext = true
}

func (finder *finderState) count() int {
	if finder.scope == contentScope {
		return len(finder.hits)
	}
	return len(finder.matches)
}

func (finder *finderState) rowHeight() float32 {
	if finder.scope == contentScope {
		return contentRowHeight
	}
	return finderRowHeight
}

func (finder *finderState) refresh(application *App) {
	if !finder.stale && finder.asked == finder.query {
		return
	}
	finder.asked = finder.query
	finder.stale = false
	finder.selected = 0
	finder.list.Offset = 0
	if finder.scope == namesScope {
		finder.matches = review.FuzzyFind(application.candidates, finder.query, finderLimit)
		return
	}
	finder.searchContent(application, strings.TrimSpace(finder.query))
}

// searchContent looks for a text in every project shown, a few at a time, after a short pause
// so that typing a word does not start a search for each of its letters.
func (finder *finderState) searchContent(application *App, text string) {
	if utf8.RuneCountInString(text) < contentMinLength {
		application.work.cancel(contentSearch)
		finder.hits, finder.searching, finder.truncated = nil, false, false
		return
	}
	type searched struct {
		project int
		path    string
	}
	var projects []searched
	for index, project := range application.projects {
		if project.Loaded && !application.hidden(project) && application.inScope(workspace.FileReference{Project: index, Path: ""}) {
			projects = append(projects, searched{index, project.Path})
		}
	}
	options := git.GrepOptions{WorkingTree: application.settings.WorkingTree, PerFile: contentPerFile}
	synchronous := application.settings.Synchronous
	finder.searching = true
	application.work.run(contentSearch, func(stop cancellation) func() {
		if !synchronous {
			select {
			case <-time.After(contentPause):
			case <-stop.Done():
				return nil // a newer search, typed during the pause
			}
		}
		found := make([][]git.Hit, len(projects))
		var group sync.WaitGroup
		for position, project := range projects {
			group.Add(1)
			go func() {
				defer group.Done()
				application.work.read(stop, func() {
					found[position], _ = git.Grep(project.path, text, options) // a project git cannot search shows no lines
				})
			}()
		}
		group.Wait()
		return func() {
			finder.hits, finder.truncated = nil, false
			for position, hits := range found {
				index := projects[position].project
				if index >= len(application.projects) {
					continue
				}
				project := application.projects[index]
				for _, hit := range hits {
					change, changed := project.ChangeOf(hit.Path)
					finder.hits = append(finder.hits, contentHit{index, changed, change.Status, hit})
				}
			}
			// the development's own files first: they are what is being reviewed
			sort.SliceStable(finder.hits, func(first, second int) bool {
				return finder.hits[first].changed && !finder.hits[second].changed
			})
			if len(finder.hits) > contentHitLimit {
				finder.hits, finder.truncated = finder.hits[:contentHitLimit], true
			}
			finder.searching = false
			finder.selected = 0
			finder.list.Offset = 0
		}
	})
}

func (finder *finderState) choose(application *App) {
	if finder.selected < 0 || finder.selected >= finder.count() {
		return
	}
	var reference workspace.FileReference
	line := 0
	if finder.scope == contentScope {
		hit := finder.hits[finder.selected]
		reference, line = workspace.FileReference{Project: hit.project, Path: hit.Path}, hit.Line
	} else {
		candidate := finder.matches[finder.selected].Candidate
		reference = workspace.FileReference{Project: candidate.Project, Path: candidate.Path}
	}
	finder.open = false
	application.openFileAt(reference, line)
	application.sidebar.revealFile(application, reference)
	application.focus = editorPane
}

func (finder *finderState) build(context *ui.Context, application *App) {
	if !finder.open {
		return
	}
	keys := application.keys
	switch {
	case keys.Typed(input.KeyDown):
		finder.selected++
	case keys.Typed(input.KeyUp):
		finder.selected--
	case keys.Typed(input.KeyPageDown):
		finder.selected += finderRows
	case keys.Typed(input.KeyPageUp):
		finder.selected -= finderRows
	case keys.Pressed(input.KeyTab):
		finder.setScope(1 - finder.scope)
	}
	palette := context.Theme
	look := application.look
	options := ui.LayerOptions{AlignX: ui.Center, AlignY: ui.Start, Padding: ui.Padding{Top: 64}, Width: ui.Fixed(760)}
	dismissed := context.Modal("finder", options, func() {
		panel := ui.Look{Fill: palette.Surface, Border: look.Chosen, BorderWidth: 1, Radius: finderPanelRadius, Shadow: true}
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(8), Gap: 6}, panel, func() {
			submitted := finder.buildQuery(context, application)
			finder.refresh(application)
			count := finder.count()
			finder.selected = min(max(finder.selected, 0), max(count-1, 0))
			finder.list.RowHeight = finder.rowHeight()
			finder.list.Count = count
			finder.list.Reveal(finder.selected)
			onBar := finder.list.Follow(context, keys, palette.ScrollStep)
			x, y := context.Pointer()
			if index := finder.list.RowAt(x, y); index >= 0 && !onBar && keys.Clicked() {
				finder.selected = index
				submitted = true
			}
			rows := min(max(count, 1), finderRows)
			if finder.scope == contentScope {
				rows = min(max(count, 1), contentRows)
			}
			height := float32(rows) * finder.rowHeight()
			context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(height)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
				finder.list.Place(area)
				finder.paint(context, application, drawing)
			})
			finder.buildFooter(context)
			if submitted {
				finder.choose(application)
			}
		})
	})
	if dismissed {
		finder.open = false
	}
}

// buildQuery is the field typed in, as a pill like the window's search field, with the switch
// between names and content on its right. It tells whether Enter was pressed in it.
func (finder *finderState) buildQuery(context *ui.Context, application *App) bool {
	palette := context.Theme
	look := application.look
	submitted := false
	pill := ui.Look{Fill: look.Hover.WithAlpha(0.7), Radius: 20, Border: palette.Primary.WithAlpha(0.5), BorderWidth: 1}
	context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(40), Padding: ui.Padding{Left: 14, Right: 5}, Gap: 10, AlignY: ui.Center}, pill, func() {
		icon, placeholder := "search", "Find a file by its name, in any project"
		if finder.scope == contentScope {
			icon, placeholder = "text-search", "Find a text in the files of every project"
		}
		context.IconSized(icon, 16, palette.InkSoft)
		if finder.focusNext {
			context.FocusField("query")
			finder.focusNext = false
		}
		submitted = context.EntryField("query", &finder.query, ui.TextFieldOptions{Placeholder: placeholder, Plain: true, Width: ui.Grow()})
		scope := int(finder.scope)
		if theme.IconSwitch(context, "finder scope", &scope, []theme.IconChoice{
			{Icon: "file-search", Name: "File names (Tab)"}, {Icon: "text-search", Name: "File content (Tab)"},
		}) {
			finder.setScope(finderScope(scope))
		}
	})
	return submitted
}

// buildFooter says what was found and the keys, each in a key cap.
func (finder *finderState) buildFooter(context *ui.Context) {
	palette := context.Theme
	quiet := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(30), Padding: ui.PadXY(8, 0), Gap: 6, AlignY: ui.Center}, func() {
		switch {
		case finder.scope == contentScope && finder.searching:
			context.Text("Searching…", quiet)
		case finder.scope == contentScope && utf8.RuneCountInString(strings.TrimSpace(finder.query)) < contentMinLength:
			context.Text("Type two letters or more; a capital makes it match case", quiet)
		case finder.scope == contentScope:
			words := fmt.Sprintf("%d lines", len(finder.hits))
			if finder.truncated {
				words = fmt.Sprintf("the first %d lines", contentHitLimit)
			}
			context.Text(words, quiet)
		default:
			context.Text(fmt.Sprintf("%d files", len(finder.matches)), quiet)
		}
		context.Spacer()
		for _, hint := range [][2]string{{"↑↓", "choose"}, {"Enter", "open"}, {"Tab", "names or content"}, {"Esc", "close"}} {
			keyCap(context, hint[0])
			context.Text(hint[1], quiet)
			context.Box(ui.Layout{Width: ui.Fixed(4)}, nil)
		}
	})
}

// keyCap draws a key as a small rounded cap.
func keyCap(context *ui.Context, key string) {
	palette := context.Theme
	context.Frame(ui.Layout{Height: ui.Fixed(20), Padding: ui.PadXY(7, 0), AlignY: ui.Center},
		ui.Look{Fill: palette.Chosen.WithAlpha(0.6), Radius: 6}, func() {
			context.Text(key, ui.TextStyle{Size: palette.SmallTextSize - 1, Color: palette.InkSoft, Weight: draw2d.Medium})
		})
}

func (finder *finderState) paint(context *ui.Context, application *App, drawing *draw2d.List) {
	palette := context.Theme
	area := finder.list.Area
	drawing.PushClip(area)
	if finder.count() == 0 {
		message := "No file matches"
		if finder.scope == contentScope {
			message = "No line holds it"
			if finder.searching || utf8.RuneCountInString(strings.TrimSpace(finder.query)) < contentMinLength {
				message = ""
			}
		}
		drawing.Text(message, area.X+14, area.Y+24, palette.TextSize, draw2d.Regular, palette.InkQuiet)
	}
	pointerX, pointerY := context.Pointer()
	hovered := finder.list.RowAt(pointerX, pointerY)
	first, last := finder.list.Visible()
	for index := first; index < last; index++ {
		top := finder.list.RowTop(index)
		rowArea := draw2d.Rectangle{X: area.X, Y: top + 1, Width: area.Width - pane.ScrollBarWidth - 2, Height: finder.rowHeight() - 2}
		switch index {
		case finder.selected:
			drawing.Rectangle(rowArea, 8, application.look.Chosen, draw2d.Color{}, 0)
		case hovered:
			drawing.Rectangle(rowArea, 8, application.look.Hover, draw2d.Color{}, 0)
		}
		if finder.scope == contentScope {
			finder.paintHit(context, application, drawing, finder.hits[index], rowArea)
		} else {
			finder.paintName(context, application, drawing, finder.matches[index], rowArea)
		}
	}
	finder.list.PaintBar(drawing, application.look.ScrollThumb)
	drawing.PopClip()
}

// paintName paints a file found by its name: its icon, its name and, quieter, its project and
// folder, with the letters that matched in the accent; its status on the right.
func (finder *finderState) paintName(context *ui.Context, application *App, drawing *draw2d.List, match review.Match, row draw2d.Rectangle) {
	palette := context.Theme
	measurer := context.Measurer()
	size := palette.TextSize
	metrics := measurer.MeasureText("Ag", size, draw2d.Regular, 1)
	baseline := row.Y + (row.Height+metrics.Ascent-metrics.Descent)/2
	candidate := match.Candidate
	project := application.projects[candidate.Project]
	change, changed := project.ChangeOf(candidate.Path)
	icon, iconInk := theme.FileIcon(candidate.Path, application.look)
	drawing.Icon(icon, row.X+12, row.Y+(row.Height-16)/2, 16, iconInk)
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
func (finder *finderState) paintHit(context *ui.Context, application *App, drawing *draw2d.List, hit contentHit, row draw2d.Rectangle) {
	palette := context.Theme
	measurer := context.Measurer()
	const codeSize = 13
	code := measurer.MeasureText("M", codeSize, draw2d.Mono, 1)
	small := measurer.MeasureText("Ag", palette.SmallTextSize, draw2d.Regular, 1)
	icon, iconInk := theme.FileIcon(hit.Path, application.look)
	drawing.Icon(icon, row.X+12, row.Y+9, 16, iconInk)
	left := row.X + 40
	drawing.PushClip(draw2d.Rectangle{X: row.X, Y: row.Y, Width: row.Width - 8, Height: row.Height})
	// the line from its first letter, or from a little before what was found in a long line
	before := hit.Text[:min(hit.Column, len(hit.Text))]
	text := strings.ReplaceAll(hit.Text, "\t", "    ")
	column := len(before) + 3*strings.Count(before, "\t")
	start := len(text) - len(strings.TrimLeft(text, " "))
	if column-start > 48 {
		start = column - 24
	}
	start = min(start, column, len(text))
	shown := text[start:]
	codeBaseline := row.Y + 8 + code.Ascent
	found := utf8.RuneCountInString(strings.TrimSpace(finder.asked))
	matchX := left + float32(utf8.RuneCountInString(text[start:min(column, len(text))]))*code.Width
	wash := draw2d.Rectangle{X: matchX - 1, Y: row.Y + 6, Width: float32(found)*code.Width + 2, Height: code.Ascent + code.Descent + 4}
	drawing.Rectangle(wash, 3, palette.Primary.WithAlpha(0.3), draw2d.Color{}, 0)
	drawing.Text(shown, left, codeBaseline, codeSize, draw2d.Mono, application.look.Syntax.RoleColour(highlight.Plain))
	where := fmt.Sprintf("%s/%s:%d", application.projects[hit.project].Name, hit.Path, hit.Line)
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
	matched := map[int]bool{}
	for _, position := range positions {
		matched[position] = true
	}
	for start < end {
		next := start + 1
		for next < end && matched[next] == matched[start] {
			next++
		}
		piece := label[start:next]
		weight, colour := draw2d.Regular, ink
		if matched[start] {
			weight, colour = draw2d.Bold, accent
		}
		drawing.Text(piece, x, baseline, size, weight, colour)
		x += measurer.MeasureText(piece, size, weight, 1).Width
		start = next
	}
	return x
}
