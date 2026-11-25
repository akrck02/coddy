package window

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/akrck02/go-renderer/input"
)

func TestTheSidebarKeyFoldsItAwayAndTheCodeTakesItsRoom(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, Open: "lighthouse/src/lamp.go", ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	beside := application.viewer.List.Area.Width
	application.focus = sidebarPane
	press(window, input.KeyE, input.KeySuper, input.KeyShift)
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
	press(window, input.KeyE, input.KeySuper, input.KeyShift)
	if application.SidebarFolded() || application.focus != sidebarPane {
		test.Error("s again did not bring the sidebar back with the keyboard in it")
	}
}

func TestTheRailsButtonFoldsTheSidebarAndBringsItBack(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, Open: "lighthouse/src/lamp.go", ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	if err := window.Click("Hide the sidebar (⌘⇧E)"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if !application.SidebarFolded() {
		test.Fatal("the rail's button left the sidebar in sight")
	}
	if err := window.Click("Show the sidebar (⌘⇧E)"); err != nil {
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

// ⌘O opens the system's folder chooser in the folder opened last, and opens the folder chosen.
func TestTheFolderChosenInTheSystemsChooserIsOpened(test *testing.T) {
	first, second := makeRepository(test), makeRepository(test)
	chooser := &folderChooser{answer: second}
	application := newReader(Settings{Folders: []string{first}, Synchronous: true, ChooseFolder: chooser.choose})
	window := newWindow(test, application)
	press(window, input.KeyO, input.KeySuper)
	if chooser.opened != 1 || chooser.start != first {
		test.Fatalf("the chooser opened %d times, in %q; want once, in %q", chooser.opened, chooser.start, first)
	}
	if !slices.Contains(application.session.Folders, second) {
		test.Errorf("the folder chosen is not open: %v", application.session.Folders)
	}
}

// Closing the chooser without a folder changes nothing.
func TestClosingTheChooserOpensNothing(test *testing.T) {
	folder := makeRepository(test)
	chooser := &folderChooser{}
	application := newReader(Settings{Folders: []string{folder}, Synchronous: true, ChooseFolder: chooser.choose})
	window := newWindow(test, application)
	press(window, input.KeyO, input.KeySuper)
	if chooser.opened != 1 || len(application.session.Folders) != 1 {
		test.Errorf("the chooser opened %d times; the folders open are %v", chooser.opened, application.session.Folders)
	}
}

// A folder with no repository changes nothing in sight, so Coddy says what it found there.
func TestAFolderOpenedIsTheWholeWorkspace(test *testing.T) {
	folder := makeRepository(test)
	inside := filepath.Join(folder, "lighthouse", "src")
	chooser := &folderChooser{answer: inside}
	application := newReader(Settings{Folders: []string{folder}, Synchronous: true, ChooseFolder: chooser.choose})
	window := newWindow(test, application)
	press(window, input.KeyO, input.KeySuper)
	window.Frame()
	projects := application.session.Projects
	if len(projects) != 1 || projects[0].Name != "src" || !slices.Equal(application.session.Folders, []string{inside}) {
		test.Errorf("the workspace has the folders %v, want only %s", application.session.Folders, inside)
	}
}

func TestAFolderWithNoRepositoryOpensAsAPlainFolder(test *testing.T) {
	folder := makeRepository(test)
	plain := filepath.Join(test.TempDir(), "recipes")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain, "soup.txt"), []byte("water\n"), 0o644); err != nil {
		test.Fatal(err)
	}
	chooser := &folderChooser{answer: plain}
	application := newReader(Settings{Folders: []string{folder}, Synchronous: true, ChooseFolder: chooser.choose})
	window := newWindow(test, application)
	press(window, input.KeyO, input.KeySuper)
	window.Frame()
	names := []string{}
	for _, project := range application.session.Projects {
		names = append(names, project.Name)
	}
	if !slices.Equal(names, []string{"recipes"}) {
		test.Errorf("the projects open are %v, want only recipes", names)
	}
}
