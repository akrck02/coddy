package window

import (
	"slices"
	"testing"

	"github.com/akrck02/go-renderer/input"
)

// A folder of the bar's breadcrumb shows what it holds where the file was, and a file chosen there
// is opened in its place.
func TestAFolderOfTheBreadcrumbShowsItsFilesAndOpensOne(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	application.openByLabel("lighthouse/src/lamp.go")
	window.Frames(2)
	if err := window.Click("src"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	for _, name := range []string{"beam.go", "keeper.go", "lamp.go"} {
		if !slices.Contains(window.Texts(), name) {
			test.Errorf("the folder src does not show %s: %v", name, window.Texts())
		}
	}
	if err := window.Click("keeper.go"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if file, open := application.OpenedFile(); !open || file.Path != "src/keeper.go" || application.explorer.Open {
		test.Errorf("choosing keeper.go in the folder did not open it in its place: %v %v", file, open)
	}
}

// A folder chosen in the sidebar, by the keyboard or a click, shows what it holds as the
// breadcrumb's does, as a file chosen there shows the file.
func TestAFolderChosenInTheSidebarShowsItsFiles(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	application.openByLabel("lighthouse/src/lamp.go")
	window.Frames(2)
	selectRow(test, application, "src")
	press(window, input.KeyEnter)
	if !application.explorer.Open || application.explorer.Path != "src" {
		test.Errorf("choosing the folder src shows %q (open %v)", application.explorer.Path, application.explorer.Open)
	}
	selectRow(test, application, "lighthouse")
	press(window, input.KeyDown)
	press(window, input.KeyUp)
	if !application.explorer.Open || application.explorer.Path != "" {
		test.Errorf("moving onto the project does not show its top: %q", application.explorer.Path)
	}
}
