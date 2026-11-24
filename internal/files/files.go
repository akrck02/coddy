// Package files is the files mode: every file of every project, opened whole, its changes marked
// beside the lines; the changed files can be listed alone with g.
package files

import (
	"fmt"

	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
	ui "github.com/akrck02/go-ui"
)

// Mode is the files mode.
type Mode struct {
	window mode.Window
}

// New makes the files mode.
func New() *Mode { return &Mode{} }

// Name is the sidebar's title in this mode.
func (files *Mode) Name() string { return "Files" }

// Icon is the mode's icon on the rail.
func (files *Mode) Icon() string { return "folder-tree" }

// Description is the rail's tooltip.
func (files *Mode) Description() string {
	return "Files (1): read any file, the changes marked beside the lines"
}

// Start gives the mode its window.
func (files *Mode) Start(window mode.Window) { files.window = window }

// ChangesOnly is false: every file is listed, unless g asks for the changed ones.
func (files *Mode) ChangesOnly() bool { return false }

// HidesDeleted is true: a file the development deleted is not there to read or edit; the review
// mode lists it, to read what was removed.
func (files *Mode) HidesDeleted() bool { return true }

// ShowsChanges is false: a file opens whole, its changes marked beside the lines.
func (files *Mode) ShowsChanges() bool { return false }

// Header is empty: the files mode has nothing to count.
func (files *Mode) Header() string { return "" }

// Decorate tints a changed file's name by how it changed, counts a project's changed files and
// dots a folder that holds some.
func (files *Mode) Decorate(row sidebar.Row) sidebar.Decoration {
	switch row.Kind {
	case sidebar.FileRow:
		return sidebar.Decoration{Tinted: true}
	case sidebar.ProjectRow:
		if row.Changed > 0 {
			return sidebar.Decoration{Words: []sidebar.Word{{Text: fmt.Sprint(row.Changed), Ink: sidebar.WarningInk, Status: true}}}
		}
	case sidebar.FolderRow:
		return sidebar.Decoration{Dot: row.Changed > 0}
	}
	return sidebar.Decoration{}
}

// Press does nothing: the files mode marks no file.
func (files *Mode) Press(file workspace.FileReference, moveOn bool) {}

// FileButton is none: the files mode adds no button to the viewer.
func (files *Mode) FileButton(file workspace.FileReference) (viewer.Button, bool) {
	return viewer.Button{}, false
}

// Progress is none: the files mode has no work to count.
func (files *Mode) Progress() (done, total int) { return 0, 0 }

// BuildInsteadOfAFile leaves the viewer's own page: how to start.
func (files *Mode) BuildInsteadOfAFile(context *ui.Context) bool { return false }

// Commands are the files mode's keys: none of its own, as the filter of changes (g) is the window's.
func (files *Mode) Commands() []mode.Command { return nil }
