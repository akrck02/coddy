package window

import (
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/text"
)

// An idle frame's allocations, at most, in each mode. Most of them are not the reader's: the test
// window reads every frame for its screen reader, and Go UI keeps each box's content as a closure.
// What only changes with data (labels, counts, rows' decorations, breadcrumbs, the commands) is
// made when it changes, so these stay put however big the file and the workspace are.
const (
	idleFilesFrameAllocations  = 40
	idleReviewFrameAllocations = 60
)

// idleFrameAllocations is how many allocations a frame makes when nothing happens.
func idleFrameAllocations(frames func(int)) float64 {
	frames(3) // what changed settles first
	return testing.AllocsPerRun(50, func() { frames(1) })
}

func TestAnIdleFrameMakesNothingThatOnlyChangesWithData(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, WorkingTree: true,
		Open: "lighthouse/src/lamp.go"})
	window := newWindow(test, application)
	if allocations := idleFrameAllocations(window.Frames); allocations > idleFilesFrameAllocations {
		test.Errorf("an idle frame of the files mode allocates %v times, more than %d", allocations, idleFilesFrameAllocations)
	}
	application.setMode(application.modes[1])
	if allocations := idleFrameAllocations(window.Frames); allocations > idleReviewFrameAllocations {
		test.Errorf("an idle frame of the review mode allocates %v times, more than %d", allocations, idleReviewFrameAllocations)
	}
}

func TestAnIdleFrameCostsTheSameForAFileOfAnySize(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, WorkingTree: true,
		Open: "lighthouse/src/lamp.go"})
	window := newWindow(test, application)
	small := idleFrameAllocations(window.Frames)
	application.viewer.Apply(application, func(buffer *text.Buffer) {
		buffer.Insert(strings.Repeat("// a line of a long file\n", 3000))
	})
	if big := idleFrameAllocations(window.Frames); big > small {
		test.Errorf("an idle frame with a file of 3,000 lines allocates %v times, with one of 5 lines %v", big, small)
	}
}
