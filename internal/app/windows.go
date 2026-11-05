package app

import (
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
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
