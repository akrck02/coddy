package sidebar

import (
	"fmt"
	"testing"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/theme"
)

// testWindow is a window with its projects and nothing else: it remembers what it was asked to
// show, as the real one would show it.
type testWindow struct {
	projects []*codebase.Project
	opened   []codebase.FileReference
	explored []string
}

func (window *testWindow) Look() theme.Look                     { return theme.Look{} }
func (window *testWindow) Keys() pane.Keys                      { return pane.Keys{} }
func (window *testWindow) ModalOpen() bool                      { return false }
func (window *testWindow) Projects() []*codebase.Project        { return window.projects }
func (window *testWindow) Hidden(*codebase.Project) bool        { return false }
func (window *testWindow) ShowEveryProject()                    {}
func (window *testWindow) ToggleChangedFilter()                 {}
func (window *testWindow) IsOpened(codebase.FileReference) bool { return false }
func (window *testWindow) Unsaved(codebase.FileReference) bool  { return false }
func (window *testWindow) FocusViewer()                         {}
func (window *testWindow) FocusSidebar()                        {}
func (window *testWindow) SidebarFocused() bool                 { return true }
func (window *testWindow) ListEveryFile()                       {}
func (window *testWindow) Loading() bool                        { return false }

func (window *testWindow) ProjectIndex(path string) int {
	for index, project := range window.projects {
		if project.Path == path {
			return index
		}
	}
	return -1
}

func (window *testWindow) OpenFile(file codebase.FileReference) {
	window.opened = append(window.opened, file)
}

func (window *testWindow) ExploreFolder(project int, folder string) {
	window.explored = append(window.explored, fmt.Sprint(project, ":", folder))
}

func (window *testWindow) OpenedFile() (codebase.FileReference, bool) {
	if len(window.opened) == 0 {
		return codebase.FileReference{}, false
	}
	return window.opened[len(window.opened)-1], true
}

// everyFile lists every file, as the files mode does; headed gives it words beside its title.
type everyFile struct{ header string }

func (everyFile) Name() string            { return "Files" }
func (everyFile) ChangesOnly() bool       { return false }
func (everyFile) Decorate(Row) Decoration { return Decoration{} }
func (listing everyFile) Header() string  { return listing.header }

// newTestSidebar is a sidebar listing every file of made-up projects, read and without changes.
func newTestSidebar(projects ...*codebase.Project) (*Sidebar, *testWindow) {
	sidebar := New()
	sidebar.SetListing(everyFile{})
	window := &testWindow{projects: projects}
	sidebar.Rebuild(window)
	return &sidebar, window
}

func lighthouse() *codebase.Project {
	return &codebase.Project{Name: "lighthouse", Path: "/projects/lighthouse", Loaded: true,
		Files: []string{"README.md", "docs/guide.md", "src/lamp.go", "src/wick.go"}}
}

func harbour() *codebase.Project {
	return &codebase.Project{Name: "harbour", Path: "/projects/harbour", Loaded: true, Files: []string{"pier.go"}}
}

// labels are the rows' labels, in order.
func labels(rows []Row) []string {
	words := make([]string, len(rows))
	for index, row := range rows {
		words[index] = row.Label
	}
	return words
}

func TestALoneProjectListsItsFoldersBeforeItsFiles(test *testing.T) {
	sidebar, _ := newTestSidebar(lighthouse())
	got := fmt.Sprint(labels(sidebar.Rows))
	if want := "[lighthouse docs src README.md]"; got != want {
		test.Errorf("the rows are %s, want %s", got, want)
	}
}

func TestAFolderOpenedByHandStaysOpenAfterTheRowsAreMadeAgain(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse())
	sidebar.SetExpanded(sidebar.Rows[2], true) // src
	sidebar.Rebuild(window)
	sidebar.Rebuild(window)
	got := fmt.Sprint(labels(sidebar.Rows))
	if want := "[lighthouse docs src lamp.go wick.go README.md]"; got != want {
		test.Errorf("the rows are %s, want %s", got, want)
	}
}

func TestOpeningAFolderShowsWhatItHolds(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse())
	sidebar.activate(window, 1) // docs
	if len(window.explored) != 1 || window.explored[0] != "0:docs" {
		test.Errorf("activating docs explored %v, want [0:docs]", window.explored)
	}
	sidebar.Rebuild(window)
	if !sidebar.Rows[1].Expanded {
		test.Error("activating a closed folder leaves it closed")
	}
}

func TestTheFilterListsOnlyTheFilesWhosePathHoldsEachWord(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse(), harbour())
	sidebar.SetFilter("SRC la")
	sidebar.Rebuild(window)
	got := fmt.Sprint(labels(sidebar.Rows))
	if want := "[lighthouse src lamp.go]"; got != want {
		test.Errorf("filtered by \"SRC la\", the rows are %s, want %s", got, want)
	}
}

func TestAFilterThatNamesAProjectListsItWhole(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse(), harbour())
	sidebar.SetFilter("harb")
	sidebar.Rebuild(window)
	got := fmt.Sprint(labels(sidebar.Rows))
	if want := "[harbour]"; got != want {
		test.Errorf("filtered by \"harb\", the rows are %s, want %s", got, want)
	}
}

func TestTheSelectionStaysOnTheSameRowWhenRowsAppearAboveIt(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse())
	sidebar.Selected = 3 // README.md
	sidebar.SetExpanded(sidebar.Rows[1], true)
	sidebar.Rebuild(window)
	if row, _ := sidebar.SelectedRow(); row.Label != "README.md" {
		test.Errorf("after docs opened, the selection is on %q, want README.md", row.Label)
	}
}

func TestMovingTheSelectionStopsAtTheEndsAndShowsTheFile(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse())
	sidebar.MoveSelection(window, 10)
	if row, _ := sidebar.SelectedRow(); row.Label != "README.md" {
		test.Errorf("moving past the end selects %q, want README.md", row.Label)
	}
	if file, opened := window.OpenedFile(); !opened || file.Path != "README.md" {
		test.Errorf("moving onto README.md opened %+v", file)
	}
	sidebar.MoveSelection(window, -10)
	if sidebar.Selected != 0 {
		test.Errorf("moving past the start selects row %d, want 0", sidebar.Selected)
	}
}

func TestStepsBackCloseAFolderThenGoToItsParent(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse())
	sidebar.SetExpanded(sidebar.Rows[2], true) // src
	sidebar.Rebuild(window)
	sidebar.Selected = 3 // lamp.go
	sidebar.OpenOrClose(window, -1)
	if row, _ := sidebar.SelectedRow(); row.Label != "src" {
		test.Fatalf("a step back from lamp.go selects %q, want src", row.Label)
	}
	sidebar.OpenOrClose(window, -1)
	sidebar.Rebuild(window)
	if sidebar.Rows[2].Expanded {
		test.Error("a step back on an open folder leaves it open")
	}
}

func TestTheHeaderIsTheListingsWordsMadeWithTheRows(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse())
	sidebar.SetListing(everyFile{header: "3 of 7 viewed"})
	sidebar.Rebuild(window)
	if sidebar.header != "3 of 7 viewed" {
		test.Errorf("the header is %q, want the listing's words", sidebar.header)
	}
	sidebar.SetListing(Listing(statusListing{}))
	sidebar.Rebuild(window)
	if sidebar.header != "" {
		test.Errorf("a listing without words has the header %q", sidebar.header)
	}
}

func TestTheTreesKeptPerFilterStayWithinTheirBound(test *testing.T) {
	sidebar, window := newTestSidebar(lighthouse(), harbour())
	typed := ""
	for _, letter := range "lamp of the lighthouse" {
		typed += string(letter)
		sidebar.SetFilter(typed)
		sidebar.Rebuild(window)
	}
	filtered := 0
	for key := range sidebar.trees {
		if key.filter != "" {
			filtered++
		}
	}
	if most := keptFilters * len(window.projects); filtered > most {
		test.Errorf("after typing a filter, %d filtered trees are kept, more than %d", filtered, most)
	}
	sidebar.SetFilter("lamp")
	sidebar.Rebuild(window)
	if got := fmt.Sprint(labels(sidebar.Rows)); got != "[lighthouse src lamp.go]" {
		test.Errorf("after the trees were forgotten, \"lamp\" lists %s", got)
	}
}
