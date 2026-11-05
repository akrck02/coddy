package viewer

import (
	"testing"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/workspace"
)

func lines(count int) []highlight.Line {
	result := make([]highlight.Line, count)
	for index := range result {
		result[index] = highlight.Line{Spans: []highlight.Span{{Text: "beam"}}, Columns: 4}
	}
	return result
}

func TestAnUnchangedFileIsOnlyShownWhole(test *testing.T) {
	view := New()
	file := workspace.FileReference{Project: 0, Path: "lighthouse/lamp.txt"}
	view.Start(file, &workspace.Project{Name: "lighthouse"})
	view.Show(&workspace.Document{Path: file.Path, NewLines: lines(4)}, true)
	view.SetMode(DiffView)
	if view.Mode != FileView || view.RowCount() != 4 {
		test.Errorf("an unchanged file shows in mode %d with %d rows, want the whole file of 4 lines", view.Mode, view.RowCount())
	}
}

func TestAChangedFileOpensOnItsChangesWhenReviewing(test *testing.T) {
	view := New()
	file := workspace.FileReference{Project: 0, Path: "lighthouse/lamp.txt"}
	view.Start(file, &workspace.Project{Name: "lighthouse"})
	view.Show(&workspace.Document{
		Path: file.Path, Changed: true, Change: git.Change{Path: file.Path, Status: git.Modified},
		OldLines: lines(1), NewLines: lines(1),
		Diff: []git.DiffLine{{Kind: git.Removal, OldNumber: 1}, {Kind: git.Insertion, NewNumber: 1}},
	}, true)
	if view.Mode != DiffView || view.Added != 1 || view.Gone != 1 {
		test.Errorf("a changed file shows in mode %d with +%d −%d, want its diff with +1 −1", view.Mode, view.Added, view.Gone)
	}
}
