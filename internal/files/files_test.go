package files

import (
	"testing"

	"github.com/akrck02/code-review/internal/sidebar"
)

func TestTheFilesModeCountsAProjectsChangesAndTintsItsFiles(test *testing.T) {
	files := New()
	project := files.Decorate(sidebar.Row{Kind: sidebar.ProjectRow, Changed: 3})
	if len(project.Words) != 1 || project.Words[0].Text != "3" || project.Progress != nil {
		test.Errorf("a project with 3 changes is decorated %+v, want its count alone", project)
	}
	if file := files.Decorate(sidebar.Row{Kind: sidebar.FileRow}); !file.Tinted || file.Mark != sidebar.NoMark {
		test.Errorf("a file is decorated %+v, want its name tinted and no mark", file)
	}
	if files.ChangesOnly() || files.ShowsChanges() {
		test.Error("the files mode lists only changes or opens a file on its changes")
	}
}
