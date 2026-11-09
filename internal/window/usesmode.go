package window

import (
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
)

// usesHolder is the mode that shows the uses of a name, when the reader has one; otherwise the
// finder lists them.
func (application *Window) usesHolder() (mode.UsesHolder, bool) {
	for _, each := range application.modes {
		if holder, holds := each.(mode.UsesHolder); holds {
			return holder, true
		}
	}
	return nil, false
}

// showUses shows the uses found so far: in the uses mode, turning it on when the search starts
// (opening), or in the finder.
func (application *Window) showUses(search *usesSearch, opening bool) {
	note := application.usesNote(search)
	search.noted = note
	holder, held := application.usesHolder()
	if !held {
		if opening {
			application.finder.ShowUsesFrom(application, "uses of "+search.name, search.home, application.usesListed(search))
		} else {
			application.finder.UpdateUses(application, application.usesListed(search))
		}
		application.finder.Note = note
		return
	}
	holder.SetUses(search.name, search.home, application.modeUses(search), note)
	if opening {
		application.setMode(holder)
	}
	application.markUses()
}

// noteUses says again how the search goes, when that changed.
func (application *Window) noteUses(search *usesSearch) {
	if application.usesNote(search) != search.noted {
		application.showUses(search, false)
	}
}

// modeUses are the uses found, as the uses mode keeps them.
func (application *Window) modeUses(search *usesSearch) []mode.Use {
	var found []mode.Use
	for _, location := range search.found {
		if use, known := application.modeUseOf(location); known {
			found = append(found, use)
		}
	}
	return found
}

// modeUseOf is a place a server gave as a use, when it is in a project open.
func (application *Window) modeUseOf(location lsp.Location) (mode.Use, bool) {
	path := lsp.PathOf(location.URI)
	reference, found := application.referenceOf(path)
	if !found {
		return mode.Use{}, false
	}
	start, end := location.Range.Start, location.Range.End
	text := lineOf(path, start.Line, application)
	column := lsp.CharacterColumn(text, start.Character)
	last := column
	if end.Line == start.Line {
		last = lsp.CharacterColumn(text, end.Character)
	}
	return mode.Use{File: reference, Line: start.Line + 1, Start: column, End: last}, true
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
func (application *Window) usesOfFile(file workspace.FileReference) map[int][]workspace.Span {
	if holder, holds := application.mode.(mode.UsesHolder); holds {
		return holder.UsesIn(file)
	}
	return nil
}

// RevealLine opens a file at a line, marked, and shows it in the sidebar.
func (application *Window) RevealLine(file workspace.FileReference, line int) {
	application.openFileAt(file, line)
	application.sidebar.RevealFile(application, file)
}

// Relist lists the files again, as what the mode lists changed.
func (application *Window) Relist() { application.sidebar.Invalidate() }

// SearchUsesAgain makes the last search of uses again, from the start.
func (application *Window) SearchUsesAgain() {
	last := application.uses
	if last == nil {
		application.toast = "No search to make again: press ⇧F12 on a name"
		return
	}
	application.searchUses(last.asked, last.at, last.name, last.home)
}
