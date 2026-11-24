package finder

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
)

// fakeWindow is a window of one project that runs its work at once, as a test window does, and
// keeps the file it was asked to open.
type fakeWindow struct {
	projects   []*workspace.Project
	candidates []Candidate
	opened     workspace.FileReference
	openedLine int
}

func (window *fakeWindow) Look() theme.Look               { return theme.Look{} }
func (window *fakeWindow) Keys() pane.Keys                { return pane.Keys{} }
func (window *fakeWindow) Projects() []*workspace.Project { return window.projects }
func (window *fakeWindow) Candidates() []Candidate        { return window.candidates }
func (window *fakeWindow) Searchable(project int) bool    { return true }
func (window *fakeWindow) WorkingTree() bool              { return true }
func (window *fakeWindow) Synchronous() bool              { return true }
func (window *fakeWindow) Cancel(key string)              {}

func (window *fakeWindow) Read(stop context.Context, reading func()) bool {
	reading()
	return true
}

func (window *fakeWindow) Run(key string, do func(stop context.Context) func()) {
	if done := do(context.Background()); done != nil {
		done()
	}
}

func (window *fakeWindow) OpenAt(file workspace.FileReference, line int) {
	window.opened, window.openedLine = file, line
}

// newFakeWindow is a window over one git repository that holds lamp.go.
func newFakeWindow(test *testing.T) *fakeWindow {
	folder := test.TempDir()
	lines := "package lamp\n\n// Keeper keeps the light\nfunc Keeper() {}\n"
	if err := os.WriteFile(filepath.Join(folder, "lamp.go"), []byte(lines), 0o644); err != nil {
		test.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "."}} {
		command := exec.Command("git", arguments...)
		command.Dir = folder
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v %s", arguments, err, output)
		}
	}
	return &fakeWindow{
		projects: []*workspace.Project{{Name: "lighthouse", Path: folder}},
		candidates: []Candidate{
			{Project: 0, Path: "lamp.go", Label: "lighthouse/lamp.go"},
			{Project: 0, Path: "tower.md", Label: "lighthouse/tower.md"},
		},
	}
}

func TestTheNamesSourceFindsAFileByPartOfItsNameAndOpensItAtItsTop(test *testing.T) {
	window := newFakeWindow(test)
	var finder Finder
	finder.Show(NamesScope)
	finder.Query = "lamp"
	finder.refresh(window)
	if finder.count() != 1 {
		test.Fatalf("%d files match \"lamp\", want 1", finder.count())
	}
	finder.choose(window)
	if window.opened.Path != "lamp.go" || window.openedLine != 0 {
		test.Errorf("opened %v at line %d, want lamp.go at its top", window.opened, window.openedLine)
	}
}

func TestTheContentSourceFindsTheLinesThatHoldTheTextAndOpensOneAtItsLine(test *testing.T) {
	window := newFakeWindow(test)
	var finder Finder
	finder.Show(ContentScope)
	finder.Query = "Keeper"
	finder.refresh(window)
	if finder.count() != 2 {
		test.Fatalf("%d lines hold \"Keeper\", want 2", finder.count())
	}
	finder.choose(window)
	if window.opened.Path != "lamp.go" || window.openedLine != 3 {
		test.Errorf("opened %v at line %d, want lamp.go at line 3", window.opened, window.openedLine)
	}
}

func TestTheContentSourceWaitsForTwoLetters(test *testing.T) {
	window := newFakeWindow(test)
	var finder Finder
	finder.Show(ContentScope)
	finder.Query = "K"
	finder.refresh(window)
	if finder.count() != 0 {
		test.Errorf("one letter found %d lines, want none", finder.count())
	}
}

func TestTheReferencesSourceNarrowsTheUsesByFileOrLine(test *testing.T) {
	window := newFakeWindow(test)
	var finder Finder
	finder.ShowUses(window, "uses of Keeper", []Use{
		{Project: 0, Path: "lamp.go", Line: 4, Column: 5, Length: 6, Text: "func Keeper() {}"},
		{Project: 0, Path: "tower.go", Line: 9, Column: 1, Length: 6, Text: "Keeper()"},
	})
	finder.refresh(window)
	if finder.count() != 2 {
		test.Fatalf("%d uses listed, want 2", finder.count())
	}
	finder.Query = "tower"
	finder.refresh(window)
	if finder.count() != 1 {
		test.Fatalf("%d uses hold \"tower\", want 1", finder.count())
	}
	finder.choose(window)
	if window.opened.Path != "tower.go" || window.openedLine != 9 {
		test.Errorf("opened %v at line %d, want tower.go at line 9", window.opened, window.openedLine)
	}
}
