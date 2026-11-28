package window

import (
	"path/filepath"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/servers"
)

// The window is the servers' Host: what follows is what they ask of it.

// InSight tells whether a project is shown in the sidebar, so a server for the whole window reads it.
func (application *Window) InSight(project *codebase.Project) bool {
	return !application.hidden(project)
}

// servedFile is the file open as its server is told it: none unless it is on disk and editable.
func (application *Window) servedFile() servers.OpenFile {
	view := &application.viewer
	if !view.IsOpen() || view.Project == nil {
		return servers.OpenFile{}
	}
	open := servers.OpenFile{Project: view.Project.Path}
	if view.Document == nil || !view.Document.Editable {
		return open
	}
	open.Path = filepath.Join(view.Project.Path, filepath.FromSlash(view.Reference.Path))
	open.Document, open.Buffer = view.Document, view.Edit
	return open
}
