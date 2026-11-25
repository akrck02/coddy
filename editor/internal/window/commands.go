package window

import (
	"github.com/akrck02/coddy/editor/internal/finder"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/settings"
	"github.com/akrck02/coddy/editor/internal/text"
	"github.com/akrck02/coddy/editor/internal/viewer"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// place is where a command works: anywhere in the window, in the pane with the keyboard, or in a
// file being edited, where the plain keys type and only keys with Command, F-keys and Esc run commands.
type place int

const (
	anywhere place = iota
	inSidebar
	inViewer
	editing
)

// command is one thing the reader does from the keyboard: the keys that do it, what the help says
// of it, and where it works.
type command struct {
	// id names the command in a profile of keys: it stays when the description changes
	id string
	// topic is the section of the keys screen it is listed in, beside its icon
	topic       topic
	icon        string
	description string
	// owner is the name of the mode the command is kept to, "" for one that works in every mode
	owner string
	keys  []pane.Press
	place place
	// inDialogs runs the command even while a dialog is open (quitting)
	inDialogs bool
	// applies tells whether the command can act now; nil means always
	applies func(application *Window) bool
	do      func(application *Window, context *ui.Context, step int)
}

// handleKeys runs the command the frame's keys ask for: the first one, in the order of commands,
// whose key was pressed and that can act now, and tells whether one ran. While a dialog is open,
// the dialog reads the keys.
func (application *Window) handleKeys(context *ui.Context) bool {
	if application.settingsScreen.Recording() {
		return false // the keys pressed become a command's, in the settings
	}
	dialogOpen := application.ModalOpen() || context.WantsKeyboard()
	typing := application.editing()
	for _, command := range application.allCommands() {
		if dialogOpen && !command.inDialogs {
			continue
		}
		if !application.isIn(command.place) || (command.applies != nil && !command.applies(application)) {
			continue
		}
		for _, press := range command.keys {
			if typing && !press.WorksWhileEditing() {
				continue // the key types
			}
			if press.Matches(application.keys) {
				command.do(application, context, press.Step)
				return true
			}
		}
	}
	return false
}

// isIn tells whether the keyboard is in a place.
func (application *Window) isIn(where place) bool {
	switch where {
	case inSidebar:
		return application.focus == sidebarPane
	case inViewer:
		return application.focus == viewerPane && !application.editing()
	case editing:
		return application.editing()
	}
	return true
}

// editing tells whether the keyboard is in a file being edited: the whole file, in the viewer, and
// not read-only.
func (application *Window) editing() bool {
	return !application.settings.ReadOnly && application.focus == viewerPane && application.viewer.Editable()
}

// commands are every shortcut of the reader, in the order they are tried. The help lists them.
var commands = []command{
	{id: "quit", topic: topicWindow, icon: "log-out", description: "Quit", inDialogs: true, keys: []pane.Press{pane.CommandKey(input.KeyQ, "⌘Q")},
		do: func(application *Window, _ *ui.Context, _ int) { application.quit() }},
	{id: "full-screen", topic: topicWindow, icon: "maximize", description: "The whole screen, or a window again", inDialogs: true, keys: []pane.Press{pane.Key(input.KeyF11, "F11")},
		do: func(_ *Window, context *ui.Context, _ int) { context.SetFullscreen(!context.Fullscreen()) }},
	{id: "find-text-in-project", topic: topicFinding, icon: "file-text", description: "Find a text in the files of the project you are in",
		keys: []pane.Press{pane.CommandKey(input.KeyF, "⌘F")},
		do:   func(application *Window, _ *ui.Context, _ int) { application.findInProject() }},
	{id: "find-text", topic: topicFinding, icon: "text-search", description: "Find a text in the files of every project (Tab switches in the finder)",
		keys: []pane.Press{pane.CommandShiftKey(input.KeyF, "⌘⇧F")},
		do:   func(application *Window, _ *ui.Context, _ int) { application.finder.Show(finder.ContentScope) }},
	{id: "filter-sidebar", topic: topicFinding, icon: "list-filter", description: "Filter the projects and files the sidebar lists (Enter goes to them)", keys: []pane.Press{{Key: input.KeyF, Alt: true, Label: "⌥F"}},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.sidebar.FilterAsked = true
			application.focus = sidebarPane
		}},
	{id: "find-file", topic: topicFinding, icon: "file-search", description: "Find a file by its name, in any project",
		keys: []pane.Press{{Key: input.KeyShift, Twice: true, Label: "⇧⇧"}},
		do:   func(application *Window, _ *ui.Context, _ int) { application.finder.Show(finder.NamesScope) }},
	{id: "open-folder", topic: topicProjects, icon: "folder-open", description: "Open a repository or a folder of repositories",
		keys: []pane.Press{pane.CommandKey(input.KeyO, "⌘O")},
		do:   func(application *Window, _ *ui.Context, _ int) { application.OpenFolder() }},
	{id: "reload", topic: topicProjects, icon: "refresh-cw", description: "Read every project again",
		keys: []pane.Press{pane.CommandKey(input.KeyR, "⌘R")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.reloadAll()
			application.toast = "Reading the projects again"
		}},
	{id: "choose-base", topic: topicChanges, icon: "git-compare", description: "Compare against another branch, tag or commit", keys: []pane.Press{pane.CommandShiftKey(input.KeyB, "⌘⇧B")},
		do: func(application *Window, _ *ui.Context, _ int) { application.openBaseDialog() }},
	{id: "stop-editing", topic: topicEditing, icon: "pencil-off", description: "Stop editing: the sidebar takes the keyboard", place: editing,
		keys: []pane.Press{pane.AnyKey(input.KeyEscape, "Esc")},
		do:   func(application *Window, _ *ui.Context, _ int) { application.focus = sidebarPane }},
	{id: "save", topic: topicEditing, icon: "save", description: "Save", place: editing, keys: []pane.Press{pane.CommandKey(input.KeyS, "⌘S")},
		do: func(application *Window, _ *ui.Context, _ int) { application.save() }},
	{id: "undo", topic: topicEditing, icon: "undo-2", description: "Undo / redo", place: editing,
		keys: []pane.Press{{Key: input.KeyZ, Command: true, Label: "⌘Z", Step: -1}, {Key: input.KeyZ, Command: true, Shift: true, Label: "⌘⇧Z", Step: 1}},
		do: func(application *Window, _ *ui.Context, step int) {
			application.viewer.Apply(application, func(buffer *text.Buffer) {
				if step < 0 {
					buffer.Undo()
				} else {
					buffer.Redo()
				}
			})
		}},
	{id: "clipboard", topic: topicEditing, icon: "clipboard", description: "Cut, copy, paste", place: editing,
		keys: []pane.Press{{Key: input.KeyX, Command: true, Label: "⌘X", Step: cutting}, {Key: input.KeyC, Command: true, Label: "⌘C", Step: copying},
			{Key: input.KeyV, Command: true, Label: "⌘V", Step: pasting}},
		do: func(application *Window, context *ui.Context, step int) { application.clipboardEdit(context, step) }},
	{id: "select-all", topic: topicEditing, icon: "text-select", description: "Select all", place: editing, keys: []pane.Press{pane.CommandKey(input.KeyA, "⌘A")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.viewer.Apply(application, func(buffer *text.Buffer) { buffer.SelectAll() })
		}},
	{id: "leave-project", topic: topicProjects, icon: "arrow-left", description: "Back to every project", keys: []pane.Press{pane.AnyKey(input.KeyEscape, "Esc"), pane.AnyKey(input.KeyBackspace, "Backspace")},
		applies: func(application *Window) bool { return application.sidebar.Entered != "" },
		do: func(application *Window, _ *ui.Context, _ int) {
			application.sidebar.Leave(application)
			application.focus = sidebarPane
		}},
	{id: "enter-project", topic: topicProjects, icon: "log-in", description: "Enter the project: the sidebar lists only its tree", keys: []pane.Press{pane.CommandKey(input.KeyE, "⌘E")},
		do: func(application *Window, _ *ui.Context, _ int) { application.enterCurrentProject() }},
	{id: "toggle-sidebar", topic: topicWindow, icon: "panel-left", description: "Hide the sidebar, the file taking its room, or show it again",
		keys: []pane.Press{pane.CommandShiftKey(input.KeyE, "⌘⇧E")},
		do:   func(application *Window, _ *ui.Context, _ int) { application.toggleSidebar() }},
	{id: "switch-pane", topic: topicWindow, icon: "arrow-left-right", description: "Move between the sidebar and the file", keys: []pane.Press{pane.AnyKey(input.KeyTab, "Tab"), pane.CommandKey(input.KeyTab, "⌃Tab")},
		do: func(application *Window, _ *ui.Context, _ int) { application.toggleFocus() }},
	{id: "step-changed-file", topic: topicChanges, icon: "file-diff", description: "Next / previous changed file, across every project",
		keys: []pane.Press{
			// with Shift: macOS's window menu takes ⌘M to minimize
			{Key: input.KeyN, Command: true, Shift: true, Label: "⌘⇧N", Step: 1}, {Key: input.KeyM, Command: true, Shift: true, Label: "⌘⇧M", Step: -1}},
		do: func(application *Window, _ *ui.Context, step int) { application.stepChangedFile(step) }},
	{id: "step-change", topic: topicChanges, icon: "diff", description: "Next / previous change in the file",
		keys: []pane.Press{{Key: input.KeyF7, Label: "F7", Step: 1}, {Key: input.KeyF7, Shift: true, Label: "⇧F7", Step: -1}},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.StepHunk(step) }},
	{id: "toggle-diff", topic: topicChanges, icon: "git-compare-arrows", description: "Diff or the whole file", keys: []pane.Press{pane.CommandKey(input.KeyD, "⌘D")},
		do: func(application *Window, _ *ui.Context, _ int) { application.toggleView() }},
	{id: "toggle-fold", topic: topicView, icon: "fold-vertical", description: "Fold the unchanged lines, or show them all", keys: []pane.Press{pane.CommandKey(input.KeyU, "⌘U")},
		do: func(application *Window, _ *ui.Context, _ int) {
			if application.viewer.Mode != viewer.FileView {
				application.viewer.ToggleContext()
			}
		}},
	{id: "fold-all", topic: topicView, icon: "chevrons-down-up", description: "Fold every unchanged line / show them all",
		keys: []pane.Press{{Key: input.KeyMinus, Command: true, Shift: true, Label: "⌘⇧-", Step: -1}, {Key: input.KeyPlus, Command: true, Shift: true, Label: "⌘⇧+", Step: 1}},
		do: func(application *Window, _ *ui.Context, step int) {
			if application.viewer.Mode != viewer.FileView && application.viewer.Unfolded() != (step > 0) {
				application.viewer.ToggleContext()
			}
		}},
	{id: "only-changes", topic: topicChanges, icon: "filter", description: "Only the changes, projects and files, or everything (sidebar and finder)", keys: []pane.Press{pane.CommandShiftKey(input.KeyG, "⌘⇧G")},
		do: func(application *Window, _ *ui.Context, _ int) { application.setOnlyChanged(!application.onlyChanged) }},
	{id: "working-tree", topic: topicChanges, icon: "git-commit-horizontal", description: "Count the changes not committed yet, or only commits", keys: []pane.Press{pane.CommandShiftKey(input.KeyW, "⌘⇧W")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.keepingEdits(application.toggleWorkingTree)
		}},
	{id: "find-uses", topic: topicCode, icon: "list-tree", description: "Every use of the name at the caret, or clicked", keys: []pane.Press{{Key: input.KeyF7, Alt: true, Label: "⌥F7"}, {Key: input.KeyF12, Shift: true, Label: "⇧F12"}},
		applies: func(application *Window) bool { return application.viewer.IsOpen() },
		do:      func(application *Window, _ *ui.Context, _ int) { application.usesAsked() }},
	{id: "go-to-declaration", topic: topicCode, icon: "locate", description: "Go to the declaration of the name at the caret, or clicked, or ⌘-click it; on the declaration, its uses",
		keys:    []pane.Press{pane.CommandKey(input.KeyB, "⌘B"), pane.Key(input.KeyF12, "F12")},
		applies: func(application *Window) bool { return application.viewer.IsOpen() },
		do:      func(application *Window, _ *ui.Context, _ int) { application.definitionAsked() }},
	{id: "step-trail", topic: topicCode, icon: "history", description: "Back / forward, after following a name",
		keys: []pane.Press{{Key: input.KeyLeftBracket, Command: true, Label: "⌘[", Step: -1}, {Key: input.KeyRightBracket, Command: true, Label: "⌘]", Step: 1},
			{Key: input.KeyLeft, Command: true, Alt: true, Label: "⌘⌥←", Step: -1}, {Key: input.KeyRight, Command: true, Alt: true, Label: "⌘⌥→", Step: 1}},
		do: func(application *Window, _ *ui.Context, step int) { application.stepTrail(step) }},
	{id: "read-only", topic: topicEditing, icon: "lock", description: "Read-only: files are only read, or can be edited",
		keys: []pane.Press{pane.CommandShiftKey(input.KeyL, "⌘⇧L")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.setReadOnly(!application.settings.ReadOnly)
		}},
	{id: "zoom", topic: topicView, icon: "zoom-in", description: "Larger / smaller code, or its first size (or ⌘ and the wheel over the code)",
		keys: []pane.Press{{Key: input.KeyPlus, Command: true, Label: "⌘+", Step: 1}, {Key: input.KeyMinus, Command: true, Label: "⌘-", Step: -1},
			{Key: input.Key0, Command: true, Label: "⌘0", Step: 0}},
		do: func(application *Window, _ *ui.Context, step int) { application.viewer.Zoom(step) }},
	{id: "copy-path", topic: topicProjects, icon: "clipboard-copy", description: "Copy the open file's path", keys: []pane.Press{pane.CommandShiftKey(input.KeyC, "⌘⇧C")},
		do: func(application *Window, context *ui.Context, _ int) { application.copyPath(context) }},
	{id: "show-keys", topic: topicWindow, icon: "keyboard", description: "These keys", keys: []pane.Press{pane.Key(input.KeyF1, "F1")},
		do: func(application *Window, _ *ui.Context, _ int) { application.settingsScreen.Toggle(settings.Keys) }},
	{id: "show-settings", topic: topicWindow, icon: "settings", description: "The settings", keys: []pane.Press{pane.CommandKey(input.KeyComma, "⌘,")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.settingsScreen.Toggle(settings.Appearance)
		}},

	{id: "sidebar-select", topic: topicProjects, icon: "list", description: "Select the next / previous row, a page, the first / last", place: inSidebar,
		keys: []pane.Press{pane.StepKey(input.KeyDown, "↓", 1), pane.StepKey(input.KeyUp, "↑", -1),
			pane.StepKey(input.KeyPageDown, "PgDn", pane.PageStep), pane.StepKey(input.KeyPageUp, "PgUp", -pane.PageStep),
			{Key: input.KeyHome, Label: "Home", Step: -pane.EndStep}, {Key: input.KeyEnd, Label: "End", Step: pane.EndStep}},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do: func(application *Window, _ *ui.Context, step int) {
			application.sidebar.MoveSelection(application, step)
		}},
	{id: "sidebar-fold", topic: topicProjects, icon: "folder-tree", description: "Open / close a folder", place: inSidebar,
		keys:    []pane.Press{pane.StepKey(input.KeyRight, "→", 1), pane.StepKey(input.KeyLeft, "←", -1)},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do:      func(application *Window, _ *ui.Context, step int) { application.sidebar.OpenOrClose(application, step) }},
	{id: "sidebar-open", topic: topicProjects, icon: "corner-down-left", description: "Open the row: a file (Enter moves to it), a folder, or enter a project", place: inSidebar,
		keys:    []pane.Press{{Key: input.KeyEnter, Label: "Enter", Step: 1}, pane.Key(input.KeySpace, "Space")},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do: func(application *Window, _ *ui.Context, step int) {
			application.sidebar.OpenSelected(application, step == 1)
		}},

	{id: "scroll-line", topic: topicView, icon: "arrow-down-up", description: "Scroll a line down / up", place: inViewer,
		keys: []pane.Press{pane.StepKey(input.KeyDown, "↓", 1), pane.StepKey(input.KeyUp, "↑", -1)},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollLines(step) }},
	{id: "scroll-page-up", topic: topicView, icon: "chevrons-up", description: "Scroll a page up", place: inViewer,
		keys: []pane.Press{{Key: input.KeySpace, Shift: true, Label: "Shift+Space", Step: -pane.PageStep, Repeats: true},
			pane.StepKey(input.KeyPageUp, "PgUp", -pane.PageStep)},
		do: func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollLines(step) }},
	{id: "scroll-page-down", topic: topicView, icon: "chevrons-down", description: "Scroll a page down", place: inViewer,
		keys: []pane.Press{pane.StepKey(input.KeySpace, "Space", pane.PageStep), pane.StepKey(input.KeyPageDown, "PgDn", pane.PageStep)},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollLines(step) }},
	{id: "scroll-end", topic: topicView, icon: "arrow-up-to-line", description: "Go to the top / bottom", place: inViewer,
		keys: []pane.Press{{Key: input.KeyHome, Label: "Home", Step: -pane.EndStep}, {Key: input.KeyEnd, Label: "End", Step: pane.EndStep}},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollLines(step) }},
	{id: "scroll-sideways", topic: topicView, icon: "arrow-right-left", description: "Scroll sideways", place: inViewer,
		keys: []pane.Press{pane.StepKey(input.KeyRight, "→", 1), pane.StepKey(input.KeyLeft, "←", -1)},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollSideways(step) }},
	{id: "open-fold", topic: topicView, icon: "unfold-vertical", description: "Open a fold in sight", place: inViewer, keys: []pane.Press{pane.Key(input.KeyEnter, "Enter")},
		do: func(application *Window, _ *ui.Context, _ int) { application.viewer.OpenFoldInSight() }},
}

// inputTap keeps the frame's input for the program's own shortcuts and painted views. go-ui's
// window hands its driver the input state before the interface runs, and that is the only place
// a program built with window.Run can see the keys itself.
type inputTap struct {
	state *input.State
}

func (tap *inputTap) Drive(state *input.State, context *ui.Context) { tap.state = state }
func (tap *inputTap) Show(drawing *draw2d.List)                     {}
