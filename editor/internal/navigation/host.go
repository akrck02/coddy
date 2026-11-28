// Package navigation follows names through the code: where a name is declared, the trail of places
// to go back to, what a server says of a name, and every use of a name across the projects open.
// It asks the servers through the window, its Host, and imports no pane.
package navigation

import (
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/lsp"
)

// Host is what navigation asks of the window it runs in.
type Host interface {
	// Projects are the projects open, in the sidebar's order.
	Projects() []*codebase.Project
	// LineOf is a line of a file on disk, from 0, as the reader has it: with the edits not saved
	// of the file open.
	LineOf(path string, line int) string
	// ClientFor is the running server of a file of a project, by its key; reads tells whether any
	// server reads the file, client is nil while it starts.
	ClientFor(project, path string) (client *lsp.Client, key string, reads bool)
	// ShowUses shows the uses a search found so far, with its note; opening tells that the search
	// has just begun.
	ShowUses(search *Search, note string, opening bool)
	// NoteUses says again how a search goes, its uses unchanged.
	NoteUses(note string)
}
