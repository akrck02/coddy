package window

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/ui/sidebar"
	"github.com/akrck02/go-renderer/input"
)

func TestCommandPlusAndMinusZoomTheCodeAndCommandZeroBringsItBack(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, Open: "lighthouse/src/lamp.go", ReadOnly: true})
	window := newWindow(test, application)
	first := application.viewer.CodeSize()
	press(window, input.KeyPlus, input.KeySuper)
	press(window, input.KeyPlus, input.KeySuper)
	if size := application.viewer.CodeSize(); size != first+2 {
		test.Fatalf("after ⌘+ twice the code is %v pixels, want %v", size, first+2)
	}
	press(window, input.KeyMinus, input.KeySuper)
	if size := application.viewer.CodeSize(); size != first+1 {
		test.Fatalf("after ⌘- the code is %v pixels, want %v", size, first+1)
	}
	press(window, input.Key0, input.KeySuper)
	if size := application.viewer.CodeSize(); size != first {
		test.Errorf("after ⌘0 the code is %v pixels, want its first %v", size, first)
	}
}

func TestTheFilesModeLeavesTheDeletedFilesOutAndTheReviewKeepsThem(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	listed := func() bool {
		for _, row := range application.sidebar.Rows {
			if row.Kind == sidebar.FileRow && row.Label == "notes.txt" {
				return true
			}
		}
		return false
	}
	window.Frames(2)
	if listed() {
		test.Error("the files mode lists notes.txt, which the development deleted")
	}
	press(window, input.Key2, input.KeySuper)
	window.Frames(2)
	if !listed() {
		test.Error("the review mode leaves out notes.txt, which the development deleted")
	}
}
