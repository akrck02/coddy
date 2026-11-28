package window

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/workspace"
)

// walkFiles visits a tree's files in the order the sidebar shows them: folders first.
func walkFiles(folder *codebase.Folder, visit func(path string)) {
	for _, child := range folder.Folders {
		walkFiles(child, visit)
	}
	for _, path := range folder.Files {
		visit(path)
	}
}

// openFile opens a file of a project in the viewer, reading it on another goroutine. Another
// file asks first about the edits not saved of the one open.
func (application *Window) openFile(reference codebase.FileReference) {
	if reference.Project < 0 || reference.Project >= len(application.workspace.Projects) {
		return
	}
	if application.viewer.Unsaved() && reference != application.viewer.Reference {
		application.keepingEdits(func() { application.openFile(reference) })
		return
	}
	project := application.workspace.Projects[reference.Project]
	application.folder.shown = false
	application.viewer.Start(reference, project)
	application.viewer.Uses = application.usesOfFile(reference)
	application.viewer.Comments = application.review.commentsOf(project, reference.Path)
	if document, cached := application.workspace.Document(reference); cached {
		application.viewer.Show(document, application.mode.ShowsChanges())
		return
	}
	workingTree := application.workspace.WorkingTree
	request := application.viewer.Request
	application.work.Run("", func(context.Context) func() {
		read := workspace.ReadDocument(project, reference, workingTree)
		return func() {
			application.workspace.Keep(read)
			if application.viewer.Request == request {
				application.viewer.Show(read.Document, application.mode.ShowsChanges())
			}
		}
	})
}

// openFileAt opens a file and shows one of its lines, marked; 0 shows its top (or first change).
func (application *Window) openFileAt(reference codebase.FileReference, line int) {
	application.viewer.GoTo = line
	application.openFile(reference)
	view := &application.viewer
	if view.GoTo > 0 && view.Document != nil && view.Reference == reference {
		view.RevealLine(view.GoTo)
	}
}

// stepChangedFile opens the next (or previous) changed file of the whole development.
func (application *Window) stepChangedFile(step int) {
	var order []codebase.FileReference
	for _, reference := range application.changedOrder {
		if application.inScope(reference) && !application.sidebar.LeavesOut(application.workspace.Projects[reference.Project], reference.Path) {
			order = append(order, reference)
		}
	}
	if len(order) == 0 {
		return
	}
	current := -1
	for index, reference := range order {
		if reference == application.viewer.Reference && application.viewer.IsOpen() {
			current = index
			break
		}
	}
	next := current + step
	if current < 0 && step < 0 {
		next = len(order) - 1
	}
	next = (next + len(order)) % len(order)
	application.openFile(order[next])
	application.sidebar.RevealFile(application, order[next])
}

// openByLabel opens a file named as "project/path/in/it", once the projects have been read.
func (application *Window) openByLabel(label string) {
	for _, candidate := range application.candidates {
		if candidate.Label == label {
			reference := codebase.FileReference{Project: candidate.Project, Path: candidate.Path}
			application.openFile(reference)
			application.sidebar.RevealFile(application, reference)
			application.focus = viewerPane
			return
		}
	}
	application.toast = "No file " + label + " in the projects open"
}
