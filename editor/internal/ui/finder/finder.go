// Package finder finds a file by its name or by a text it holds, in any project, and opens it
// at the line found.
package finder

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/ui/controls"
	"github.com/akrck02/coddy/editor/internal/ui/pane"
	"github.com/akrck02/coddy/editor/internal/ui/theme"
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
	Projects() []*codebase.Project
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
	OpenAt(file codebase.FileReference, line int)
}

// Finder is the finder: type part of a file's name, or a text it holds, from any project,
// and open it with Enter, at the line found.
type Finder struct {
	Open      bool
	focusNext bool
	Scope     Scope
	from      source // the source of the scope, set with it
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
	// Home is the project listed first among uses, the one of the name asked; Note says how the
	// list is still being made ("checking 40 of 120 places in 6 projects")
	Home int
	Note string
	// Project is the only project the content search reads, -1 for every one; projectName is
	// its name, for the field
	Project     int
	projectName string
}

// ContentHit is a line found by the content search.
type ContentHit struct {
	project int
	changed bool
	status  git.Status
	git.Hit
	length int // how many characters were found, for a use of a name; 0 is the query's length
	// where names the file and the line, and text is the line with its tabs as spaces: made with
	// the hit, as its row is painted every frame
	where, text string
}

// newContentHit is a hit of a project of the window, with the words its row shows.
func newContentHit(window Window, project int, change git.Change, changed bool, hit git.Hit, length int) ContentHit {
	return ContentHit{project: project, changed: changed, status: change.Status, Hit: hit, length: length,
		where: fmt.Sprintf("%s/%s:%d", window.Projects()[project].Name, hit.Path, hit.Line),
		text:  strings.ReplaceAll(hit.Text, "\t", "    ")}
}

// Project is the place of the hit's project in the window.
func (hit ContentHit) Project() int { return hit.project }

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

// Show opens the finder on a scope, with nothing typed, searching every project.
func (finder *Finder) Show(scope Scope) {
	finder.ShowIn(scope, -1, "")
}

// ShowIn opens the finder on a scope, with nothing typed; its content search reads only the
// project at a place in the window, named name, or every project for -1.
func (finder *Finder) ShowIn(scope Scope, project int, name string) {
	finder.Project, finder.projectName = project, name
	finder.Open = true
	finder.focusNext = true
	finder.Scope, finder.from = scope, sources[scope]
	finder.Query = ""
	finder.Stale = true
	finder.selected = 0
	finder.list.Offset = 0
	finder.Hits, finder.searching = nil, false
}

func (finder *Finder) setScope(scope Scope) {
	finder.Scope, finder.from = scope, sources[scope]
	finder.Stale = true
	finder.focusNext = true
}

func (finder *Finder) count() int { return finder.from.count(finder) }

func (finder *Finder) refresh(window Window) {
	if !finder.Stale && finder.asked == finder.Query {
		return
	}
	finder.asked = finder.Query
	finder.Stale = false
	finder.selected = 0
	finder.list.Offset = 0
	finder.from.find(finder, window)
}

func (finder *Finder) choose(window Window) {
	if finder.selected < 0 || finder.selected >= finder.count() {
		return
	}
	reference, line := finder.from.chosen(finder, finder.selected)
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
	case keys.Pressed(input.KeyTab):
		finder.setScope(finder.from.tabbed())
	}
}

// buildPanel adds what the finder's panel holds, and opens what was chosen in it.
func (finder *Finder) buildPanel(context *ui.Context, window Window) {
	submitted := finder.buildQuery(context, window)
	finder.refresh(window)
	count := finder.count()
	finder.selected = pane.Within(finder.selected, count)
	finder.list.RowHeight = finder.from.rowHeight()
	finder.list.Count = count
	finder.list.Reveal(finder.selected)
	onBar := finder.list.Follow(context, window.Keys(), context.Theme.ScrollStep)
	x, y := context.Pointer()
	if index := finder.list.RowAt(x, y); index >= 0 && !onBar && window.Keys().Clicked() {
		finder.selected = index
		submitted = true
	}
	rows := min(max(count, 1), finder.from.mostRows())
	height := float32(rows) * finder.from.rowHeight()
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
		icon, placeholder := finder.from.field(finder)
		context.IconSized(icon, 16, palette.InkSoft)
		if finder.focusNext {
			context.FocusField("query")
			finder.focusNext = false
		}
		submitted = context.EntryField("query", &finder.Query, ui.TextFieldOptions{Placeholder: placeholder, Plain: true, Width: ui.Grow()})
		scope := int(finder.Scope)
		if controls.IconSwitch(context, "finder scope", &scope, []controls.IconChoice{
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
		words, note := finder.from.summary(finder)
		context.Text(words, quiet)
		if note != "" {
			context.Text("· "+note, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Primary})
		}
		context.Spacer()
		for _, hint := range [][2]string{{"↑↓", "choose"}, {"Enter", "open"}, {"Tab", "names or content"}, {"Esc", "close"}} {
			controls.KeyCaps(context, hint[0])
			context.Text(hint[1], quiet)
			context.Box(ui.Layout{Width: ui.Fixed(4)}, nil)
		}
	})
}
