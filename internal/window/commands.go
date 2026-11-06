package window

import (
	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/text"
	"github.com/akrck02/code-review/internal/viewer"
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
	description string
	keys        []pane.Press
	place       place
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
	{description: "Quit", inDialogs: true, keys: []pane.Press{pane.CommandKey(input.KeyQ, "⌘Q")},
		do: func(application *Window, _ *ui.Context, _ int) { application.quit() }},
	{description: "Find a text in the files of every project (Tab switches in the finder)",
		keys: []pane.Press{pane.CommandShiftKey(input.KeyF, "⌘⇧F"), {Text: "/", Label: "/"}},
		do:   func(application *Window, _ *ui.Context, _ int) { application.finder.Show(finder.ContentScope) }},
	{description: "Find a file by its name, in any project",
		keys: []pane.Press{pane.CommandKey(input.KeyP, "⌘P"), {Key: input.KeyP, Shift: true, Label: "Shift+P"}},
		do:   func(application *Window, _ *ui.Context, _ int) { application.finder.Show(finder.NamesScope) }},
	{description: "Open a repository or a folder of repositories",
		keys: []pane.Press{pane.Key(input.KeyO, "o"), pane.CommandKey(input.KeyO, "⌘O")},
		do:   func(application *Window, _ *ui.Context, _ int) { application.pickerOpen = true }},
	{description: "Read every project again",
		keys: []pane.Press{pane.Key(input.KeyR, "r"), pane.CommandKey(input.KeyR, "⌘R")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.reloadAll()
			application.toast = "Reading the projects again"
		}},
	{description: "Compare against another branch, tag or commit", keys: []pane.Press{pane.Key(input.KeyB, "b"), pane.CommandKey(input.KeyB, "⌘B")},
		do: func(application *Window, _ *ui.Context, _ int) { application.openBaseDialog() }},
	{description: "Stop editing: the sidebar takes the keyboard", place: editing,
		keys: []pane.Press{pane.AnyKey(input.KeyEscape, "Esc")},
		do:   func(application *Window, _ *ui.Context, _ int) { application.focus = sidebarPane }},
	{description: "Save", place: editing, keys: []pane.Press{pane.CommandKey(input.KeyS, "⌘S")},
		do: func(application *Window, _ *ui.Context, _ int) { application.save() }},
	{description: "Undo / redo", place: editing,
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
	{description: "Cut, copy, paste", place: editing,
		keys: []pane.Press{{Key: input.KeyX, Command: true, Label: "⌘X", Step: cutting}, {Key: input.KeyC, Command: true, Label: "⌘C", Step: copying},
			{Key: input.KeyV, Command: true, Label: "⌘V", Step: pasting}},
		do: func(application *Window, context *ui.Context, step int) { application.clipboardEdit(context, step) }},
	{description: "Select all", place: editing, keys: []pane.Press{pane.CommandKey(input.KeyA, "⌘A")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.viewer.Apply(application, func(buffer *text.Buffer) { buffer.SelectAll() })
		}},
	{description: "Back to every project", keys: []pane.Press{pane.AnyKey(input.KeyEscape, "Esc"), pane.AnyKey(input.KeyBackspace, "Backspace")},
		applies: func(application *Window) bool { return application.sidebar.Entered != "" },
		do: func(application *Window, _ *ui.Context, _ int) {
			application.sidebar.Leave(application)
			application.focus = sidebarPane
		}},
	{description: "Enter the project: the sidebar lists only its tree", keys: []pane.Press{pane.Key(input.KeyE, "e"), pane.CommandKey(input.KeyE, "⌘E")},
		do: func(application *Window, _ *ui.Context, _ int) { application.enterCurrentProject() }},
	{description: "Move between the sidebar and the file", keys: []pane.Press{pane.AnyKey(input.KeyTab, "Tab"), pane.CommandKey(input.KeyTab, "⌃Tab")},
		do: func(application *Window, _ *ui.Context, _ int) { application.toggleFocus() }},
	{description: "Next / previous changed file, across every project",
		keys: []pane.Press{{Key: input.KeyN, Label: "n", Step: 1}, {Key: input.KeyM, Label: "m", Step: -1},
			// with Shift: macOS's window menu takes ⌘M to minimize
			{Key: input.KeyN, Command: true, Shift: true, Label: "⌘⇧N", Step: 1}, {Key: input.KeyM, Command: true, Shift: true, Label: "⌘⇧M", Step: -1}},
		do: func(application *Window, _ *ui.Context, step int) { application.stepChangedFile(step) }},
	{description: "Next / previous change in the file",
		keys: []pane.Press{{Key: input.KeyRightBracket, Label: "]", Step: 1, AnyModifiers: true}, {Key: input.KeyLeftBracket, Label: "[", Step: -1, AnyModifiers: true},
			{Key: input.KeyRightBracket, Command: true, Label: "⌘]", Step: 1}, {Key: input.KeyLeftBracket, Command: true, Label: "⌘[", Step: -1}},
		do: func(application *Window, _ *ui.Context, step int) { application.viewer.StepHunk(step) }},
	{description: "Diff or the whole file", keys: []pane.Press{pane.Key(input.KeyD, "d"), pane.CommandKey(input.KeyD, "⌘D")},
		do: func(application *Window, _ *ui.Context, _ int) { application.toggleView() }},
	{description: "Fold the unchanged lines, or show them all", keys: []pane.Press{pane.Key(input.KeyF, "f"), pane.CommandKey(input.KeyU, "⌘U")},
		do: func(application *Window, _ *ui.Context, _ int) {
			if application.viewer.Mode == viewer.DiffView {
				application.viewer.ToggleContext()
			}
		}},
	{description: "Only the projects with changes, or every project (sidebar and finder)", keys: []pane.Press{pane.Key(input.KeyH, "h"), pane.CommandShiftKey(input.KeyH, "⌘⇧H")},
		do: func(application *Window, _ *ui.Context, _ int) { application.setOnlyChanged(!application.onlyChanged) }},
	{description: "Count the changes not committed yet, or only commits", keys: []pane.Press{pane.Key(input.KeyW, "w"), pane.CommandShiftKey(input.KeyW, "⌘⇧W")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.keepingEdits(application.toggleWorkingTree)
		}},
	{description: "Every use of the name at the caret, or clicked", keys: []pane.Press{{Key: input.KeyF12, Shift: true, Label: "⇧F12"}},
		applies: func(application *Window) bool { return application.viewer.IsOpen() },
		do:      func(application *Window, _ *ui.Context, _ int) { application.usesAsked() }},
	{description: "Go to the declaration of the name at the caret, or clicked; or ⌘-click it", keys: []pane.Press{pane.Key(input.KeyF12, "F12")},
		applies: func(application *Window) bool { return application.viewer.IsOpen() },
		do:      func(application *Window, _ *ui.Context, _ int) { application.definitionAsked() }},
	{description: "Back / forward, after following a name",
		keys: []pane.Press{{Key: input.KeyMinus, Command: true, Label: "⌃-", Step: -1}, {Key: input.KeyMinus, Command: true, Shift: true, Label: "⌃⇧-", Step: 1}},
		do:   func(application *Window, _ *ui.Context, step int) { application.stepTrail(step) }},
	{description: "Read-only: files are only read, or can be edited",
		keys: []pane.Press{pane.Key(input.KeyL, "l"), pane.CommandShiftKey(input.KeyL, "⌘⇧L")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.setReadOnly(!application.settings.ReadOnly)
		}},
	{description: "Light or dark", keys: []pane.Press{pane.Key(input.KeyT, "t"), pane.CommandShiftKey(input.KeyT, "⌘⇧T")},
		do: func(application *Window, context *ui.Context, _ int) {
			application.dark = !application.dark
			application.applyTheme(context)
		}},
	{description: "Copy the open file's path", keys: []pane.Press{pane.Key(input.KeyY, "y"), pane.CommandShiftKey(input.KeyC, "⌘⇧C")},
		do: func(application *Window, context *ui.Context, _ int) { application.copyPath(context) }},
	{description: "These keys", keys: []pane.Press{{Text: "?", Label: "?"}, pane.Key(input.KeyF1, "F1")},
		do: func(application *Window, _ *ui.Context, _ int) { application.helpOpen = true }},

	{description: "Select the next / previous row, a page, the first / last", place: inSidebar,
		keys: []pane.Press{pane.StepKey(input.KeyDown, "↓", 1), pane.StepKey(input.KeyUp, "↑", -1),
			pane.StepKey(input.KeyPageDown, "PgDn", pane.PageStep), pane.StepKey(input.KeyPageUp, "PgUp", -pane.PageStep),
			{Key: input.KeyHome, Label: "Home", Step: -pane.EndStep}, {Key: input.KeyEnd, Label: "End", Step: pane.EndStep}},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do: func(application *Window, _ *ui.Context, step int) {
			application.sidebar.MoveSelection(application, step)
		}},
	{description: "Open / close a folder", place: inSidebar,
		keys:    []pane.Press{pane.StepKey(input.KeyRight, "→", 1), pane.StepKey(input.KeyLeft, "←", -1)},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do:      func(application *Window, _ *ui.Context, step int) { application.sidebar.OpenOrClose(application, step) }},
	{description: "Open the row: a file (Enter moves to it), a folder, or enter a project", place: inSidebar,
		keys:    []pane.Press{{Key: input.KeyEnter, Label: "Enter", Step: 1}, pane.Key(input.KeySpace, "Space")},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do: func(application *Window, _ *ui.Context, step int) {
			application.sidebar.OpenSelected(application, step == 1)
		}},

	{description: "Scroll a line down / up", place: inViewer,
		keys: []pane.Press{pane.StepKey(input.KeyDown, "↓", 1), pane.StepKey(input.KeyJ, "j", 1), pane.StepKey(input.KeyUp, "↑", -1), pane.StepKey(input.KeyK, "k", -1)},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollLines(step) }},
	{description: "Scroll a page up", place: inViewer,
		keys: []pane.Press{{Key: input.KeySpace, Shift: true, Label: "Shift+Space", Step: -pane.PageStep, Repeats: true},
			pane.StepKey(input.KeyPageUp, "PgUp", -pane.PageStep)},
		do: func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollLines(step) }},
	{description: "Scroll a page down", place: inViewer,
		keys: []pane.Press{pane.StepKey(input.KeySpace, "Space", pane.PageStep), pane.StepKey(input.KeyPageDown, "PgDn", pane.PageStep)},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollLines(step) }},
	{description: "Go to the top / bottom", place: inViewer,
		keys: []pane.Press{{Key: input.KeyHome, Label: "Home", Step: -pane.EndStep}, {Key: input.KeyEnd, Label: "End", Step: pane.EndStep}},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollLines(step) }},
	{description: "Scroll sideways", place: inViewer,
		keys: []pane.Press{pane.StepKey(input.KeyRight, "→", 1), pane.StepKey(input.KeyLeft, "←", -1)},
		do:   func(application *Window, _ *ui.Context, step int) { application.viewer.ScrollSideways(step) }},
	{description: "Open a fold in sight", place: inViewer, keys: []pane.Press{pane.Key(input.KeyEnter, "Enter")},
		do: func(application *Window, _ *ui.Context, _ int) { application.viewer.OpenFoldInSight() }},
}

// helpRow is a command as the help shows it: its keys and what it does.
type helpRow struct {
	keys        []string
	description string
}

// helpRows lists the commands of a place, of those given, for the help.
func helpRows(given []command, where place) []helpRow {
	var rows []helpRow
	for _, command := range given {
		if command.place != where {
			continue
		}
		labels := make([]string, len(command.keys))
		for index, press := range command.keys {
			labels[index] = press.Label
		}
		rows = append(rows, helpRow{labels, command.description})
	}
	return rows
}

// editingKeyRows are the keys the file view answers itself while it is edited, for the help.
func editingKeyRows() []helpRow {
	rows := make([]helpRow, len(viewer.EditingKeys))
	for index, key := range viewer.EditingKeys {
		rows[index] = helpRow{key.Keys, key.Description}
	}
	return rows
}
