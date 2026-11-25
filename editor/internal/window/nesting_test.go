package window

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/sidebar"
	"github.com/akrck02/coddy/editor/internal/workspace"
)

// folderWithLooseFiles is makeRepository's folder, with a loose file and a folder of no
// repository beside the repository: a folder opened as one tree.
func folderWithLooseFiles(test *testing.T) string {
	test.Helper()
	folder := makeRepository(test)
	for _, path := range []string{"guide.pdf", "recipes/soup.txt"} {
		full := filepath.Join(folder, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			test.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("text\n"), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	return folder
}

// rowsOf are the sidebar's rows as "depth:label", one per row, joined by commas.
func rowsOf(application *Window) string {
	var rows []string
	for _, row := range application.sidebar.Rows {
		rows = append(rows, strings.Repeat(">", row.Depth)+row.Label)
	}
	return strings.Join(rows, ",")
}

func TestAFolderShowsItsRepositoriesInsideItsTree(test *testing.T) {
	folder := folderWithLooseFiles(test)
	application := newReader(Settings{Folders: []string{folder}, Synchronous: true})
	newWindow(test, application)
	rows := rowsOf(application)
	want := filepath.Base(folder) + ",>lighthouse,>recipes,>guide.pdf"
	if rows != want {
		test.Errorf("the sidebar lists %s, want %s", rows, want)
	}
}

func TestARepositoryInsideTheTreeKeepsItsChanges(test *testing.T) {
	folder := folderWithLooseFiles(test)
	application := newReader(Settings{Folders: []string{folder}, Synchronous: true})
	window := newWindow(test, application)
	selectRow(test, application, "lighthouse")
	row := application.sidebar.Rows[application.sidebar.Selected]
	if row.Kind != sidebar.ProjectRow || row.Changed != 3 || application.session.Projects[row.Project].Branch != "feature/brighter" {
		test.Fatalf("the row of lighthouse is %+v, want a project with 3 changes on feature/brighter", row)
	}
	application.sidebar.SetExpanded(row, true)
	application.sidebar.Rebuild(application)
	window.Frame()
	if rows := rowsOf(application); !strings.Contains(rows, ">lighthouse,>>src,") {
		test.Errorf("the open repository lists %s, want its folders one level under it", rows)
	}
}

func TestTheReviewListsTheRepositoriesFlat(test *testing.T) {
	folder := folderWithLooseFiles(test)
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, Mode: "Review"})
	newWindow(test, application)
	rows := rowsOf(application)
	if !strings.HasPrefix(rows, "lighthouse,") || strings.Contains(rows, "guide.pdf") {
		test.Errorf("the review lists %s, want lighthouse at the top and no loose file", rows)
	}
}

func TestAFileOfARepositoryInsideTheTreeIsRevealed(test *testing.T) {
	folder := folderWithLooseFiles(test)
	application := newReader(Settings{Folders: []string{folder}, Synchronous: true})
	window := newWindow(test, application)
	lighthouse := application.ProjectIndex(filepath.Join(folder, "lighthouse"))
	application.sidebar.SetExpanded(application.sidebar.Rows[0], false)
	application.Reveal(workspace.FileReference{Project: lighthouse, Path: "src/lamp.go"})
	window.Frame()
	row, found := application.sidebar.SelectedRow()
	if !found || row.Project != lighthouse || row.Path != "src/lamp.go" {
		test.Errorf("the row selected is %+v, want lamp.go of lighthouse, in %s", row, rowsOf(application))
	}
}
