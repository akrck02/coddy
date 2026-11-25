package window

import (
	"slices"
	"testing"

	"github.com/akrck02/go-renderer/draw2d"
)

// Hiding git's status hides it in the file open too, not only in the sidebar: its status, the
// lines it added and removed, and the marks in its gutter and along its scroll bar.
func TestHidingGitsStatusHidesItInTheFileOpen(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true,
		Open: "lighthouse/src/lamp.go", HidesStatus: true})
	window := newWindow(test, application)
	window.Frames(2)
	if slices.Contains(window.Texts(), "Modified") {
		test.Error("with the status hidden the file's title still says it was modified")
	}
	if marks := colouredRectangles(window.DrawList(), application.Look().Modified); marks > 0 {
		test.Errorf("with the status hidden %d marks of changed lines are painted", marks)
	}
	application.sidebar.HidesStatus = false
	window.Frames(2)
	if !slices.Contains(window.Texts(), "Modified") || colouredRectangles(window.DrawList(), application.Look().Modified) == 0 {
		test.Error("shown again, the status is not in the file's title and gutter")
	}
}

// colouredRectangles counts the rectangles of the last frame filled in a colour.
func colouredRectangles(list *draw2d.List, colour draw2d.Color) int {
	count := 0
	for _, command := range list.Commands {
		if command.Kind == draw2d.RectangleKind && command.Fill == colour {
			count++
		}
	}
	return count
}
