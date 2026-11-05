package app

import (
	"context"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	ui "github.com/akrck02/go-ui"
)

// What the panes ask of the window they sit in. Each pane declares, as an interface, the little it
// needs; App answers all of them.

// Look is how the window is painted now, light or dark.
func (application *App) Look() theme.Look { return application.look }

// Keys is what the keyboard and mouse did in this frame.
func (application *App) Keys() pane.Keys { return application.keys }

// HasProjects tells whether any repository is open.
func (application *App) HasProjects() bool { return len(application.projects) > 0 }

// OpenFolder asks for a folder of repositories to open.
func (application *App) OpenFolder() { application.pickerOpen = true }

// FocusViewer gives the keyboard to the file's view.
func (application *App) FocusViewer() { application.focus = editorPane }

// BuildInsteadOfAFile shows the review's overview while no file is open.
func (application *App) BuildInsteadOfAFile(context *ui.Context) bool {
	if application.sidebar.mode != reviewMode || len(application.changedOrder) == 0 {
		return false
	}
	application.buildOverview(context)
	return true
}

// Projects are the repositories open, in the order the sidebar lists them.
func (application *App) Projects() []*workspace.Project { return application.projects }

// Candidates are every file the finder can find by name.
func (application *App) Candidates() []finder.Candidate { return application.candidates }

// Searchable tells whether the finder's content search looks in a project.
func (application *App) Searchable(index int) bool {
	project := application.projects[index]
	return project.Loaded && !application.hidden(project) && application.inScope(workspace.FileReference{Project: index})
}

// WorkingTree tells whether the changes not committed yet count.
func (application *App) WorkingTree() bool { return application.settings.WorkingTree }

// Synchronous tells whether work runs at once, for a capture or a test.
func (application *App) Synchronous() bool { return application.settings.Synchronous }

// Run starts background work under a key, replacing the work under that key still running.
func (application *App) Run(key string, do func(stop context.Context) func()) {
	application.work.run(key, do)
}

// Cancel stops the work under a key, if any.
func (application *App) Cancel(key string) { application.work.cancel(key) }

// Read reads a repository once one of the few readers is free.
func (application *App) Read(stop context.Context, reading func()) bool {
	return application.work.read(stop, reading)
}

// OpenAt opens a file at a line, shows it in the sidebar and gives it the keyboard.
func (application *App) OpenAt(file workspace.FileReference, line int) {
	application.openFileAt(file, line)
	application.sidebar.revealFile(application, file)
	application.focus = editorPane
}
