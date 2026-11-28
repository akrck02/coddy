package window

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/java"
	"github.com/akrck02/coddy/editor/internal/ui/controls"
	"github.com/akrck02/coddy/editor/internal/ui/finder"
	"github.com/akrck02/coddy/editor/internal/ui/pane"
	"github.com/akrck02/coddy/editor/internal/ui/sidebar"
	"github.com/akrck02/coddy/editor/internal/ui/theme"
	"github.com/akrck02/coddy/editor/internal/ui/viewer"
	ui "github.com/akrck02/go-ui"
)

// The window has every power the panes may look for: a name changed on either side would
// otherwise leave the power out without a word.
var (
	_ viewer.Marker       = (*Window)(nil)
	_ viewer.Navigator    = (*Window)(nil)
	_ viewer.Page         = (*Window)(nil)
	_ sidebar.ClassReader = (*Window)(nil)
	_ sidebar.ServerFiles = (*Window)(nil)
	_ sidebar.LeftOuter   = (*Window)(nil)
)

// What the panes ask of the window they sit in. Each pane declares, as an interface, the little it
// needs; Window answers all of them.

// Look is how the window is painted now, light or dark.
func (application *Window) Look() theme.Look { return application.look }

// Keys is what the keyboard and mouse did in this frame.
func (application *Window) Keys() pane.Keys { return application.keys }

// HasProjects tells whether any repository is open.
func (application *Window) HasProjects() bool { return len(application.workspace.Projects) > 0 }

// The system's folder chooser: its title, the job that waits for it, and what Coddy says where
// the computer has none.
const (
	openFolderTitle = "Open a folder"
	chooseFolderJob = "choose a folder"
	noFolderChooser = "This computer offers no folder chooser: give the folders to coddy when it starts"
)

// OpenFolder asks for a folder of repositories to open, with the system's own chooser, starting
// in the folder opened last; with no chooser, it says how else to open one.
func (application *Window) OpenFolder() {
	choose := application.settings.ChooseFolder
	start := ""
	if folders := application.workspace.Folders; len(folders) > 0 {
		start = folders[len(folders)-1]
	}
	application.Run(chooseFolderJob, func(stop context.Context) func() {
		answer := make(chan string, 1)
		if choose == nil || !choose(openFolderTitle, start, func(folder string) { answer <- folder }) {
			return func() { application.toast = noFolderChooser }
		}
		select {
		case folder := <-answer:
			if folder == "" {
				return nil // closed without a folder
			}
			return func() { application.openChosenFolder(folder) }
		case <-stop.Done():
			return nil
		}
	})
}

// openChosenFolder makes a folder chosen in the system's chooser the workspace, in place of the
// one open: the folder itself when it is a repository or holds none, else the repositories in it.
func (application *Window) openChosenFolder(folder string) {
	application.keepingEdits(func() {
		application.workspace.Clear()
		application.openFolders([]string{folder})
	})
}

// FocusViewer gives the keyboard to the file's view.
func (application *Window) FocusViewer() { application.focus = viewerPane }

// BuildInsteadOfAFile shows what the mode that is on shows while no file is open, if anything.
func (application *Window) BuildInsteadOfAFile(context *ui.Context) bool {
	page := application.powers.page
	return page != nil && page.BuildInsteadOfAFile(context)
}

// Projects are the repositories open, in the order the sidebar lists them.
func (application *Window) Projects() []*codebase.Project { return application.workspace.Projects }

// Candidates are every file the finder can find by name.
func (application *Window) Candidates() []finder.Candidate { return application.candidates }

// Searchable tells whether the finder's content search looks in a project.
func (application *Window) Searchable(index int) bool {
	project := application.workspace.Projects[index]
	return project.Loaded && !application.hidden(project) && application.inScope(codebase.FileReference{Project: index})
}

// WorkingTree tells whether the changes not committed yet count.
func (application *Window) WorkingTree() bool { return application.workspace.WorkingTree }

// Synchronous tells whether work runs at once, for a capture or a test.
func (application *Window) Synchronous() bool { return application.settings.Synchronous }

// Run starts background work under a key, replacing the work under that key still running.
func (application *Window) Run(key string, do func(stop context.Context) func()) {
	application.work.Run(key, do)
}

// Cancel stops the work under a key, if any.
func (application *Window) Cancel(key string) { application.work.Cancel(key) }

// Read reads a repository once one of the few readers is free.
func (application *Window) Read(stop context.Context, reading func()) bool {
	return application.work.Read(stop, reading)
}

// OpenAt opens a file at a line, shows it in the sidebar and gives it the keyboard.
func (application *Window) OpenAt(file codebase.FileReference, line int) {
	application.openFileAt(file, line)
	application.sidebar.RevealFile(application, file)
	application.focus = viewerPane
}

// ProjectIndex is a project's place in Projects, by its path; -1 when it is not open.
func (application *Window) ProjectIndex(path string) int { return application.projectIndex(path) }

// Hidden tells whether a project is left out of the lists.
func (application *Window) Hidden(project *codebase.Project) bool {
	return application.hidden(project)
}

// ShowEveryProject shows the projects with no changes again.
func (application *Window) ShowEveryProject() { application.setOnlyChanged(false) }

// OpenFile shows a file in the viewer.
func (application *Window) OpenFile(file codebase.FileReference) { application.openFile(file) }

// OpenedFile is the file the viewer shows, if any.
func (application *Window) OpenedFile() (codebase.FileReference, bool) {
	return application.viewer.Reference, application.viewer.IsOpen()
}

// IsOpened tells whether the viewer shows a file.
func (application *Window) IsOpened(file codebase.FileReference) bool {
	return application.viewer.IsOpen() && application.viewer.Reference == file
}

// FocusSidebar gives the keyboard to the sidebar.
func (application *Window) FocusSidebar() { application.focus = sidebarPane }

// SidebarFocused tells whether the sidebar has the keyboard.
func (application *Window) SidebarFocused() bool { return application.focus == sidebarPane }

// ClassKind is what a Java or Kotlin file declares, once read.
func (application *Window) ClassKind(file codebase.FileReference) java.ClassKind {
	return application.classKind(file)
}

// Loading tells whether projects are being read.
func (application *Window) Loading() bool { return application.loading > 0 }

// ReadingKinds tells whether class kinds are being read.
func (application *Window) ReadingKinds() bool { return len(application.kindsWanted) > 0 }

// FileButton is the viewer header's button for a file: the one of the mode that is on, and none
// when that mode has no button.
func (application *Window) FileButton(file codebase.FileReference) (viewer.Button, bool) {
	if owner := application.powers.button; owner != nil {
		return owner.FileButton(file)
	}
	return viewer.Button{}, false
}

// PressFileButton presses the header's button for the file open, moving on as a reviewer would.
func (application *Window) PressFileButton(file codebase.FileReference) {
	owner := application.powers.button
	if owner == nil {
		return
	}
	if _, shown := owner.FileButton(file); shown {
		owner.Press(file, true)
	}
}

// ListEveryFile turns to the first mode that lists every file. It is asked by the sidebar while
// it reveals a file, so it only changes what the sidebar lists; the sidebar goes on revealing.
func (application *Window) ListEveryFile() {
	for _, each := range application.modes {
		if !each.ChangesOnly() {
			application.turnOn(each)
			application.sidebar.Stale = true
			return
		}
	}
}

// ChangedFiles are the files the development changed, in the order n goes through them.
func (application *Window) ChangedFiles() []codebase.FileReference { return application.changedOrder }

// Shows tells whether a file is in the project entered, if any.
func (application *Window) Shows(file codebase.FileReference) bool { return application.inScope(file) }

// Reveal opens a file and shows it in the sidebar.
func (application *Window) Reveal(file codebase.FileReference) {
	application.openFile(file)
	application.sidebar.RevealFile(application, file)
}

// ExploreFolder shows, in the viewer's place, a hint to choose a file of a folder chosen in the
// sidebar, "" a project's top.
func (application *Window) ExploreFolder(project int, folder string) {
	if project < len(application.workspace.Projects) {
		application.folder.show(project, application.workspace.Projects[project].Name, folder)
	}
}

// RevealFolder shows a folder of a project in the sidebar, with the keyboard, and brings the
// sidebar back if it was folded away.
func (application *Window) RevealFolder(project int, folder string) {
	application.sidebar.RevealFolder(application, project, folder)
	application.focus = sidebarPane
}

// Target is the file a key acts on: the one selected in the sidebar while it has the keyboard,
// else the one open. inViewer tells whether it is the one open, or the viewer has the keyboard.
func (application *Window) Target() (file codebase.FileReference, inViewer bool, found bool) {
	switch {
	case application.focus == sidebarPane:
		if row, ok := application.sidebar.SelectedRow(); ok && row.Kind == sidebar.FileRow {
			file, found = codebase.FileReference{Project: row.Project, Path: row.Path}, true
		} else if application.viewer.IsOpen() {
			file, found = application.viewer.Reference, true
		}
	case application.viewer.IsOpen():
		file, found = application.viewer.Reference, true
	}
	opened := application.viewer.IsOpen() && application.viewer.Reference == file
	return file, found && (application.focus == viewerPane || opened), found
}

// ToggleChangedFilter shows only the changes, projects and files, or everything again.
func (application *Window) ToggleChangedFilter() {
	application.setOnlyChanged(!application.onlyChanged)
}

// Toast shows a short note at the bottom of the window.
func (application *Window) Toast(words string) { application.toast = words }

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
	name, icon := "Hide the sidebar (⌘⇧E)", "panel-left-close"
	if application.sidebarFolded {
		name, icon = "Show the sidebar (⌘⇧E)", "panel-left-open"
	}
	if controls.ToolButton(context, name, icon, false) {
		application.toggleSidebar()
	}
}
