package window

import (
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/mode"
	"github.com/akrck02/coddy/editor/internal/navigation"
	"github.com/akrck02/coddy/editor/internal/viewer"
)

// usesHolder is the mode that shows the uses of a name, when the reader has one; otherwise the
// finder lists them.
func (application *Window) usesHolder() (mode.UsesHolder, bool) {
	return application.usesMode, application.usesMode != nil
}

// The window is the Host of navigation's searches of uses: what follows is what they ask of it.

// ClientFor is the running server of a file of a project, by its key; reads tells whether any
// server reads the file, client is nil while it starts.
func (application *Window) ClientFor(project, path string) (*lsp.Client, string, bool) {
	return application.servers.ClientFor(project, path)
}

// ShowUses shows the uses found so far: in the uses mode, turning it on when the search starts
// (opening), or in the finder.
func (application *Window) ShowUses(search *navigation.Search, note string, opening bool) {
	holder, held := application.usesHolder()
	if !held {
		listed := navigation.Listed(application, search)
		if opening {
			application.finder.ShowUsesFrom(application, "uses of "+search.Name, search.Home, listed)
		} else {
			application.finder.UpdateUses(application, listed)
		}
		application.finder.Note = note
		return
	}
	holder.SetUses(search.Name, search.Home, navigation.ModeUses(application, search), note)
	if opening {
		application.setMode(holder)
	}
	application.markUses()
}

// NoteUses says again how the search of uses goes, its uses unchanged.
func (application *Window) NoteUses(note string) {
	if holder, held := application.usesHolder(); held {
		holder.SetUsesNote(note)
		return
	}
	application.finder.Note = note
}

// markUses gives the viewer the uses of the file open, when the uses mode is on.
func (application *Window) markUses() {
	view := &application.viewer
	if !view.IsOpen() {
		return
	}
	marked := application.usesOfFile(view.Reference)
	view.SetUses(marked)
	if len(marked) > 0 && view.Mode == viewer.FileView && view.Edit == nil {
		view.SetMode(viewer.UsesView)
	}
}

// usesOfFile are the uses to mark in a file: those of the uses mode, while it is on.
func (application *Window) usesOfFile(file codebase.FileReference) map[int][]codebase.Span {
	if holder := application.powers.uses; holder != nil {
		return holder.UsesIn(file)
	}
	return nil
}

// RevealLine opens a file at a line, marked, and shows it in the sidebar.
func (application *Window) RevealLine(file codebase.FileReference, line int) {
	application.openFileAt(file, line)
	application.sidebar.RevealFile(application, file)
}

// Relist lists the files again, as what the mode lists changed.
func (application *Window) Relist() { application.sidebar.Invalidate() }

// SearchUsesAgain makes the last search of uses again, from the start.
func (application *Window) SearchUsesAgain() {
	last := application.uses.Last()
	if last == nil {
		application.toast = "No search to make again: press ⌥F7 on a name"
		return
	}
	application.searchUses(last.Question)
}
