// Package mode says what a mode of the reader is: one way of looking at the workspace, chosen
// from the mode rail. The window holds the modes and asks them; the modes implement Mode and
// reach the window through Window. Neither imports the other, so a mode is one package and one
// line in main.
package mode

import (
	"context"

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
	// OnlyWhenOn keeps the command to while its mode is on, before the window's own keys: a mode
	// may then give a key of the window (n) a meaning of its own.
	OnlyWhenOn bool
}

// Use is a place a name is used: a line of a file, from 1, and the characters of the name in it.
type Use struct {
	File       workspace.FileReference
	Line       int
	Start, End int
	// Probable is a place no server confirmed yet, whose file names the declaration for sure
	Probable bool
}

// UsesHolder is a mode that shows the uses of a name the window searches (⌥F7): the window gives
// it the uses as they are found, and asks it which to mark in a file.
type UsesHolder interface {
	Mode
	// SetUses gives the uses of a name found so far, in the order to go through them, and a note
	// on how the search goes; home is the project the name was asked in.
	SetUses(name string, home int, uses []Use, note string)
	// UsesIn are the uses of a file to mark, by line from 1.
	UsesIn(file workspace.FileReference) map[int][]workspace.Span
}

// Cover is a mode that may show something of its own over the viewer, such as a graph; it tells
// whether it did.
type Cover interface {
	BuildOverViewer(context *ui.Context) bool
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
	// RevealLine opens a file at a line, from 1, marked, and shows it in the sidebar.
	RevealLine(file workspace.FileReference, line int)
	// Relist lists the files again, as what the mode lists changed.
	Relist()
	// Keys are the frame's keys and pointer, for a mode that draws something of its own.
	Keys() pane.Keys
	// Run starts background work under a key, replacing the work under that key still running;
	// what it returns runs on the interface's goroutine.
	Run(key string, do func(stop context.Context) func())
}
