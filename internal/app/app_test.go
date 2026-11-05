package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

// makeRepository makes a repository with a main branch and a development on top of it: one file
// changed, one added, one deleted, and one left alone.
func makeRepository(test *testing.T) string {
	test.Helper()
	folder := test.TempDir()
	repository := filepath.Join(folder, "lighthouse")
	if err := os.Mkdir(repository, 0o755); err != nil {
		test.Fatal(err)
	}
	gitIn := func(arguments ...string) {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	write := func(path, text string) {
		full := filepath.Join(repository, path)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	gitIn("init", "-q", "-b", "main")
	write("src/lamp.go", "package lamp\n\nfunc On() bool {\n\treturn false\n}\n")
	write("src/keeper.go", "package lamp\n\n// Keeper keeps the lamp.\ntype Keeper struct{}\n")
	write("notes.txt", "old notes\n")
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "start")
	gitIn("checkout", "-q", "-b", "feature/brighter")
	write("src/lamp.go", "package lamp\n\nfunc On() bool {\n\treturn true\n}\n")
	write("src/beam.go", "package lamp\n\nconst Beam = 12\n")
	gitIn("rm", "-q", "notes.txt")
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "brighter")
	return folder
}

func newWindow(test *testing.T, application *App) *uitest.Window {
	test.Helper()
	window, err := uitest.New(application.Build, uitest.Driver(application.Driver()), uitest.Size(1200, 800))
	if err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	return window
}

func TestAWorkspaceListsTheChangedFilesOfItsRepositories(test *testing.T) {
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	newWindow(test, application)
	if len(application.projects) != 1 {
		test.Fatalf("found %d projects, want 1", len(application.projects))
	}
	project := application.projects[0]
	if project.Base != "main" || len(project.Changes) != 3 {
		test.Fatalf("base %q with %d changes, want main with 3 (%s)", project.Base, len(project.Changes), project.Error)
	}
	var labels []string
	for _, row := range application.sidebar.rows {
		labels = append(labels, row.label)
	}
	shown := strings.Join(labels, ",")
	for _, wanted := range []string{"lighthouse", "beam.go", "lamp.go", "notes.txt"} {
		if !strings.Contains(shown, wanted) {
			test.Errorf("the sidebar shows %s, without %s", shown, wanted)
		}
	}
	if strings.Contains(shown, "keeper.go") {
		test.Errorf("the sidebar lists an unchanged file in the changes mode: %s", shown)
	}
}

func TestNOpensTheChangedFilesOneAfterTheOther(test *testing.T) {
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	var opened []string
	for range 3 {
		window.PressKey(input.KeyN)
		window.Frames(2)
		document := application.editor.document
		if document == nil || !document.Changed {
			test.Fatalf("n opened no changed file")
		}
		opened = append(opened, document.Path)
	}
	if got := strings.Join(opened, ","); got != "src/beam.go,src/lamp.go,notes.txt" {
		test.Errorf("n opened %s", got)
	}
	added, removed := application.editor.added, application.editor.gone
	if added != 0 || removed != 1 {
		test.Errorf("the deleted file shows +%d −%d, want +0 −1", added, removed)
	}
}

func TestTheDiffOfAChangedFileMarksItsLines(test *testing.T) {
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go"})
	newWindow(test, application)
	if application.editor.mode != diffView {
		test.Fatalf("a changed file opened in the file view")
	}
	if application.editor.added != 1 || application.editor.gone != 1 {
		test.Errorf("lamp.go shows +%d −%d, want +1 −1", application.editor.added, application.editor.gone)
	}
	if len(application.editor.hunks) != 1 {
		test.Errorf("found %d runs of changes, want 1", len(application.editor.hunks))
	}
	application.toggleView()
	if application.editor.mode != fileView || application.editor.rowCount() != 5 {
		test.Errorf("the file view shows %d lines, want 5", application.editor.rowCount())
	}
}

func TestTheFinderOpensAnUnchangedFileByPartOfItsName(test *testing.T) {
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	window.HoldKey(input.KeySuper)
	window.PressKey(input.KeyP)
	window.ReleaseKey(input.KeySuper)
	window.Frames(2)
	if !application.finder.open {
		test.Fatal("⌘P did not open the finder")
	}
	window.Type("kpr")
	window.Frames(2)
	window.PressKey(input.KeyEnter)
	window.Frames(2)
	if application.finder.open {
		test.Fatal("Enter left the finder open")
	}
	document := application.editor.document
	if document == nil || document.Path != "src/keeper.go" || document.Changed {
		test.Fatalf("the finder opened %+v, want the unchanged src/keeper.go", document)
	}
	if application.sidebar.mode != filesMode {
		test.Errorf("the sidebar did not switch to every file to show an unchanged one")
	}
}
