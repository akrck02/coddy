package window

import (
	"slices"
	"testing"

	"github.com/akrck02/coddy/editor/internal/ui/controls"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

// hasButton tells whether a button of this name is in the window.
func hasButton(window *uitest.Window, name string) bool {
	_, err := window.Element(name)
	return err == nil
}

func TestTheBarNamesTheFileOpenAndElseTheWorkspace(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	if !slices.Contains(window.Texts(), application.title()) {
		test.Errorf("with no file open the bar does not name the workspace, %q: %v", application.title(), window.Texts())
	}
	application.openByLabel("lighthouse/src/lamp.go")
	window.Frames(2)
	if !slices.Contains(window.Texts(), "lamp.go") {
		test.Errorf("the bar does not name the file open: %v", window.Texts())
	}
	if slices.Contains(window.Texts(), application.title()) {
		test.Error("the bar names the workspace while a file is open")
	}
}

func TestTheBarShowsTheButtonsOfTheModeOn(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	base := "Compare against another branch, tag or commit (b)"
	lock := "Read-only: on, files are only read (⌘⇧L)"
	if hasButton(window, base) || !hasButton(window, lock) {
		test.Error("the files mode's bar does not offer the lock alone, without the base")
	}
	press(window, input.Key2, input.KeySuper)
	window.Frames(2)
	if !hasButton(window, base) || hasButton(window, lock) {
		test.Error("the review mode's bar does not offer the base alone, without the lock")
	}
	if !hasButton(window, "Settings (⌘,)") || hasButton(window, "Read the projects again (r)") {
		test.Error("the bar does not offer the settings, or still offers to read the projects again")
	}
}

// Reading every project again is done less often than the rest: it is in the menu, beside
// opening a folder, not a button of its own.
func TestTheMenuReadsTheProjectsAgain(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true, WorkingTree: true})
	window := newWindow(test, application)
	addAFileUnseen(test, application, window)
	if err := window.Click(menuName); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if err := window.Click("Read the projects again r"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if len(application.workspace.Projects[0].Changes) != 4 {
		test.Error("after the menu's item the file added since the start is not among the changes")
	}
}

// The rail and the sidebar reach the top of the window; the bar sits over the viewer alone, and
// its search is the first of its buttons, on the right.
func TestTheBarSitsOverTheViewerAlone(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	search, err := window.Element(searchName)
	if err != nil {
		test.Fatal(err)
	}
	filter, err := window.Element("Filter projects and files (⌥F)")
	if err != nil {
		test.Fatal(err)
	}
	sidebarRight := float32(modeRailWidth + 1 + application.sidebar.Width)
	if search.Rect.X < sidebarRight {
		test.Errorf("the search begins at %v, over the sidebar ending at %v", search.Rect.X, sidebarRight)
	}
	if search.Rect.Y+search.Rect.Height > controls.BarHeight || filter.Rect.Y > controls.BarHeight+20 {
		test.Errorf("the search is at %v, the sidebar's filter at %v: the sidebar does not begin at the top", search.Rect, filter.Rect)
	}
	refresh, err := window.Element("Settings (⌘,)")
	if err != nil {
		test.Fatal(err)
	}
	width := float32(1200) // newWindow's
	middle := sidebarRight + (width-sidebarRight)/2
	if search.Rect.X < middle || search.Rect.X > refresh.Rect.X {
		test.Errorf("the search is at %v, not on the right before the other buttons (from %v)", search.Rect.X, refresh.Rect.X)
	}
}

// The search is a button like the mode's: with a file open, it stands with them, after the
// file's own buttons and the line that ends them, not apart before the file's.
func TestWithAFileOpenTheSearchStandsBesideTheModesButtons(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	application.openByLabel("lighthouse/src/lamp.go")
	window.Frames(2)
	search, err := window.Element(searchName)
	if err != nil {
		test.Fatal(err)
	}
	lock, err := window.Element("Read-only: on, files are only read (⌘⇧L)")
	if err != nil {
		test.Fatal(err)
	}
	if gap := lock.Rect.X - search.Rect.Right(); gap < 0 || gap > 8 {
		test.Errorf("the search is not just before the lock: search %v, lock %v", search.Rect, lock.Rect)
	}
}
