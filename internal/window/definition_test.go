package window

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

// addHarbour adds a repository whose Go file uses Keeper, declared in the lighthouse repository.
func addHarbour(test *testing.T, folder string) {
	test.Helper()
	repository := filepath.Join(folder, "harbour")
	if err := os.MkdirAll(filepath.Join(repository, "src"), 0o755); err != nil {
		test.Fatal(err)
	}
	use := "package harbour\n\n// a keeper for the harbour\nvar keeper = lamp.Keeper{}\n"
	if err := os.WriteFile(filepath.Join(repository, "src", "use.go"), []byte(use), 0o644); err != nil {
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

// followingNames opens harbour's use.go, edited, with one fake server for every Go project of the window.
func followingNames(test *testing.T) (*Window, *uitest.Window) {
	folder := makeRepository(test)
	addHarbour(test, folder)
	servers := []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"go"}, OnePerWindow: true}}
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true,
		Open: "harbour/src/use.go", Servers: servers})
	test.Cleanup(application.stopServers)
	window := newWindow(test, application)
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	window.Frames(1)
	return application, window
}

// isAt tells whether the viewer shows a file of a project at a line.
func isAt(application *Window, project, path string, line int) bool {
	view := application.viewer
	return view.Project != nil && filepath.Base(view.Project.Path) == project && view.Reference.Path == path && view.MarkedLine == line
}

func TestDefinitionOpensTheDeclaration(test *testing.T) {
	application, window := followingNames(test)
	application.viewer.PlaceCaret(viewer.Place{Line: 3, Column: 20}) // in "Keeper" of lamp.Keeper{}
	press(window, input.KeyF12)
	if !isAt(application, "lighthouse", "src/keeper.go", 4) {
		test.Fatalf("F12 opened %s at line %d, want lighthouse's src/keeper.go at line 4 (%s)",
			application.viewer.Reference.Path, application.viewer.MarkedLine, application.toast)
	}
	if place, _ := application.viewer.Caret(); place != (viewer.Place{Line: 3, Column: 5}) {
		test.Errorf("the caret is at %+v, want on Keeper, at line 3 column 5 from 0", place)
	}
}

func TestBackReturnsToThePlaceBefore(test *testing.T) {
	application, window := followingNames(test)
	application.viewer.PlaceCaret(viewer.Place{Line: 3, Column: 20})
	application.GoToDefinition(viewer.Place{Line: 3, Column: 20})
	window.Frames(1)
	press(window, input.KeyMinus, input.KeyControl)
	if !isAt(application, "harbour", "src/use.go", 4) {
		test.Fatalf("⌃- went to %s at line %d, want harbour's src/use.go at line 4",
			application.viewer.Reference.Path, application.viewer.MarkedLine)
	}
	press(window, input.KeyMinus, input.KeyControl, input.KeyShift)
	if !isAt(application, "lighthouse", "src/keeper.go", 4) {
		test.Errorf("⌃⇧- went to %s at line %d, want src/keeper.go again", application.viewer.Reference.Path, application.viewer.MarkedLine)
	}
	if len(application.trail.back) != 1 {
		test.Errorf("the trail keeps %d places back, want 1", len(application.trail.back))
	}
}
