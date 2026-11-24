package finder

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// source is where the finder looks for what is typed, and how its rows look: one kind per Scope.
// A new kind is a file of its own and a line in sources.
type source interface {
	// field is the icon and the words shown in the empty field typed in.
	field(finder *Finder) (icon, placeholder string)
	rowHeight() float32
	// mostRows is how many rows the list shows before it scrolls.
	mostRows() int
	count(finder *Finder) int
	// find lists what the query finds, at once or as a job of the window.
	find(finder *Finder, window Window)
	// chosen is the file and line a row opens, the line 0 for the file's top.
	chosen(finder *Finder, row int) (workspace.FileReference, int)
	// summary says what was found, and a note in the accent, "" for none.
	summary(finder *Finder) (words, note string)
	// nothingFound is what the empty list says, "" for nothing.
	nothingFound(finder *Finder) string
	paintRow(finder *Finder, context *ui.Context, window Window, drawing *draw2d.List, row int, area draw2d.Rectangle)
	// tabbed is the scope Tab switches to.
	tabbed() Scope
}

// sources are the finder's sources, by their scope.
var sources = [...]source{
	NamesScope:      namesSource{},
	ContentScope:    contentSource{},
	ReferencesScope: referencesSource{},
}

// namesSource finds a file by its name, fuzzy, over every file's project and path.
type namesSource struct{}

func (namesSource) field(*Finder) (string, string) {
	return "search", "Find a file by its name, in any project"
}
func (namesSource) rowHeight() float32          { return finderRowHeight }
func (namesSource) mostRows() int               { return finderRows }
func (namesSource) count(finder *Finder) int    { return len(finder.matches) }
func (namesSource) tabbed() Scope               { return ContentScope }
func (namesSource) nothingFound(*Finder) string { return "No file matches" }

func (namesSource) find(finder *Finder, window Window) {
	finder.matches = FuzzyFind(window.Candidates(), finder.Query, finderLimit)
}

func (namesSource) chosen(finder *Finder, row int) (workspace.FileReference, int) {
	candidate := finder.matches[row].Candidate
	return workspace.FileReference{Project: candidate.Project, Path: candidate.Path}, 0
}

func (namesSource) summary(finder *Finder) (string, string) {
	return fmt.Sprintf("%d files", len(finder.matches)), ""
}

func (namesSource) paintRow(finder *Finder, context *ui.Context, window Window, drawing *draw2d.List, row int, area draw2d.Rectangle) {
	finder.paintName(context, window, drawing, finder.matches[row], area)
}

// hitsSource is what the sources that list lines share: their rows, and the line each opens.
type hitsSource struct{}

func (hitsSource) rowHeight() float32       { return contentRowHeight }
func (hitsSource) mostRows() int            { return contentRows }
func (hitsSource) count(finder *Finder) int { return len(finder.Hits) }
func (hitsSource) tabbed() Scope            { return NamesScope }

func (hitsSource) chosen(finder *Finder, row int) (workspace.FileReference, int) {
	hit := finder.Hits[row]
	return workspace.FileReference{Project: hit.project, Path: hit.Path}, hit.Line
}

func (hitsSource) paintRow(finder *Finder, context *ui.Context, window Window, drawing *draw2d.List, row int, area draw2d.Rectangle) {
	finder.paintHit(context, window, drawing, finder.Hits[row], area)
}

// contentSource finds the lines that hold the text typed, in every file.
type contentSource struct{ hitsSource }

func (contentSource) field(*Finder) (string, string) {
	return "text-search", "Find a text in the files of every project"
}

func (contentSource) find(finder *Finder, window Window) {
	finder.searchContent(window, strings.TrimSpace(finder.Query))
}

func (contentSource) summary(finder *Finder) (string, string) {
	switch {
	case finder.searching:
		return "Searching…", ""
	case tooShort(finder.Query):
		return "Type two letters or more; a capital makes it match case", ""
	case finder.truncated:
		return fmt.Sprintf("the first %d lines", contentHitLimit), ""
	}
	return fmt.Sprintf("%d lines", len(finder.Hits)), ""
}

func (contentSource) nothingFound(finder *Finder) string {
	if finder.searching || tooShort(finder.Query) {
		return ""
	}
	return "No line holds it"
}

// tooShort tells whether a query is too short to search the content for: it would match nearly
// every line.
func tooShort(query string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(query)) < contentMinLength
}

// referencesSource lists the places a language server gave, which typing narrows.
type referencesSource struct{ hitsSource }

func (referencesSource) field(finder *Finder) (string, string) {
	return "list-tree", "Narrow the " + finder.Name + " by file or line"
}

func (referencesSource) find(finder *Finder, window Window) {
	finder.Hits = narrowed(finder.references, finder.Query, window)
}

func (referencesSource) summary(finder *Finder) (string, string) {
	return fmt.Sprintf("%d of %d %s", len(finder.Hits), len(finder.references), finder.Name), finder.Note
}

func (referencesSource) nothingFound(*Finder) string { return "No use left" }
