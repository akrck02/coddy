package window

import (
	"strings"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
	platform "github.com/akrck02/go-ui/window"
)

// place is where a command works: anywhere in the window, or in the pane with the keyboard.
type place int

const (
	anywhere place = iota
	inSidebar
	inEditor
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
// whose key was pressed and that can act now. While a dialog is open, the dialog reads the keys.
func (application *Window) handleKeys(context *ui.Context) {
	dialogOpen := application.ModalOpen() || context.WantsKeyboard()
	for _, command := range commands {
		if dialogOpen && !command.inDialogs {
			continue
		}
		if !application.isIn(command.place) || (command.applies != nil && !command.applies(application)) {
			continue
		}
		for _, press := range command.keys {
			if press.Matches(application.keys) {
				command.do(application, context, press.Step)
				return
			}
		}
	}
}

// isIn tells whether the keyboard is in a place.
func (application *Window) isIn(where place) bool {
	switch where {
	case inSidebar:
		return application.focus == sidebarPane
	case inEditor:
		return application.focus == editorPane
	}
	return true
}

// commands are every shortcut of the reader, in the order they are tried. The help lists them.
var commands = []command{
	{description: "Quit", inDialogs: true, keys: []pane.Press{pane.CommandKey(input.KeyQ, "⌘Q")},
		do: func(*Window, *ui.Context, int) { platform.Quit() }},
	{description: "Find a text in the files of every project (Tab switches in the finder)",
		keys: []pane.Press{{Key: input.KeyF, Command: true, Shift: true, Label: "⌘⇧F"}, {Text: "/", Label: "/"}},
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
	{description: "Compare against another branch, tag or commit", keys: []pane.Press{pane.Key(input.KeyB, "b")},
		do: func(application *Window, _ *ui.Context, _ int) { application.openBaseDialog() }},
	{description: "Back to every project", keys: []pane.Press{pane.Key(input.KeyEscape, "Esc"), pane.Key(input.KeyBackspace, "Backspace")},
		applies: func(application *Window) bool { return application.sidebar.Entered != "" },
		do: func(application *Window, _ *ui.Context, _ int) {
			application.sidebar.Leave(application)
			application.focus = sidebarPane
		}},
	{description: "Files mode: only the changed files, or every file", keys: []pane.Press{pane.Key(input.KeyG, "g")},
		do: func(application *Window, _ *ui.Context, _ int) { application.sidebar.ToggleChangedFilter() }},
	{description: "Enter the project: the sidebar lists only its tree", keys: []pane.Press{pane.Key(input.KeyE, "e")},
		do: func(application *Window, _ *ui.Context, _ int) { application.enterCurrentProject() }},
	{description: "Move between the sidebar and the editor", keys: []pane.Press{pane.Key(input.KeyTab, "Tab")},
		do: func(application *Window, _ *ui.Context, _ int) { application.toggleFocus() }},
	{description: "Next / previous changed file, across every project",
		keys: []pane.Press{{Key: input.KeyN, Label: "n", Step: 1}, {Key: input.KeyM, Label: "m", Step: -1}},
		do:   func(application *Window, _ *ui.Context, step int) { application.stepChangedFile(step) }},
	{description: "Next / previous change in the file",
		keys: []pane.Press{{Key: input.KeyRightBracket, Label: "]", Step: 1}, {Key: input.KeyLeftBracket, Label: "[", Step: -1}},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.StepHunk(step) }},
	{description: "Diff or the whole file", keys: []pane.Press{pane.Key(input.KeyD, "d")},
		do: func(application *Window, _ *ui.Context, _ int) { application.toggleView() }},
	{description: "Fold the unchanged lines, or show them all", keys: []pane.Press{pane.Key(input.KeyF, "f")},
		do: func(application *Window, _ *ui.Context, _ int) {
			if application.editor.Mode == viewer.DiffView {
				application.editor.ToggleContext()
			}
		}},
	{description: "Files mode (read any file) / Review mode (the changes, as diffs) / the other one",
		keys: []pane.Press{{Key: input.Key1, Label: "1", Step: 1}, {Key: input.Key2, Label: "2", Step: 2}, pane.Key(input.KeyC, "c")},
		do: func(application *Window, _ *ui.Context, step int) {
			switch step {
			case 1:
				application.setMode(sidebar.FilesMode)
			case 2:
				application.setMode(sidebar.ReviewMode)
			default:
				application.setMode(1 - application.sidebar.Mode)
			}
		}},
	{description: "Mark the file as viewed and open the next one not viewed", keys: []pane.Press{pane.Key(input.KeyV, "v")},
		do: func(application *Window, _ *ui.Context, _ int) {
			if reference, ok := application.markTarget(); ok {
				application.ToggleViewed(reference, application.focus == editorPane || application.editor.Reference == reference)
			}
		}},
	{description: "Only the projects with changes, or every project (sidebar and finder)", keys: []pane.Press{pane.Key(input.KeyH, "h")},
		do: func(application *Window, _ *ui.Context, _ int) { application.setOnlyChanged(!application.onlyChanged) }},
	{description: "Count the changes not committed yet, or only commits", keys: []pane.Press{pane.Key(input.KeyW, "w")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.settings.WorkingTree = !application.settings.WorkingTree
			application.clearDocuments()
			application.reloadAll()
			application.toast = workingTreeNote(application.settings.WorkingTree)
		}},
	{description: "Light or dark", keys: []pane.Press{pane.Key(input.KeyT, "t")},
		do: func(application *Window, context *ui.Context, _ int) {
			application.dark = !application.dark
			application.applyTheme(context)
		}},
	{description: "Copy the open file's path", keys: []pane.Press{pane.Key(input.KeyY, "y")},
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

	{description: "Scroll a line down / up", place: inEditor,
		keys: []pane.Press{pane.StepKey(input.KeyDown, "↓", 1), pane.StepKey(input.KeyJ, "j", 1), pane.StepKey(input.KeyUp, "↑", -1), pane.StepKey(input.KeyK, "k", -1)},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.ScrollLines(step) }},
	{description: "Scroll a page up", place: inEditor,
		keys: []pane.Press{{Key: input.KeySpace, Shift: true, Label: "Shift+Space", Step: -pane.PageStep, Repeats: true},
			pane.StepKey(input.KeyPageUp, "PgUp", -pane.PageStep)},
		do: func(application *Window, _ *ui.Context, step int) { application.editor.ScrollLines(step) }},
	{description: "Scroll a page down", place: inEditor,
		keys: []pane.Press{pane.StepKey(input.KeySpace, "Space", pane.PageStep), pane.StepKey(input.KeyPageDown, "PgDn", pane.PageStep)},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.ScrollLines(step) }},
	{description: "Go to the top / bottom", place: inEditor,
		keys: []pane.Press{{Key: input.KeyHome, Label: "Home", Step: -pane.EndStep}, {Key: input.KeyEnd, Label: "End", Step: pane.EndStep}},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.ScrollLines(step) }},
	{description: "Scroll sideways", place: inEditor,
		keys: []pane.Press{pane.StepKey(input.KeyRight, "→", 1), pane.StepKey(input.KeyLeft, "←", -1)},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.ScrollSideways(step) }},
	{description: "Open a fold in sight", place: inEditor, keys: []pane.Press{pane.Key(input.KeyEnter, "Enter")},
		do: func(application *Window, _ *ui.Context, _ int) { application.editor.OpenFoldInSight() }},
}

// helpRow is a command as the help shows it: its keys and what it does.
type helpRow struct{ keys, description string }

// helpRows lists the commands of a place for the help.
func helpRows(where place) []helpRow {
	var rows []helpRow
	for _, command := range commands {
		if command.place != where {
			continue
		}
		labels := make([]string, len(command.keys))
		for index, press := range command.keys {
			labels[index] = press.Label
		}
		rows = append(rows, helpRow{strings.Join(labels, "  "), command.description})
	}
	return rows
}
