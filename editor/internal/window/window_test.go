package window

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/files"
	"github.com/akrck02/coddy/editor/internal/finder"
	"github.com/akrck02/coddy/editor/internal/review"
	"github.com/akrck02/coddy/editor/internal/sidebar"
	"github.com/akrck02/coddy/editor/internal/viewer"
	"github.com/akrck02/coddy/editor/internal/workspace"
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

func newWindow(test *testing.T, application *Window) *uitest.Window {
	test.Helper()
	window, err := uitest.New(application.Build, uitest.Driver(application.Driver()), uitest.Size(1200, 800))
	if err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	return window
}

func TestAWorkspaceListsTheChangedFilesOfItsRepositories(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true, Mode: "Review"})
	newWindow(test, application)
	if len(application.session.Projects) != 1 {
		test.Fatalf("found %d projects, want 1", len(application.session.Projects))
	}
	project := application.session.Projects[0]
	if project.Base != "main" || len(project.Changes) != 3 {
		test.Fatalf("base %q with %d changes, want main with 3 (%s)", project.Base, len(project.Changes), project.Error)
	}
	var labels []string
	for _, row := range application.sidebar.Rows {
		labels = append(labels, row.Label)
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

func TestTheNextChangedFileKeyOpensThemOneAfterTheOther(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true, Mode: "Review"})
	window := newWindow(test, application)
	var opened []string
	for range 3 {
		press(window, input.KeyN, input.KeySuper, input.KeyShift)
		window.Frames(2)
		document := application.viewer.Document
		if document == nil || !document.Changed {
			test.Fatalf("n opened no changed file")
		}
		opened = append(opened, document.Path)
	}
	if got := strings.Join(opened, ","); got != "src/beam.go,src/lamp.go,notes.txt" {
		test.Errorf("n opened %s", got)
	}
	added, removed := application.viewer.Added, application.viewer.Gone
	if added != 0 || removed != 1 {
		test.Errorf("the deleted file shows +%d −%d, want +0 −1", added, removed)
	}
}

func TestTheNextChangedFileKeyInTheFilesModeLeavesTheDeletedFilesOut(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	var opened []string
	for range 3 {
		press(window, input.KeyN, input.KeySuper, input.KeyShift)
		window.Frames(2)
		opened = append(opened, application.viewer.Document.Path)
	}
	if got := strings.Join(opened, ","); got != "src/beam.go,src/lamp.go,src/beam.go" {
		test.Errorf("n opened %s", got)
	}
}

func TestTheDiffOfAChangedFileMarksItsLines(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Mode: "Review"})
	newWindow(test, application)
	if application.viewer.Mode != viewer.DiffView {
		test.Fatalf("a changed file opened in the file view")
	}
	if application.viewer.Added != 1 || application.viewer.Gone != 1 {
		test.Errorf("lamp.go shows +%d −%d, want +1 −1", application.viewer.Added, application.viewer.Gone)
	}
	if len(application.viewer.Hunks) != 1 {
		test.Errorf("found %d runs of changes, want 1", len(application.viewer.Hunks))
	}
	application.toggleView()
	// five lines, and the empty one after the last line break, where the caret can go
	if application.viewer.Mode != viewer.FileView || application.viewer.RowCount() != 6 {
		test.Errorf("the file view shows %d lines, want 5 and an empty one", application.viewer.RowCount())
	}
}

func TestTheFinderOpensAnUnchangedFileByPartOfItsName(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true, Mode: "Review"})
	window := newWindow(test, application)
	window.PressKey(input.KeyShift)
	window.PressKey(input.KeyShift)
	window.Frames(2)
	if !application.finder.Open {
		test.Fatal("Shift twice did not open the finder")
	}
	window.Type("kpr")
	window.Frames(2)
	window.PressKey(input.KeyEnter)
	window.Frames(2)
	if application.finder.Open {
		test.Fatal("Enter left the finder open")
	}
	document := application.viewer.Document
	if document == nil || document.Path != "src/keeper.go" || document.Changed {
		test.Fatalf("the finder opened %+v, want the unchanged src/keeper.go", document)
	}
	if application.mode.Name() != "Files" {
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

func sidebarShows(application *Window, label string) bool {
	application.sidebar.Rebuild(application)
	for _, row := range application.sidebar.Rows {
		if row.Label == label {
			return true
		}
	}
	return false
}

func finderFinds(application *Window, label string) bool {
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
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, OnlyChanged: true})
	window := newWindow(test, application)
	if len(application.session.Projects) != 2 {
		test.Fatalf("found %d projects, want 2", len(application.session.Projects))
	}
	for _, each := range application.modes {
		application.setMode(each)
		if sidebarShows(application, "harbour") || !sidebarShows(application, "lighthouse") {
			test.Errorf("in the %s mode the sidebar does not show only the changed project", each.Name())
		}
	}
	if finderFinds(application, "harbour") || !finderFinds(application, "lighthouse") {
		test.Error("the finder does not find only the changed project's files")
	}
	press(window, input.KeyG, input.KeySuper, input.KeyShift)
	window.Frames(2)
	if application.onlyChanged || !sidebarShows(application, "harbour") || !finderFinds(application, "harbour") {
		test.Error("h did not show every project")
	}
}

func TestTheFilesModeShowsAChangedFileWholeWithItsChangesMarked(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go"})
	window := newWindow(test, application)
	if application.mode.Name() != "Files" {
		test.Fatal("the reader did not start in the files mode")
	}
	view := &application.viewer
	if view.Mode != viewer.FileView || view.RowCount() != 6 {
		test.Fatalf("lamp.go opened in mode %d with %d rows, want the whole file of 5 lines and an empty one", view.Mode, view.RowCount())
	}
	if view.Marks[4] != workspace.ModifiedMark {
		test.Errorf("the changed line 4 is marked %d, want modified", view.Marks[4])
	}
	if len(view.Hunks) != 1 || view.Hunks[0] != 3 {
		test.Errorf("the runs of marked lines are %v, want [3]", view.Hunks)
	}
	press(window, input.Key2, input.KeySuper) // the file has the keyboard, where 2 would type
	if application.mode.Name() != "Review" || view.Mode != viewer.DiffView {
		test.Errorf("⌘2 did not move to the review mode with the file as its diff")
	}
	if span, found := view.Inline[view.Hunks[0]]; !found || span.Start != 11 {
		test.Errorf("the changed word of the replaced line is %+v (found %v), want it from column 11, after the indent and \"return \"", span, found)
	}
}

func TestMarkingAFileViewedOpensTheNextOneNotViewed(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Mode: "Review"})
	window := newWindow(test, application)
	window.PressKey(input.KeyTab)
	window.Frames(1)
	press(window, input.KeyV, input.KeySuper, input.KeyShift)
	window.Frames(2)
	reviewing := application.mode.(*review.Mode)
	if !reviewing.IsViewed(workspace.FileReference{Project: 0, Path: "src/lamp.go"}) {
		test.Fatal("v did not mark lamp.go as viewed")
	}
	if document := application.viewer.Document; document == nil || document.Path != "notes.txt" {
		test.Fatalf("v did not open the next file not viewed, notes.txt")
	}
	if viewed, total := reviewing.Progress(); viewed != 1 || total != 3 {
		test.Errorf("the progress is %d of %d, want 1 of 3", viewed, total)
	}
}

func TestEnteringAProjectListsOnlyItsTree(test *testing.T) {
	folder := makeRepository(test)
	addUntouchedRepository(test, folder)
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	tree := &application.sidebar
	tree.Rebuild(application)
	for index, row := range tree.Rows {
		if row.Kind == sidebar.ProjectRow && row.Label == "lighthouse" {
			tree.Selected = index
		}
	}
	window.PressKey(input.KeyEnter)
	window.Frames(2)
	if tree.Entered == "" {
		test.Fatal("Enter on a project did not enter it")
	}
	inside := application.projectIndex(tree.Entered)
	if application.session.Projects[inside].Name != "lighthouse" {
		test.Fatalf("entered %s, want lighthouse", tree.Entered)
	}
	for _, row := range tree.Rows {
		if row.Kind == sidebar.ProjectRow || row.Project != inside {
			test.Fatalf("inside lighthouse the sidebar still lists %q of project %d", row.Label, row.Project)
		}
	}
	if !sidebarShows(application, "src") {
		test.Error("the project's own tree is not listed from its top")
	}
	window.PressKey(input.KeyEscape)
	window.Frames(2)
	if tree.Entered != "" || !sidebarShows(application, "harbour") {
		test.Error("Esc did not go back to every project")
	}
}

// Inside a project, Collapse all folds its folders, as it folds the projects outside; then the
// same button, Expand all, opens them again.
func TestInsideAProjectTheFoldButtonFoldsAndOpensItsFolders(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	application.sidebar.Enter(application, 0)
	application.sidebar.SetExpanded(sidebar.Row{Kind: sidebar.FolderRow, Project: 0, Path: "src"}, true)
	window.Frames(2)
	if !sidebarShows(application, "lamp.go") {
		test.Fatal("the open src folder does not list lamp.go")
	}
	if err := window.Click("Collapse all"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if sidebarShows(application, "lamp.go") || !sidebarShows(application, "src") {
		test.Error("Collapse all inside the project does not fold src, or hides it")
	}
	if err := window.Click("Expand all"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if !sidebarShows(application, "lamp.go") {
		test.Error("Expand all inside the project does not open src again")
	}
}

func TestTheFilterOfChangesLeavesOutProjectsAndFilesWithout(test *testing.T) {
	folder := makeRepository(test)
	addHarbour(test, folder) // a project with no changes
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	lighthouse := application.ProjectIndex(filepath.Join(folder, "lighthouse"))
	application.sidebar.SetExpanded(sidebar.Row{Project: lighthouse, Path: ""}, true)
	window.Frames(2)
	if !sidebarShows(application, "src") || !sidebarShows(application, "harbour") {
		test.Fatal("the projects are not open in the files mode")
	}
	press(window, input.KeyG, input.KeySuper, input.KeyShift)
	window.Frames(2)
	if sidebarShows(application, "harbour") || sidebarShows(application, "lamp.go") {
		test.Error("g left harbour, which has no changes, or opened src by itself")
	}
	application.sidebar.SetExpanded(sidebar.Row{Project: lighthouse, Path: "src"}, true)
	window.Frames(2)
	if !sidebarShows(application, "lamp.go") || sidebarShows(application, "keeper.go") {
		test.Error("⌘⇧G did not list only the changed files")
	}
	press(window, input.KeyG, input.KeySuper, input.KeyShift)
	window.Frames(2)
	if !sidebarShows(application, "keeper.go") || !sidebarShows(application, "harbour") {
		test.Error("g again did not list every project and file")
	}
}

func TestTheFinderFindsATextAndOpensItsLine(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	window.HoldKey(input.KeySuper)
	window.HoldKey(input.KeyShift)
	window.PressKey(input.KeyF)
	window.ReleaseKey(input.KeyShift)
	window.ReleaseKey(input.KeySuper)
	window.Frames(2)
	if !application.finder.Open || application.finder.Scope != finder.ContentScope {
		test.Fatal("⌘⇧F did not open the finder on the files' content")
	}
	window.Type("Beam")
	window.Frames(2)
	if len(application.finder.Hits) != 1 {
		test.Fatalf("found %d lines holding Beam, want 1: %+v", len(application.finder.Hits), application.finder.Hits)
	}
	window.PressKey(input.KeyEnter)
	window.Frames(2)
	view := application.viewer
	if view.Document == nil || view.Document.Path != "src/beam.go" || view.MarkedLine != 3 {
		test.Fatalf("Enter opened %+v at line %d, want src/beam.go at line 3", view.Document, view.MarkedLine)
	}
}

// newReader makes a reader with the program's two modes, keeping the viewed marks only in memory,
// and no language server unless a test gives one.
func newReader(settings Settings) *Window {
	reader, err := New(settings, files.New(), review.New(""))
	if err != nil {
		panic(err) // the two modes are given: this cannot happen
	}
	return reader
}

// startIn names the mode a test starts in: the review mode, or the first.
func startIn(reviewing bool) string {
	if reviewing {
		return "Review"
	}
	return ""
}

func TestTheFilesModeHasNothingOfTheReview(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go"})
	window := newWindow(test, application)
	file := workspace.FileReference{Project: 0, Path: "src/lamp.go"}
	if application.mode.Name() != "Files" {
		test.Fatalf("the reader started in the %s mode, want Files", application.mode.Name())
	}
	if button, shown := application.FileButton(file); shown {
		test.Errorf("the files mode shows the review's %q in the bar", button.Tip)
	}
	if _, total, _ := application.progress(); total > 0 {
		test.Error("the files mode counts the files viewed in the status bar")
	}
	var reviewing *review.Mode
	for _, each := range application.modes {
		if found, isReview := each.(*review.Mode); isReview {
			reviewing = found
		}
	}
	application.focus = viewerPane
	press(window, input.KeyV, input.KeySuper, input.KeyShift)
	if reviewing.IsViewed(file) {
		test.Error("v marked the file as viewed in the files mode")
	}
}

// The circle of a file in the sidebar marks it without opening another file, so nothing else
// makes the rows again: the mark itself must, or the counts keep their old number.
func TestMarkingAFileViewedUpdatesTheCountsAtOnce(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Mode: "Review"})
	window := newWindow(test, application)
	window.Frames(2)
	reviewing := application.mode.(*review.Mode)
	reviewing.Press(workspace.FileReference{Project: 0, Path: "src/lamp.go"}, false)
	window.Frames(2)
	if viewed, total, _ := application.progress(); viewed != 1 || total != 3 {
		test.Errorf("the status bar counts %d of %d viewed, want 1 of 3", viewed, total)
	}
}

// ⌘F searches the text of the project you are in, ⌘⇧F that of every project.
func TestCommandFSearchesOnlyTheProjectYouAreIn(test *testing.T) {
	folder := makeRepository(test)
	addLongRepository(test, folder)
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	selectRow(test, application, "lighthouse")
	press(window, input.KeyF, input.KeySuper)
	lighthouse := application.projectIndex(filepath.Join(folder, "lighthouse"))
	if !application.finder.Open || application.finder.Scope != finder.ContentScope || application.finder.Project != lighthouse {
		test.Fatalf("⌘F opened the finder %v on scope %v of project %d, want the content of lighthouse (%d)",
			application.finder.Open, application.finder.Scope, application.finder.Project, lighthouse)
	}
	if !slices.Contains(window.Texts(), "Find a text in the files of lighthouse") {
		test.Errorf("the finder does not say it searches lighthouse: %v", window.Texts())
	}
	application.finder.Open = false // as its field's Esc does, once the field lets go of it
	press(window, input.KeyF, input.KeySuper, input.KeyShift)
	if !application.finder.Open || application.finder.Project != -1 {
		test.Errorf("⌘⇧F searches project %d, want every project", application.finder.Project)
	}
}
