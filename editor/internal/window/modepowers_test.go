package window

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/mode"
	"github.com/akrck02/coddy/editor/internal/sidebar"
	"github.com/akrck02/coddy/editor/internal/workspace"
	ui "github.com/akrck02/go-ui"
)

// bareMode does only what every mode does: it lists the files and nothing else.
type bareMode struct{}

func (bareMode) Name() string                            { return "Bare" }
func (bareMode) ChangesOnly() bool                       { return false }
func (bareMode) Decorate(sidebar.Row) sidebar.Decoration { return sidebar.Decoration{} }
func (bareMode) Icon() string                            { return "folder" }
func (bareMode) Description() string                     { return "Bare: the files, nothing more" }
func (bareMode) Start(mode.Window)                       {}
func (bareMode) ShowsChanges() bool                      { return false }

func TestAPowerAModeLacksIsNotOffered(test *testing.T) {
	application, err := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true}, bareMode{})
	if err != nil {
		test.Fatal(err)
	}
	window := newWindow(test, application)
	file := workspace.FileReference{Project: 0, Path: "src/lamp.go"}
	if _, shown := application.FileButton(file); shown {
		test.Error("a mode with no file button has one in the viewer's header")
	}
	application.PressFileButton(file) // nothing to press: it must not stop the reader
	if _, total, words := application.progress(); total != 0 || words != "" {
		test.Errorf("a mode with no work to count shows %d in the status bar, %q", total, words)
	}
	if application.BuildInsteadOfAFile(&ui.Context{}) {
		test.Error("a mode with no page of its own shows one")
	}
	if len(application.ownCommands()) != 0 {
		test.Error("a mode with no keys of its own has some")
	}
	window.Frames(2)
}
