package window

import (
	"context"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/theme"
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
func (application *Window) FocusViewer() { application.focus = editorPane }

// BuildInsteadOfAFile shows the review's overview while no file is open.
func (application *Window) BuildInsteadOfAFile(context *ui.Context) bool {
	if application.sidebar.Mode != sidebar.ReviewMode || len(application.changedOrder) == 0 {
		return false
	}
	application.buildOverview(context)
	return true
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
	application.focus = editorPane
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
	return application.editor.Reference, application.editor.IsOpen()
}

// IsOpened tells whether the viewer shows a file.
func (application *Window) IsOpened(file workspace.FileReference) bool {
	return application.editor.IsOpen() && application.editor.Reference == file
}

// FocusSidebar gives the keyboard to the sidebar.
func (application *Window) FocusSidebar() { application.focus = sidebarPane }

// SidebarFocused tells whether the sidebar has the keyboard.
func (application *Window) SidebarFocused() bool { return application.focus == sidebarPane }

// ReviewProgress counts the changed files viewed, and all of them, in the projects shown.
func (application *Window) ReviewProgress() (viewed, total int) { return application.reviewProgress() }

// ProjectProgress counts the changed files viewed, and all of them, in one project.
func (application *Window) ProjectProgress(project *workspace.Project) (viewed, total int) {
	return application.reviewed.Progress(project)
}

// ClassKind is what a Java or Kotlin file declares, once read.
func (application *Window) ClassKind(file workspace.FileReference) java.ClassKind {
	return application.classKind(file)
}

// Loading tells whether projects are being read.
func (application *Window) Loading() bool { return application.loading > 0 }

// ReadingKinds tells whether class kinds are being read.
func (application *Window) ReadingKinds() bool { return len(application.kindsWanted) > 0 }
