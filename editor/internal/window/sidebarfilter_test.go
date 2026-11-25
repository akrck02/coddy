package window

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/sidebar"
	"github.com/akrck02/go-renderer/input"
)

func TestTheSidebarFilterListsOnlyTheFilesItFinds(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true, Mode: "Review", ReadOnly: true})
	window := newWindow(test, application)
	press(window, input.KeyF, input.KeyAlt)
	typeText(window, "BEAM")
	window.Frames(2)
	var files []string
	for _, row := range application.sidebar.Rows {
		if row.Kind == sidebar.FileRow {
			files = append(files, row.Label)
		}
	}
	if len(files) != 1 || files[0] != "beam.go" {
		test.Fatalf("filtered by beam, the sidebar lists %v", files)
	}
	press(window, input.KeyEnter)
	row, selected := application.sidebar.SelectedRow()
	if !selected || row.Label != "beam.go" || !application.SidebarFocused() {
		test.Errorf("Enter selected %+v, the sidebar focused: %t", row, application.SidebarFocused())
	}
}

func TestAFilterNamingAProjectListsItWhole(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true, Mode: "Review", ReadOnly: true})
	window := newWindow(test, application)
	application.sidebar.SetFilter("lighthouse")
	window.Frames(2)
	files := 0
	for _, row := range application.sidebar.Rows {
		if row.Kind == sidebar.FileRow {
			files++
		}
	}
	if files != 3 {
		test.Errorf("filtered by the project's name, the sidebar lists %d files, want its 3 changed", files)
	}
}

func TestADiffOpensWholeAndItsArrowsGoFromChangeToChange(test *testing.T) {
	folder := makeRepository(test)
	addLongRepository(test, folder)
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, Open: "tides/coast.txt", Mode: "Review", ReadOnly: true})
	window := newWindow(test, application)
	if rows := application.viewer.RowCount(); rows != longFileLines+changedLines {
		test.Fatalf("the diff opens with %d rows, want all %d lines and the %d old ones", rows, longFileLines, changedLines)
	}
	application.viewer.List.Offset = 0
	if err := window.Click("Next change (F7, ])"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if first, _ := application.viewer.List.Visible(); first < 40 {
		test.Errorf("after the next change the first row in sight is %d, want the change at line 70", first)
	}
	if err := window.Click("Previous change (⇧F7, [)"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if first, _ := application.viewer.List.Visible(); first > 2 {
		test.Errorf("after the previous change the first row in sight is %d, want the top", first)
	}
}
