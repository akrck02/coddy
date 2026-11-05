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

// keyPress is a key, or a character typed, that runs a command; step tells the command which way
// to go when several keys share it (n is +1, m is −1).
type keyPress struct {
	key input.Key
	// text is a character typed instead of a key: for / and ?, which sit on different keys in
	// different keyboard layouts
	text    string
	command bool // with Command, or Control off a Mac
	shift   bool
	// repeats runs the command again while the key is held down, as moving through rows does
	repeats bool
	step    int
	label   string // how the help writes it
}

// command is one thing the reader does from the keyboard: the keys that do it, what the help says
// of it, and where it works.
type command struct {
	description string
	keys        []keyPress
	place       place
	// inDialogs runs the command even while a dialog is open (quitting)
	inDialogs bool
	// applies tells whether the command can act now; nil means always
	applies func(application *Window) bool
	do      func(application *Window, context *ui.Context, step int)
}

// matches tells which of a command's keys the frame's input pressed.
func (press keyPress) matches(keys pane.Keys) bool {
	switch {
	case press.text != "":
		return strings.Contains(keys.TypedText(), press.text)
	case press.command:
		return keys.Command() && keys.Pressed(press.key) && (!press.shift || keys.Shift())
	case press.shift && !keys.Shift():
		return false
	case press.repeats:
		return keys.Typed(press.key) && !keys.Command() && !keys.Down(input.KeyAlt)
	default:
		return keys.Plain(press.key)
	}
}

// key is a plain key with its label.
func key(code input.Key, label string) keyPress { return keyPress{key: code, label: label} }

// stepKey is a plain key that moves, by step, and repeats while it is held.
func stepKey(code input.Key, label string, step int) keyPress {
	return keyPress{key: code, label: label, step: step, repeats: true}
}

// commandKey is a key pressed with Command.
func commandKey(code input.Key, label string) keyPress {
	return keyPress{key: code, label: label, command: true}
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
			if press.matches(application.keys) {
				command.do(application, context, press.step)
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
	{description: "Quit", inDialogs: true, keys: []keyPress{commandKey(input.KeyQ, "⌘Q")},
		do: func(*Window, *ui.Context, int) { platform.Quit() }},
	{description: "Find a text in the files of every project (Tab switches in the finder)",
		keys: []keyPress{{key: input.KeyF, command: true, shift: true, label: "⌘⇧F"}, {text: "/", label: "/"}},
		do:   func(application *Window, _ *ui.Context, _ int) { application.finder.Show(finder.ContentScope) }},
	{description: "Find a file by its name, in any project",
		keys: []keyPress{commandKey(input.KeyP, "⌘P"), {key: input.KeyP, shift: true, label: "Shift+P"}},
		do:   func(application *Window, _ *ui.Context, _ int) { application.finder.Show(finder.NamesScope) }},
	{description: "Open a repository or a folder of repositories",
		keys: []keyPress{key(input.KeyO, "o"), commandKey(input.KeyO, "⌘O")},
		do:   func(application *Window, _ *ui.Context, _ int) { application.pickerOpen = true }},
	{description: "Read every project again",
		keys: []keyPress{key(input.KeyR, "r"), commandKey(input.KeyR, "⌘R")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.reloadAll()
			application.toast = "Reading the projects again"
		}},
	{description: "Compare against another branch, tag or commit", keys: []keyPress{key(input.KeyB, "b")},
		do: func(application *Window, _ *ui.Context, _ int) { application.openBaseDialog() }},
	{description: "Back to every project", keys: []keyPress{key(input.KeyEscape, "Esc"), key(input.KeyBackspace, "Backspace")},
		applies: func(application *Window) bool { return application.sidebar.Entered != "" },
		do: func(application *Window, _ *ui.Context, _ int) {
			application.sidebar.Leave(application)
			application.focus = sidebarPane
		}},
	{description: "Files mode: only the changed files, or every file", keys: []keyPress{key(input.KeyG, "g")},
		do: func(application *Window, _ *ui.Context, _ int) { application.sidebar.ToggleChangedFilter() }},
	{description: "Enter the project: the sidebar lists only its tree", keys: []keyPress{key(input.KeyE, "e")},
		do: func(application *Window, _ *ui.Context, _ int) { application.enterCurrentProject() }},
	{description: "Move between the sidebar and the editor", keys: []keyPress{key(input.KeyTab, "Tab")},
		do: func(application *Window, _ *ui.Context, _ int) { application.toggleFocus() }},
	{description: "Next / previous changed file, across every project",
		keys: []keyPress{{key: input.KeyN, label: "n", step: 1}, {key: input.KeyM, label: "m", step: -1}},
		do:   func(application *Window, _ *ui.Context, step int) { application.stepChangedFile(step) }},
	{description: "Next / previous change in the file",
		keys: []keyPress{{key: input.KeyRightBracket, label: "]", step: 1}, {key: input.KeyLeftBracket, label: "[", step: -1}},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.StepHunk(step) }},
	{description: "Diff or the whole file", keys: []keyPress{key(input.KeyD, "d")},
		do: func(application *Window, _ *ui.Context, _ int) { application.toggleView() }},
	{description: "Fold the unchanged lines, or show them all", keys: []keyPress{key(input.KeyF, "f")},
		do: func(application *Window, _ *ui.Context, _ int) {
			if application.editor.Mode == viewer.DiffView {
				application.editor.ToggleContext()
			}
		}},
	{description: "Files mode (read any file) / Review mode (the changes, as diffs) / the other one",
		keys: []keyPress{{key: input.Key1, label: "1", step: 1}, {key: input.Key2, label: "2", step: 2}, key(input.KeyC, "c")},
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
	{description: "Mark the file as viewed and open the next one not viewed", keys: []keyPress{key(input.KeyV, "v")},
		do: func(application *Window, _ *ui.Context, _ int) {
			if reference, ok := application.markTarget(); ok {
				application.ToggleViewed(reference, application.focus == editorPane || application.editor.Reference == reference)
			}
		}},
	{description: "Only the projects with changes, or every project (sidebar and finder)", keys: []keyPress{key(input.KeyH, "h")},
		do: func(application *Window, _ *ui.Context, _ int) { application.setOnlyChanged(!application.onlyChanged) }},
	{description: "Count the changes not committed yet, or only commits", keys: []keyPress{key(input.KeyW, "w")},
		do: func(application *Window, _ *ui.Context, _ int) {
			application.settings.WorkingTree = !application.settings.WorkingTree
			application.clearDocuments()
			application.reloadAll()
			application.toast = workingTreeNote(application.settings.WorkingTree)
		}},
	{description: "Light or dark", keys: []keyPress{key(input.KeyT, "t")},
		do: func(application *Window, context *ui.Context, _ int) {
			application.dark = !application.dark
			application.applyTheme(context)
		}},
	{description: "Copy the open file's path", keys: []keyPress{key(input.KeyY, "y")},
		do: func(application *Window, context *ui.Context, _ int) { application.copyPath(context) }},
	{description: "These keys", keys: []keyPress{{text: "?", label: "?"}, key(input.KeyF1, "F1")},
		do: func(application *Window, _ *ui.Context, _ int) { application.helpOpen = true }},

	{description: "Select the next / previous row, a page, the first / last", place: inSidebar,
		keys: []keyPress{stepKey(input.KeyDown, "↓", 1), stepKey(input.KeyUp, "↑", -1),
			stepKey(input.KeyPageDown, "PgDn", pane.PageStep), stepKey(input.KeyPageUp, "PgUp", -pane.PageStep),
			{key: input.KeyHome, label: "Home", step: -pane.EndStep}, {key: input.KeyEnd, label: "End", step: pane.EndStep}},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do: func(application *Window, _ *ui.Context, step int) {
			application.sidebar.MoveSelection(application, step)
		}},
	{description: "Open / close a folder", place: inSidebar,
		keys:    []keyPress{stepKey(input.KeyRight, "→", 1), stepKey(input.KeyLeft, "←", -1)},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do:      func(application *Window, _ *ui.Context, step int) { application.sidebar.OpenOrClose(application, step) }},
	{description: "Open the row: a file (Enter moves to it), a folder, or enter a project", place: inSidebar,
		keys:    []keyPress{{key: input.KeyEnter, label: "Enter", step: 1}, key(input.KeySpace, "Space")},
		applies: func(application *Window) bool { return len(application.sidebar.Rows) > 0 },
		do: func(application *Window, _ *ui.Context, step int) {
			application.sidebar.OpenSelected(application, step == 1)
		}},

	{description: "Scroll a line down / up", place: inEditor,
		keys: []keyPress{stepKey(input.KeyDown, "↓", 1), stepKey(input.KeyJ, "j", 1), stepKey(input.KeyUp, "↑", -1), stepKey(input.KeyK, "k", -1)},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.ScrollLines(step) }},
	{description: "Scroll a page up", place: inEditor,
		keys: []keyPress{{key: input.KeySpace, shift: true, label: "Shift+Space", step: -pane.PageStep, repeats: true},
			stepKey(input.KeyPageUp, "PgUp", -pane.PageStep)},
		do: func(application *Window, _ *ui.Context, step int) { application.editor.ScrollLines(step) }},
	{description: "Scroll a page down", place: inEditor,
		keys: []keyPress{stepKey(input.KeySpace, "Space", pane.PageStep), stepKey(input.KeyPageDown, "PgDn", pane.PageStep)},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.ScrollLines(step) }},
	{description: "Go to the top / bottom", place: inEditor,
		keys: []keyPress{{key: input.KeyHome, label: "Home", step: -pane.EndStep}, {key: input.KeyEnd, label: "End", step: pane.EndStep}},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.ScrollLines(step) }},
	{description: "Scroll sideways", place: inEditor,
		keys: []keyPress{stepKey(input.KeyRight, "→", 1), stepKey(input.KeyLeft, "←", -1)},
		do:   func(application *Window, _ *ui.Context, step int) { application.editor.ScrollSideways(step) }},
	{description: "Open a fold in sight", place: inEditor, keys: []keyPress{key(input.KeyEnter, "Enter")},
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
			labels[index] = press.label
		}
		rows = append(rows, helpRow{strings.Join(labels, "  "), command.description})
	}
	return rows
}
