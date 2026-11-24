package viewer

import (
	"testing"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
)

// presentedLines is how long the file of these tests is; changedLine is the one replaced.
const (
	presentedLines = 20
	changedLine    = 10
)

// changedFile is a file of 20 lines whose tenth was replaced.
func changedFile() *workspace.Document {
	var diff []git.DiffLine
	for number := 1; number <= presentedLines; number++ {
		if number == changedLine {
			diff = append(diff, git.DiffLine{Kind: git.Removal, OldNumber: number}, git.DiffLine{Kind: git.Insertion, NewNumber: number})
			continue
		}
		diff = append(diff, git.DiffLine{Kind: git.Context, OldNumber: number, NewNumber: number})
	}
	return &workspace.Document{Path: "lighthouse/lamp.txt", Changed: true, Change: git.Change{Path: "lighthouse/lamp.txt", Status: git.Modified},
		OldLines: lines(presentedLines), NewLines: lines(presentedLines), Diff: diff}
}

// presenting opens the changed file in a view of 200 by 100 pixels, on its changes when asked.
func presenting(showChanges bool, uses map[int][]workspace.Span) *Viewer {
	view := New()
	view.List.Place(draw2d.Rectangle{Width: 200, Height: 100})
	view.Uses = uses
	view.Start(workspace.FileReference{Path: "lighthouse/lamp.txt"}, &workspace.Project{Name: "lighthouse"})
	view.Show(changedFile(), showChanges)
	return &view
}

func TestTheChangesShowEveryLineUntilFolded(test *testing.T) {
	view := presenting(true, nil)
	if want := presentedLines + 1; view.Mode != DiffView || view.RowCount() != want || !view.Unfolded() {
		test.Fatalf("the changes show in mode %d with %d rows, want the diff unfolded, every line and the removed one: %d", view.Mode, view.RowCount(), want)
	}
	view.ToggleContext()
	// three lines of context each side, the two sides of the change, and a fold above and below
	if want := 3 + 2 + 3 + 2; view.RowCount() != want {
		test.Errorf("folded, the changes show %d rows, want %d", view.RowCount(), want)
	}
	if !view.rows[0].IsFold() || len(view.Hunks) != 1 || view.Hunks[0] != 4 {
		test.Errorf("the changes start with a fold and one run at row 4, got fold %v and runs %v", view.rows[0].IsFold(), view.Hunks)
	}
}

func TestInTheChangesAFoldedLineIsOpenedToBeShown(test *testing.T) {
	view := presenting(true, nil)
	view.ToggleContext()
	view.RevealLine(1)
	if view.rows[0].IsFold() || view.MarkedLine != 1 {
		test.Errorf("revealing line 1 leaves the fold above the change closed")
	}
	if line := view.fileLineOfRow(0); line != 0 {
		test.Errorf("the first row shows line %d of the file, want 0", line)
	}
}

func TestInTheChangesARemovedLineIsNoPlaceOfTheFile(test *testing.T) {
	view := presenting(true, nil)
	view.ToggleContext()
	removed := view.Hunks[0]
	if view.rows[removed].Line.Kind != git.Removal || view.fileLineOfRow(removed) != -1 {
		test.Errorf("the removed line's row points at line %d of the file, want none", view.fileLineOfRow(removed))
	}
	if line := view.fileLineOfRow(removed + 1); line != changedLine-1 {
		test.Errorf("the added line's row points at line %d of the file, want %d", line, changedLine-1)
	}
}

func TestTheWholeFileShowsEveryLineAndStepsToItsMarkedRuns(test *testing.T) {
	view := presenting(false, nil)
	if view.Mode != FileView || view.RowCount() != presentedLines || view.showsRows() {
		test.Fatalf("the whole file shows in mode %d with %d rows, want its %d lines", view.Mode, view.RowCount(), presentedLines)
	}
	if len(view.Hunks) != 1 || view.Hunks[0] != changedLine-1 || view.Marks[changedLine] != workspace.ModifiedMark {
		test.Errorf("the whole file's runs are %v and line 10 is marked %v, want one run at row 9, modified", view.Hunks, view.Marks[changedLine])
	}
	view.StepHunk(1)
	if first := view.List.RowOf(view.List.Area.Y); first <= 0 {
		test.Errorf("stepping to the next run leaves row %d at the top, want the view scrolled down", first)
	}
	if line := view.fileLineOfRow(7); line != 7 {
		test.Errorf("row 7 of the whole file shows line %d, want 7", line)
	}
}

func TestTheUsesFoldTheLinesWithoutUsesAndLeaveWithThem(test *testing.T) {
	uses := map[int][]workspace.Span{changedLine: {{Start: 0, End: 4}}}
	view := presenting(true, uses)
	// three lines each side of the use, the use, and a fold above and below
	if want := 3 + 1 + 3 + 2; view.Mode != UsesView || view.RowCount() != want {
		test.Fatalf("the uses show in mode %d with %d rows, want %d", view.Mode, view.RowCount(), want)
	}
	if len(view.Hunks) != 1 || view.fileLineOfRow(view.Hunks[0]) != changedLine-1 {
		test.Errorf("the uses' runs are %v, want one at the line used", view.Hunks)
	}
	view.SetUses(nil)
	if view.Mode != FileView || view.RowCount() != presentedLines {
		test.Errorf("without uses the view stays in mode %d with %d rows, want the whole file", view.Mode, view.RowCount())
	}
}

// The scroll bar's marks are worked out when the view is laid out, one per run of rows, so a frame
// paints as many marks as there are runs, not as there are lines.
func TestTheScrollBarMarksEachRunOfChangesOnce(test *testing.T) {
	cases := []struct {
		name    string
		changes bool
		rows    int
		want    []barMark
	}{
		{"the changes", true, presentedLines + 1, []barMark{{first: 9, count: 1, kind: deletedBarMark}, {first: 10, count: 1, kind: createdBarMark}}},
		{"the whole file", false, presentedLines, []barMark{{first: 9, count: 1, kind: modifiedBarMark}}},
	}
	for _, each := range cases {
		marks := presenting(each.changes, nil).scrollMarks
		if marks.rows != each.rows || len(marks.runs) != len(each.want) {
			test.Errorf("%s: %d runs out of %d rows, want %v out of %d", each.name, len(marks.runs), marks.rows, each.want, each.rows)
			continue
		}
		for index, run := range marks.runs {
			if run != each.want[index] {
				test.Errorf("%s: run %d is %+v, want %+v", each.name, index, run, each.want[index])
			}
		}
	}
}

func TestTouchingMarksOfAKindAreOneRun(test *testing.T) {
	var marks barMarks
	for _, row := range []int{3, 4, 5, 8} {
		marks.add(row, createdBarMark)
	}
	marks.add(9, deletedBarMark)
	want := []barMark{{3, 3, createdBarMark}, {8, 1, createdBarMark}, {9, 1, deletedBarMark}}
	if len(marks.runs) != len(want) || marks.runs[0] != want[0] || marks.runs[1] != want[1] || marks.runs[2] != want[2] {
		test.Errorf("the marks are %+v, want %+v", marks.runs, want)
	}
}

func TestTheViewSwitchesOnlyToAPresentationWithSomethingToShow(test *testing.T) {
	view := presenting(true, nil)
	view.SetMode(UsesView)
	if view.Mode != DiffView {
		test.Errorf("a file without uses switched to its uses")
	}
	view.SetMode(FileView)
	if view.Mode != FileView || view.RowCount() != presentedLines {
		test.Errorf("switched to the whole file, the view is in mode %d with %d rows", view.Mode, view.RowCount())
	}
}
