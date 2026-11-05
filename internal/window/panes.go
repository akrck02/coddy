package window

import (
	"context"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
	ui "github.com/akrck02/go-ui"
)

// What the panes ask of the window they sit in. Each pane declares, as an interface, the little it
// needs; Window answers all of them.

// Look is how the window is painted now, light or dark.
func (application *Window) Look() theme.Look { return application.look }

// Keys is what the keyboard and mouse did in this frame.
func (application *Window) Keys() pane.Keys { return application.keys }

// HasProjects tells whether any repository is open.
func (application *Window) HasProjects() bool { return len(application.projects) > 0 }

// OpenFolder asks for a folder of repositories to open.
func (application *Window) OpenFolder() { application.pickerOpen = true }

// FocusViewer gives the keyboard to the file's view.
func (application *Window) FocusViewer() { application.focus = viewerPane }

// BuildInsteadOfAFile shows what the mode that is on shows while no file is open, if anything.
func (application *Window) BuildInsteadOfAFile(context *ui.Context) bool {
	return application.mode.BuildInsteadOfAFile(context)
}

// Projects are the repositories open, in the order the sidebar lists them.
func (application *Window) Projects() []*workspace.Project { return application.projects }

// Candidates are every file the finder can find by name.
func (application *Window) Candidates() []finder.Candidate { return application.candidates }

// Searchable tells whether the finder's content search looks in a project.
func (application *Window) Searchable(index int) bool {
	project := application.projects[index]
	return project.Loaded && !application.hidden(project) && application.inScope(workspace.FileReference{Project: index})
}

// WorkingTree tells whether the changes not committed yet count.
func (application *Window) WorkingTree() bool { return application.settings.WorkingTree }

// Synchronous tells whether work runs at once, for a capture or a test.
func (application *Window) Synchronous() bool { return application.settings.Synchronous }

// Run starts background work under a key, replacing the work under that key still running.
func (application *Window) Run(key string, do func(stop context.Context) func()) {
	application.work.run(key, do)
}

// Cancel stops the work under a key, if any.
func (application *Window) Cancel(key string) { application.work.cancel(key) }

// Read reads a repository once one of the few readers is free.
func (application *Window) Read(stop context.Context, reading func()) bool {
	return application.work.read(stop, reading)
}

// OpenAt opens a file at a line, shows it in the sidebar and gives it the keyboard.
func (application *Window) OpenAt(file workspace.FileReference, line int) {
	application.openFileAt(file, line)
	application.sidebar.RevealFile(application, file)
	application.focus = viewerPane
}

// ProjectIndex is a project's place in Projects, by its path; -1 when it is not open.
func (application *Window) ProjectIndex(path string) int { return application.projectIndex(path) }

// Hidden tells whether a project is left out of the lists.
func (application *Window) Hidden(project *workspace.Project) bool {
	return application.hidden(project)
}

// ShowEveryProject shows the projects with no changes again.
func (application *Window) ShowEveryProject() { application.setOnlyChanged(false) }

// OpenFile shows a file in the viewer.
func (application *Window) OpenFile(file workspace.FileReference) { application.openFile(file) }

// OpenedFile is the file the viewer shows, if any.
func (application *Window) OpenedFile() (workspace.FileReference, bool) {
	return application.viewer.Reference, application.viewer.IsOpen()
}

// IsOpened tells whether the viewer shows a file.
func (application *Window) IsOpened(file workspace.FileReference) bool {
	return application.viewer.IsOpen() && application.viewer.Reference == file
}

// FocusSidebar gives the keyboard to the sidebar.
func (application *Window) FocusSidebar() { application.focus = sidebarPane }

// SidebarFocused tells whether the sidebar has the keyboard.
func (application *Window) SidebarFocused() bool { return application.focus == sidebarPane }

// ClassKind is what a Java or Kotlin file declares, once read.
func (application *Window) ClassKind(file workspace.FileReference) java.ClassKind {
	return application.classKind(file)
}

// Loading tells whether projects are being read.
func (application *Window) Loading() bool { return application.loading > 0 }

// ReadingKinds tells whether class kinds are being read.
func (application *Window) ReadingKinds() bool { return len(application.kindsWanted) > 0 }

// FileButton is the viewer header's button for a file: the one of the mode that is on, else the
// first mode that has one, so a file can be marked as viewed whatever the mode, as v can.
func (application *Window) FileButton(file workspace.FileReference) (viewer.Button, bool) {
	if owner := application.buttonOwner(file); owner != nil {
		return owner.FileButton(file)
	}
	return viewer.Button{}, false
}

// PressFileButton presses the header's button for the file open, moving on as a reviewer would.
func (application *Window) PressFileButton(file workspace.FileReference) {
	if owner := application.buttonOwner(file); owner != nil {
		owner.Press(file, true)
	}
}

// buttonOwner is the mode whose button the viewer's header shows for a file, if any.
func (application *Window) buttonOwner(file workspace.FileReference) mode.Mode {
	if _, shown := application.mode.FileButton(file); shown {
		return application.mode
	}
	for _, each := range application.modes {
		if _, shown := each.FileButton(file); shown {
			return each
		}
	}
	return nil
}

// ListEveryFile turns to the first mode that lists every file. It is asked by the sidebar while
// it reveals a file, so it only changes what the sidebar lists; the sidebar goes on revealing.
func (application *Window) ListEveryFile() {
	for _, each := range application.modes {
		if !each.ChangesOnly() {
			application.mode = each
			application.sidebar.Listing = each
			application.sidebar.Stale = true
			return
		}
	}
}

// ChangedFiles are the files the development changed, in the order n goes through them.
func (application *Window) ChangedFiles() []workspace.FileReference { return application.changedOrder }

// Shows tells whether a file is in the project entered, if any.
func (application *Window) Shows(file workspace.FileReference) bool { return application.inScope(file) }

// Reveal opens a file and shows it in the sidebar.
func (application *Window) Reveal(file workspace.FileReference) {
	application.openFile(file)
	application.sidebar.RevealFile(application, file)
}

// Target is the file a key acts on: the one selected in the sidebar while it has the keyboard,
// else the one open. inViewer tells whether it is the one open, or the viewer has the keyboard.
func (application *Window) Target() (file workspace.FileReference, inViewer bool, found bool) {
	switch {
	case application.focus == sidebarPane:
		if row, ok := application.sidebar.SelectedRow(); ok && row.Kind == sidebar.FileRow {
			file, found = workspace.FileReference{Project: row.Project, Path: row.Path}, true
		} else if application.viewer.IsOpen() {
			file, found = application.viewer.Reference, true
		}
	case application.viewer.IsOpen():
		file, found = application.viewer.Reference, true
	}
	opened := application.viewer.IsOpen() && application.viewer.Reference == file
	return file, found && (application.focus == viewerPane || opened), found
}

// ToggleChangedFilter lists only the changed files in the sidebar, or every file again.
func (application *Window) ToggleChangedFilter() { application.sidebar.ToggleChangedFilter() }

// Toast shows a short note at the bottom of the window.
func (application *Window) Toast(words string) { application.toast = words }
