package window

import (
	"testing"

	"github.com/akrck02/go-renderer/input"
)

func TestSFoldsTheSidebarAwayAndTheCodeTakesItsRoom(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, Open: "lighthouse/src/lamp.go", ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	beside := application.viewer.List.Area.Width
	application.focus = sidebarPane
	press(window, input.KeyS)
	window.Frames(2)
	if !application.SidebarFolded() {
		test.Fatal("s left the sidebar in sight")
	}
	if application.focus != viewerPane {
		test.Error("folding the sidebar left the keyboard in it")
	}
	if wider := application.viewer.List.Area.Width; wider <= beside {
		test.Errorf("with the sidebar folded the code is %v pixels wide, as wide as beside it (%v)", wider, beside)
	}
	press(window, input.KeyS)
	if application.SidebarFolded() || application.focus != sidebarPane {
		test.Error("s again did not bring the sidebar back with the keyboard in it")
	}
}

func TestTheRailsButtonFoldsTheSidebarAndBringsItBack(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, Open: "lighthouse/src/lamp.go", ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	if err := window.Click("Hide the sidebar (s)"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if !application.SidebarFolded() {
		test.Fatal("the rail's button left the sidebar in sight")
	}
	if err := window.Click("Show the sidebar (s)"); err != nil {
		test.Fatal(err)
	}
	if application.SidebarFolded() {
		test.Error("the rail's button did not bring the sidebar back")
	}
}

func TestTheKeyboardGoingToTheFoldedSidebarBringsItBack(test *testing.T) {
	application, window := editingAFile(test)
	window.Frames(2)
	press(window, input.KeyE, input.KeySuper, input.KeyShift)
	if !application.SidebarFolded() || !application.editing() {
		test.Fatal("⌘⇧E while editing did not fold the sidebar, or stopped the editing")
	}
	press(window, input.KeyS)
	if !application.SidebarFolded() {
		test.Fatal("s typed in the file brought the sidebar back")
	}
	press(window, input.KeyEscape)
	if application.SidebarFolded() {
		test.Error("Esc gave the keyboard to the sidebar and left it folded away")
	}
}

func TestTheSidebarStartsFoldedWhenItWasLeftSo(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, Open: "lighthouse/src/lamp.go", ReadOnly: true, SidebarFolded: true})
	window := newWindow(test, application)
	window.Frames(2)
	if !application.SidebarFolded() {
		test.Error("the sidebar left folded starts in sight")
	}
}
