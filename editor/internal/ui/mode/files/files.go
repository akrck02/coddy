// Package files is the files mode: every file of every project, opened whole, its changes marked
// beside the lines; the changed files can be listed alone with g.
package files

import (
	"fmt"

	"github.com/akrck02/coddy/editor/internal/ui/mode"
	"github.com/akrck02/coddy/editor/internal/ui/sidebar"
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
