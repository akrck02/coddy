// Package mode says what a mode of the reader is: one way of looking at the workspace, chosen
// from the mode rail. The window holds the modes and asks them; the modes implement Mode and
// reach the window through Window. Neither imports the other, so a mode is one package and one
// line in main.
package mode

import (
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
	ui "github.com/akrck02/go-ui"
)

// Mode is one way of looking at the workspace. Both modes list the same tree and open the same
// files; a mode answers which files are listed, how a changed file opens, and what each row
// shows beside its name.
type Mode interface {
	sidebar.Listing
	Icon() string
	// Description is the rail's tooltip: the mode's name, its key and what it is for.
	Description() string
	// Start gives the mode the window it works in, once, before the first frame.
	Start(window Window)
	ShowsChanges() bool // opens a changed file on its changes, not whole
	// FileButton is the mode's button in the viewer's header, such as Viewed.
	FileButton(file workspace.FileReference) (viewer.Button, bool)
	// Progress is how far the mode's work has gone, for the status bar whatever mode is on;
	// a total of 0 shows nothing.
	Progress() (done, total int)
	BuildInsteadOfAFile(context *ui.Context) bool // what shows while no file is open
	Commands() []Command                          // the mode's own keys, such as v
}

// Command is a key of a mode and what it does; step tells which way to go when several keys
// share it.
type Command struct {
	Description string
	Keys        []pane.Press
	Do          func(step int)
}

// Window is what a mode needs from the window it works in.
type Window interface {
	Look() theme.Look
	Projects() []*workspace.Project
	// Hidden tells whether a project is left out of the lists.
	Hidden(project *workspace.Project) bool
	// ChangedFiles are the files the development changed, in the order n goes through them;
	// Shows tells whether one is in the project entered, if any.
	ChangedFiles() []workspace.FileReference
	Shows(file workspace.FileReference) bool
	// Reveal opens a file and shows it in the sidebar; FocusViewer gives it the keyboard.
	Reveal(file workspace.FileReference)
	FocusViewer()
	// Target is the file a key acts on: the one selected in the sidebar while it has the
	// keyboard, else the one open; inViewer tells whether it is the one open.
	Target() (file workspace.FileReference, inViewer bool, found bool)
	ToggleChangedFilter()
	Loading() bool
	// Toast shows a short note at the bottom of the window.
	Toast(words string)
}
