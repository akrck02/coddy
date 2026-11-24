package window

import (
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/text"
)

// An idle frame's allocations, at most, in each mode. Most of them are not the reader's: the test
// window reads every frame for its screen reader, and Go UI keeps each box's content as a closure.
// What only changes with data (labels, counts, rows' decorations, breadcrumbs, the commands) is
// made when it changes, so these stay put however big the file and the workspace are. The uses
// mode runs a language server, whose state the status bar's button reads each frame.
// TODO(akrck02): servers.Fleet.Views makes about 9 of the uses mode's 61 each frame; keep the views
// until a server's state or progress changes.
const (
	idleFilesFrameAllocations  = 40
	idleReviewFrameAllocations = 60
	idleUsesFrameAllocations   = 65
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

func TestAnIdleFrameOfTheUsesModeMakesNothingThatOnlyChangesWithData(test *testing.T) {
	application, window := readerOfUses(test)
	if application.uses.Last() == nil {
		test.Fatal("no search of uses was shown")
	}
	allocations := idleFrameAllocations(window.Frames)
	test.Logf("an idle frame of the uses mode allocates %v times", allocations)
	if allocations > idleUsesFrameAllocations {
		test.Errorf("an idle frame of the uses mode allocates %v times, more than %d", allocations, idleUsesFrameAllocations)
	}
}
