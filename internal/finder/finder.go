// Package finder finds a file by its name or by a text it holds, in any project, and opens it
// at the line found.
package finder

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// Scope is what the finder searches: the names of the files, or what they hold.
type Scope int

const (
	NamesScope      Scope = iota // fuzzy, over every file's project and path
	ContentScope                 // the lines that hold the text typed, in every file
	ReferencesScope              // the places a name is used, from a language server; typing narrows them
)

// Window is what the finder needs from the window it opens over.
type Window interface {
	Look() theme.Look
	Keys() pane.Keys
	Projects() []*workspace.Project
	// Candidates are every file the names search can find.
	Candidates() []Candidate
	// Searchable tells whether the content search looks in a project: one read, shown and in
	// the project entered, if any.
	Searchable(project int) bool
	// WorkingTree tells whether the changes not committed yet count.
	WorkingTree() bool
	// Synchronous tells whether work runs at once, for a capture or a test: the search does not
	// wait for the typing to pause.
	Synchronous() bool
	// Run, Cancel and Read are the window's background work: see its work.
	Run(key string, do func(stop context.Context) func())
	Cancel(key string)
	Read(stop context.Context, reading func()) bool
	// OpenAt opens a file at a line, shows it in the sidebar and gives it the keyboard.
	OpenAt(file workspace.FileReference, line int)
}

// Finder is the finder: type part of a file's name, or a text it holds, from any project,
// and open it with Enter, at the line found.
type Finder struct {
	Open      bool
	focusNext bool
	Scope     Scope
	Query     string
	asked     string // the query the matches are for
	Stale     bool   // the projects changed since the matches were found
	matches   []Match
	selected  int
	list      pane.Scroller

	// the content search runs as a job of the window, under a key: a newer search, typed while one
	// runs, replaces it
	Hits      []ContentHit
	searching bool
	truncated bool

	// the places a language server gave (the uses of a name, the files it wrote), all of them, and
	// what they are ("uses of Keeper"); Hits are those the query keeps
	Name       string
	references []ContentHit
}

// ContentHit is a line found by the content search.
type ContentHit struct {
	project int
	changed bool
	status  git.Status
	git.Hit
	length int // how many characters were found, for a use of a name; 0 is the query's length
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

func (finder *Finder) Show(scope Scope) {
	finder.Open = true
	finder.focusNext = true
	finder.Scope = scope
	finder.Query = ""
	finder.Stale = true
	finder.selected = 0
	finder.list.Offset = 0
	finder.Hits, finder.searching = nil, false
}

func (finder *Finder) setScope(scope Scope) {
	finder.Scope = scope
	finder.Stale = true
	finder.focusNext = true
}

func (finder *Finder) count() int {
	if finder.Scope != NamesScope {
		return len(finder.Hits)
	}
	return len(finder.matches)
}

func (finder *Finder) rowHeight() float32 {
	if finder.Scope != NamesScope {
		return contentRowHeight
	}
	return finderRowHeight
}

func (finder *Finder) refresh(window Window) {
	if !finder.Stale && finder.asked == finder.Query {
		return
	}
	finder.asked = finder.Query
	finder.Stale = false
	finder.selected = 0
	finder.list.Offset = 0
	switch finder.Scope {
	case NamesScope:
		finder.matches = FuzzyFind(window.Candidates(), finder.Query, finderLimit)
		return
	case ReferencesScope:
		finder.Hits = narrowed(finder.references, finder.Query, window)
		return
	}
	finder.searchContent(window, strings.TrimSpace(finder.Query))
}

// searchedProject is a project the content search reads: its place in the window and its folder.
type searchedProject struct {
	project int
	path    string
}

// searchContent looks for a text in every project shown, a few at a time, after a short pause
// so that typing a word does not start a search for each of its letters.
func (finder *Finder) searchContent(window Window, text string) {
	if utf8.RuneCountInString(text) < contentMinLength {
		window.Cancel(contentSearch)
		finder.Hits, finder.searching, finder.truncated = nil, false, false
		return
	}
	projects := searchableProjects(window)
	options := git.GrepOptions{WorkingTree: window.WorkingTree(), PerFile: contentPerFile}
	synchronous := window.Synchronous()
	finder.searching = true
	window.Run(contentSearch, func(stop context.Context) func() {
		if !synchronous {
			select {
			case <-time.After(contentPause):
			case <-stop.Done():
				return nil // a newer search, typed during the pause
			}
		}
		found := grepProjects(stop, window, projects, text, options)
		return func() { finder.takeHits(window, projects, found) }
	})
}

// searchableProjects lists the projects the window lets the finder search.
func searchableProjects(window Window) []searchedProject {
	var projects []searchedProject
	for index, project := range window.Projects() {
		if window.Searchable(index) {
			projects = append(projects, searchedProject{index, project.Path})
		}
	}
	return projects
}

// grepProjects searches every project at once, as many as the window lets read together, and
// returns each one's hits in the same order.
func grepProjects(stop context.Context, window Window, projects []searchedProject, text string, options git.GrepOptions) [][]git.Hit {
	found := make([][]git.Hit, len(projects))
	var group sync.WaitGroup
	for position, project := range projects {
		group.Add(1)
		go func() {
			defer group.Done()
			window.Read(stop, func() {
				found[position], _ = git.Grep(project.path, text, options) // a project git cannot search shows no lines
			})
		}()
	}
	group.Wait()
	return found
}

// takeHits lists the hits found, the development's own files first, up to the limit.
func (finder *Finder) takeHits(window Window, projects []searchedProject, found [][]git.Hit) {
	finder.Hits, finder.truncated = nil, false
	for position, hits := range found {
		index := projects[position].project
		if index >= len(window.Projects()) {
			continue
		}
		project := window.Projects()[index]
		for _, hit := range hits {
			change, changed := project.ChangeOf(hit.Path)
			finder.Hits = append(finder.Hits, ContentHit{index, changed, change.Status, hit, 0})
		}
	}
	// the development's own files first: they are what is being reviewed
	sort.SliceStable(finder.Hits, func(first, second int) bool {
		return finder.Hits[first].changed && !finder.Hits[second].changed
	})
	if len(finder.Hits) > contentHitLimit {
		finder.Hits, finder.truncated = finder.Hits[:contentHitLimit], true
	}
	finder.searching = false
	finder.selected = 0
	finder.list.Offset = 0
}

func (finder *Finder) choose(window Window) {
	if finder.selected < 0 || finder.selected >= finder.count() {
		return
	}
	var reference workspace.FileReference
	line := 0
	if finder.Scope != NamesScope {
		hit := finder.Hits[finder.selected]
		reference, line = workspace.FileReference{Project: hit.project, Path: hit.Path}, hit.Line
	} else {
		candidate := finder.matches[finder.selected].Candidate
		reference = workspace.FileReference{Project: candidate.Project, Path: candidate.Path}
	}
	finder.Open = false
	window.OpenAt(reference, line)
}

// Build adds the finder over the window while it is open: the field, the list and the footer.
func (finder *Finder) Build(context *ui.Context, window Window) {
	if !finder.Open {
		return
	}
	finder.followKeys(window.Keys())
	palette := context.Theme
	look := window.Look()
	options := ui.LayerOptions{AlignX: ui.Center, AlignY: ui.Start, Padding: ui.Padding{Top: 64}, Width: ui.Fixed(760)}
	dismissed := context.Modal("finder", options, func() {
		panel := ui.Look{Fill: palette.Surface, Border: look.Chosen, BorderWidth: 1, Radius: finderPanelRadius, Shadow: true}
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(8), Gap: 6}, panel, func() {
			finder.buildPanel(context, window)
		})
	})
	if dismissed {
		finder.Open = false
	}
}

// followKeys moves the selection with the arrows and the page keys, and Tab switches the scope.
func (finder *Finder) followKeys(keys pane.Keys) {
	switch {
	case keys.Typed(input.KeyDown):
		finder.selected++
	case keys.Typed(input.KeyUp):
		finder.selected--
	case keys.Typed(input.KeyPageDown):
		finder.selected += finderRows
	case keys.Typed(input.KeyPageUp):
		finder.selected -= finderRows
	case keys.Pressed(input.KeyTab) && finder.Scope == NamesScope:
		finder.setScope(ContentScope)
	case keys.Pressed(input.KeyTab):
		finder.setScope(NamesScope)
	}
}

// buildPanel adds what the finder's panel holds, and opens what was chosen in it.
func (finder *Finder) buildPanel(context *ui.Context, window Window) {
	submitted := finder.buildQuery(context, window)
	finder.refresh(window)
	count := finder.count()
	finder.selected = min(max(finder.selected, 0), max(count-1, 0))
	finder.list.RowHeight = finder.rowHeight()
	finder.list.Count = count
	finder.list.Reveal(finder.selected)
	onBar := finder.list.Follow(context, window.Keys(), context.Theme.ScrollStep)
	x, y := context.Pointer()
	if index := finder.list.RowAt(x, y); index >= 0 && !onBar && window.Keys().Clicked() {
		finder.selected = index
		submitted = true
	}
	rows := min(max(count, 1), finderRows)
	if finder.Scope != NamesScope {
		rows = min(max(count, 1), contentRows)
	}
	height := float32(rows) * finder.rowHeight()
	context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(height)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		finder.list.Place(area)
		finder.paint(context, window, drawing)
	})
	finder.buildFooter(context)
	if submitted {
		finder.choose(window)
	}
}

// buildQuery is the field typed in, as a pill like the window's search field, with the switch
// between names and content on its right. It tells whether Enter was pressed in it.
func (finder *Finder) buildQuery(context *ui.Context, window Window) bool {
	palette := context.Theme
	look := window.Look()
	submitted := false
	pill := ui.Look{Fill: look.Hover.WithAlpha(0.7), Radius: 20, Border: palette.Primary.WithAlpha(0.5), BorderWidth: 1}
	context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(40), Padding: ui.Padding{Left: 14, Right: 5}, Gap: 10, AlignY: ui.Center}, pill, func() {
		icon, placeholder := "search", "Find a file by its name, in any project"
		switch finder.Scope {
		case ContentScope:
			icon, placeholder = "text-search", "Find a text in the files of every project"
		case ReferencesScope:
			icon, placeholder = "list-tree", "Narrow the "+finder.Name+" by file or line"
		}
		context.IconSized(icon, 16, palette.InkSoft)
		if finder.focusNext {
			context.FocusField("query")
			finder.focusNext = false
		}
		submitted = context.EntryField("query", &finder.Query, ui.TextFieldOptions{Placeholder: placeholder, Plain: true, Width: ui.Grow()})
		scope := int(finder.Scope)
		if theme.IconSwitch(context, "finder scope", &scope, []theme.IconChoice{
			{Icon: "file-search", Name: "File names (Tab)"}, {Icon: "text-search", Name: "File content (Tab)"},
		}) {
			finder.setScope(Scope(scope))
		}
	})
	return submitted
}

// buildFooter says what was found and the keys, each in a key cap.
func (finder *Finder) buildFooter(context *ui.Context) {
	palette := context.Theme
	quiet := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(30), Padding: ui.PadXY(8, 0), Gap: 6, AlignY: ui.Center}, func() {
		switch {
		case finder.Scope == ContentScope && finder.searching:
			context.Text("Searching…", quiet)
		case finder.Scope == ContentScope && utf8.RuneCountInString(strings.TrimSpace(finder.Query)) < contentMinLength:
			context.Text("Type two letters or more; a capital makes it match case", quiet)
		case finder.Scope == ReferencesScope:
			context.Text(fmt.Sprintf("%d of %d %s", len(finder.Hits), len(finder.references), finder.Name), quiet)
		case finder.Scope == ContentScope:
			words := fmt.Sprintf("%d lines", len(finder.Hits))
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

func (finder *Finder) paint(context *ui.Context, window Window, drawing *draw2d.List) {
	palette := context.Theme
	area := finder.list.Area
	drawing.PushClip(area)
	if finder.count() == 0 {
		message := "No file matches"
		switch finder.Scope {
		case ReferencesScope:
			message = "No use left"
		case ContentScope:
			message = "No line holds it"
			if finder.searching || utf8.RuneCountInString(strings.TrimSpace(finder.Query)) < contentMinLength {
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
			drawing.Rectangle(rowArea, 8, window.Look().Chosen, draw2d.Color{}, 0)
		case hovered:
			drawing.Rectangle(rowArea, 8, window.Look().Hover, draw2d.Color{}, 0)
		}
		if finder.Scope != NamesScope {
			finder.paintHit(context, window, drawing, finder.Hits[index], rowArea)
		} else {
			finder.paintName(context, window, drawing, finder.matches[index], rowArea)
		}
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
	icon, iconInk := theme.FileIcon(candidate.Path, window.Look())
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
func (finder *Finder) paintHit(context *ui.Context, window Window, drawing *draw2d.List, hit ContentHit, row draw2d.Rectangle) {
	palette := context.Theme
	measurer := context.Measurer()
	const codeSize = 13
	code := measurer.MeasureText("M", codeSize, draw2d.Mono, 1)
	small := measurer.MeasureText("Ag", palette.SmallTextSize, draw2d.Regular, 1)
	icon, iconInk := theme.FileIcon(hit.Path, window.Look())
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
	if hit.length > 0 {
		found = hit.length
	}
	matchX := left + float32(utf8.RuneCountInString(text[start:min(column, len(text))]))*code.Width
	wash := draw2d.Rectangle{X: matchX - 1, Y: row.Y + 6, Width: float32(found)*code.Width + 2, Height: code.Ascent + code.Descent + 4}
	drawing.Rectangle(wash, 3, palette.Primary.WithAlpha(0.3), draw2d.Color{}, 0)
	drawing.Text(shown, left, codeBaseline, codeSize, draw2d.Mono, window.Look().Syntax.RoleColour(highlight.Plain))
	where := fmt.Sprintf("%s/%s:%d", window.Projects()[hit.project].Name, hit.Path, hit.Line)
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
