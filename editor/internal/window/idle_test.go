package window

import (
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/text"
)

// An idle frame's allocations, at most, in each mode. Most of them are not the reader's: the test
// window reads every frame for its screen reader, and Go UI keeps each box's content as a closure.
// What only changes with data (labels, counts, rows' decorations, breadcrumbs, the commands) is
// made when it changes, so these stay put however big the file and the workspace are. The uses
// mode runs a language server, whose state the status bar's button reads each frame: its views are
// kept until the server says something new, so reading them makes nothing.
const (
	idleFilesFrameAllocations  = 40
	idleReviewFrameAllocations = 60
	idleUsesFrameAllocations   = 55
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

// The status bar reads the servers every frame, but what it shows only changes when a server
// says something new: a frame with a server running and saying nothing makes nothing for them,
// and the server's next word still shows.
func TestReadingTheServersRunningCostsNothingUntilOneChanges(test *testing.T) {
	application, window := readerOfUses(test)
	views := application.servers.Views()
	if len(views) == 0 || views[0].Client == nil {
		test.Fatalf("no server runs: %+v", views)
	}
	window.Frames(2)
	if allocations := testing.AllocsPerRun(50, func() { application.servers.Views() }); allocations > 0 {
		test.Errorf("reading the servers running allocates %v times a frame, want none", allocations)
	}
	client := views[0].Client
	before := client.Progress().Total
	if err := client.AddFolder(test.TempDir()); err != nil {
		test.Fatal(err)
	}
	if total := application.servers.Views()[0].Progress.Total; total != before+1 {
		test.Errorf("after a folder was added the server reads %d projects, want %d", total, before+1)
	}
}
