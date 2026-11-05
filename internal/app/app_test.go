package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/review"
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
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true, Review: true})
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
		test.Errorf("the sidebar lists an unchanged file in the review mode: %s", shown)
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
		Open: "lighthouse/src/lamp.go", Review: true})
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
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true, Review: true})
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

// addUntouchedRepository puts a repository with no development beside the others.
func addUntouchedRepository(test *testing.T, folder string) {
	test.Helper()
	repository := filepath.Join(folder, "harbour")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "quay.go"), []byte("package harbour\n"), 0o644); err != nil {
		test.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "-m", "start"}} {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}

func sidebarShows(application *App, label string) bool {
	application.sidebar.rebuild(application)
	for _, row := range application.sidebar.rows {
		if row.label == label {
			return true
		}
	}
	return false
}

func finderFinds(application *App, label string) bool {
	for _, candidate := range application.candidates {
		if strings.HasPrefix(candidate.Label, label+"/") {
			return true
		}
	}
	return false
}

func TestOnlyTheProjectsWithChangesAreShownInBothModesAndTheFinder(test *testing.T) {
	folder := makeRepository(test)
	addUntouchedRepository(test, folder)
	application := New(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, OnlyChanged: true})
	window := newWindow(test, application)
	if len(application.projects) != 2 {
		test.Fatalf("found %d projects, want 2", len(application.projects))
	}
	for _, mode := range []sidebarMode{reviewMode, filesMode} {
		application.sidebar.mode = mode
		if sidebarShows(application, "harbour") || !sidebarShows(application, "lighthouse") {
			test.Errorf("in mode %d the sidebar does not show only the changed project", mode)
		}
	}
	if finderFinds(application, "harbour") || !finderFinds(application, "lighthouse") {
		test.Error("the finder does not find only the changed project's files")
	}
	window.PressKey(input.KeyH)
	window.Frames(2)
	if application.onlyChanged || !sidebarShows(application, "harbour") || !finderFinds(application, "harbour") {
		test.Error("h did not show every project")
	}
}

func TestTheFilesModeShowsAChangedFileWholeWithItsChangesMarked(test *testing.T) {
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go"})
	window := newWindow(test, application)
	if application.sidebar.mode != filesMode {
		test.Fatal("the reader did not start in the files mode")
	}
	editor := &application.editor
	if editor.mode != fileView || editor.rowCount() != 5 {
		test.Fatalf("lamp.go opened in mode %d with %d rows, want the whole file of 5 lines", editor.mode, editor.rowCount())
	}
	if editor.marks[4] != review.ModifiedMark {
		test.Errorf("the changed line 4 is marked %d, want modified", editor.marks[4])
	}
	if len(editor.hunks) != 1 || editor.hunks[0] != 3 {
		test.Errorf("the runs of marked lines are %v, want [3]", editor.hunks)
	}
	window.PressKey(input.Key2)
	window.Frames(2)
	if application.sidebar.mode != reviewMode || editor.mode != diffView {
		test.Errorf("2 did not move to the review mode with the file as its diff")
	}
	if span, found := editor.inline[editor.hunks[0]]; !found || span.Start != 11 {
		test.Errorf("the changed word of the replaced line is %+v (found %v), want it from column 11, after the indent and \"return \"", span, found)
	}
}

func TestMarkingAFileViewedOpensTheNextOneNotViewed(test *testing.T) {
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Review: true})
	window := newWindow(test, application)
	window.PressKey(input.KeyTab)
	window.Frames(1)
	window.PressKey(input.KeyV)
	window.Frames(2)
	if !application.isViewed(fileReference{0, "src/lamp.go"}) {
		test.Fatal("v did not mark lamp.go as viewed")
	}
	if document := application.editor.document; document == nil || document.Path != "notes.txt" {
		test.Fatalf("v did not open the next file not viewed, notes.txt")
	}
	if viewed, total := application.reviewProgress(); viewed != 1 || total != 3 {
		test.Errorf("the progress is %d of %d, want 1 of 3", viewed, total)
	}
}

func TestEnteringAProjectListsOnlyItsTree(test *testing.T) {
	folder := makeRepository(test)
	addUntouchedRepository(test, folder)
	application := New(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	sidebar := &application.sidebar
	sidebar.rebuild(application)
	for index, row := range sidebar.rows {
		if row.kind == projectRow && row.label == "lighthouse" {
			sidebar.selected = index
		}
	}
	window.PressKey(input.KeyEnter)
	window.Frames(2)
	if sidebar.entered == "" {
		test.Fatal("Enter on a project did not enter it")
	}
	inside := application.projectIndex(sidebar.entered)
	if application.projects[inside].Name != "lighthouse" {
		test.Fatalf("entered %s, want lighthouse", sidebar.entered)
	}
	for _, row := range sidebar.rows {
		if row.kind == projectRow || row.project != inside {
			test.Fatalf("inside lighthouse the sidebar still lists %q of project %d", row.label, row.project)
		}
	}
	if !sidebarShows(application, "src") || !sidebarShows(application, "notes.txt") {
		test.Error("the project's own tree is not listed from its top")
	}
	window.PressKey(input.KeyEscape)
	window.Frames(2)
	if sidebar.entered != "" || !sidebarShows(application, "harbour") {
		test.Error("Esc did not go back to every project")
	}
}

func TestTheFilesModeCanListOnlyTheChangedFiles(test *testing.T) {
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	if !sidebarShows(application, "src") {
		test.Fatal("the project is not open in the files mode")
	}
	window.PressKey(input.KeyG)
	window.Frames(2)
	if !sidebarShows(application, "lamp.go") || sidebarShows(application, "keeper.go") {
		test.Error("g did not list only the changed files")
	}
	window.PressKey(input.KeyG)
	window.Frames(2)
	application.sidebar.setExpanded(treeRow{project: 0, path: "src"}, true)
	if !sidebarShows(application, "keeper.go") {
		test.Error("g again did not list every file")
	}
}
