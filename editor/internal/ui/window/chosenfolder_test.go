package window

import (
	"slices"
	"testing"

	"github.com/akrck02/go-renderer/input"
)

// The breadcrumb only tells where a file is: its folders are words, not buttons.
func TestTheBreadcrumbsFoldersAreNotButtons(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	application.openByLabel("lighthouse/src/lamp.go")
	window.Frames(2)
	if !slices.Contains(window.Texts(), "src") {
		test.Fatalf("the breadcrumb does not name the folder src: %v", window.Texts())
	}
	if err := window.Click("src"); err == nil {
		test.Error("the crumb src can be clicked")
	}
}

// A folder chosen in the sidebar, by the keyboard or a click, is listed there already: the file's
// place only asks to choose one of its files, under a search icon.
func TestAFolderChosenInTheSidebarAsksToChooseOneOfItsFiles(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	application.openByLabel("lighthouse/src/lamp.go")
	window.Frames(2)
	selectRow(test, application, "src")
	press(window, input.KeyEnter)
	window.Frames(2)
	if !application.folder.shown || application.folder.path != "src" {
		test.Errorf("choosing the folder src shows %q (shown %v)", application.folder.path, application.folder.shown)
	}
	if !slices.Contains(window.Texts(), chosenFolderHint) || application.folder.crumbs[len(application.folder.crumbs)-1] != "src" {
		test.Errorf("the folder src does not ask to choose a file: %v", window.Texts())
	}
	selectRow(test, application, "lighthouse")
	press(window, input.KeyDown)
	press(window, input.KeyUp)
	window.Frames(2)
	if crumbs := application.folder.crumbs; len(crumbs) != 1 || crumbs[0] != "lighthouse" {
		test.Errorf("moving onto the project does not name it: %v", window.Texts())
	}
	application.openByLabel("lighthouse/src/lamp.go")
	if application.folder.shown {
		test.Error("opening a file leaves the folder in its place")
	}
}
