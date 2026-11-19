package window

import (
	"github.com/akrck02/code-review/internal/theme"
	ui "github.com/akrck02/go-ui"
)

// SidebarFolded tells whether the sidebar is folded away, to start so the next time.
func (application *Window) SidebarFolded() bool { return application.sidebarFolded }

// toggleSidebar folds the sidebar away, the file taking its room and the keyboard, or brings it
// back with the keyboard in it.
func (application *Window) toggleSidebar() {
	application.sidebarFolded = !application.sidebarFolded
	if !application.sidebarFolded {
		application.focus = sidebarPane
	} else if application.viewer.IsOpen() {
		application.focus = viewerPane
	}
	application.focusBefore = application.focus
}

// unfoldForTheKeyboard brings the folded sidebar back when the keyboard has just gone to it (Tab,
// ⌘F, Esc after editing…): what takes the keys is in sight.
func (application *Window) unfoldForTheKeyboard() {
	if application.sidebarFolded && application.focus == sidebarPane && application.focusBefore != sidebarPane {
		application.sidebarFolded = false
	}
	application.focusBefore = application.focus
}

// buildFoldButton is the rail's button that folds the sidebar away, or brings it back.
func (application *Window) buildFoldButton(context *ui.Context) {
	name, icon := "Hide the sidebar (s)", "panel-left-close"
	if application.sidebarFolded {
		name, icon = "Show the sidebar (s)", "panel-left-open"
	}
	if theme.ToolButton(context, name, icon, false) {
		application.toggleSidebar()
	}
}
