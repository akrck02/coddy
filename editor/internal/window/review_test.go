package window

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/workspace"
)

// makePullRequest is makeRepository with main checked out, and itself as its remote origin: the
// branch feature/brighter is then a pull request to fetch from it, as the fake review source says.
func makePullRequest(test *testing.T) string {
	test.Helper()
	folder := makeRepository(test)
	repository := filepath.Join(folder, "lighthouse")
	for _, arguments := range [][]string{{"checkout", "-q", "main"}, {"remote", "add", "origin", repository}} {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return folder
}

func TestAPullRequestChosenShowsItsChangesAgainstItsBaseAndItsComments(test *testing.T) {
	plugin := fakePlugin(test)
	application := newReader(Settings{Folders: []string{makePullRequest(test)}, Synchronous: true, WorkingTree: true,
		Plugins: []*plugins.Plugin{plugin}})
	window := newWindow(test, application)
	if changes := application.session.Projects[0].ChangedPaths(); len(changes) != 0 {
		test.Fatalf("main has %v changed before a pull request is chosen", changes)
	}
	application.listReviews(plugin)
	dialog := application.reviewing
	if !dialog.open || len(dialog.labels) != 1 || !strings.Contains(dialog.labels[0], "Brighter lamp") {
		test.Fatalf("the pull requests listed are %v (%q)", dialog.labels, application.toast)
	}
	window.Frames(1)
	if !slices.Contains(window.Texts(), dialog.labels[0]) {
		test.Errorf("the dialog does not show %q", dialog.labels[0])
	}
	application.chooseReview(plugin, dialog.project, dialog.reviews[0])
	window.Frames(2)
	project := application.session.Projects[0]
	if project.Head == "" || project.Base != "origin/main" || strings.Join(project.ChangedPaths(), " ") != "notes.txt src/beam.go src/lamp.go" {
		test.Fatalf("the pull request reads base %q, changes %v (%v, %q)", project.Base, project.ChangedPaths(), project.Error, application.toast)
	}
	application.openFile(workspace.FileReference{Project: 0, Path: "src/lamp.go"})
	window.Frames(2)
	document := application.viewer.Document
	if document == nil || len(document.NewLines) < 4 || !strings.Contains(document.NewLines[3].Text(), "true") {
		test.Fatalf("src/lamp.go does not show the pull request's side: %+v", document)
	}
	if words := application.viewer.Comments[4]; words != "keeper: always on?" {
		test.Errorf("the comment on line 4 reads %q", words)
	}
	if !slices.Contains(window.Texts(), "keeper: always on?") {
		test.Error("the comment is not drawn beside its line")
	}
}

func TestAReviewSourceThatCrashesIsToldWithItsCodeAndThePullRequestStillShows(test *testing.T) {
	plugin := fakePlugin(test)
	application := newReader(Settings{Folders: []string{makePullRequest(test)}, Synchronous: true, WorkingTree: true,
		Plugins: []*plugins.Plugin{plugin}})
	window := newWindow(test, application)
	application.listReviews(plugin)
	crashing := application.reviewing.reviews[0]
	crashing.ID = "8"
	application.chooseReview(plugin, application.reviewing.project, crashing)
	window.Frames(2)
	if !strings.Contains(application.shownToast, "plugins.crashed") || plugin.State != "Failed: plugins.crashed" {
		test.Errorf("the crash was told %q, the plugin's state is %q", application.shownToast, plugin.State)
	}
	if len(application.session.Projects[0].Changes) != 3 {
		test.Errorf("the pull request shows %d changes without its comments", len(application.session.Projects[0].Changes))
	}
}
