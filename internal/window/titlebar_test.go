package window

import (
	"slices"
	"testing"

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
	press(window, input.Key2)
	window.Frames(2)
	if !hasButton(window, base) || hasButton(window, lock) {
		test.Error("the review mode's bar does not offer the base alone, without the lock")
	}
	if !hasButton(window, "Read the projects again (r)") {
		test.Error("the review mode's bar does not offer to read the projects again")
	}
}
