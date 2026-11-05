package window

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

// longFileLines is long enough for the file not to fit in the test window, so it scrolls, and
// for the unchanged lines between its two changes to be folded in a diff.
const longFileLines = 80

// changedLines are the lines of the long file the development replaces; a diff shows each twice.
const changedLines = 2

// addLongRepository puts beside the others a repository whose development changes two lines far
// apart in one long file: two runs of changes, and a fold between them.
func addLongRepository(test *testing.T, folder string) {
	test.Helper()
	repository := filepath.Join(folder, "tides")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		test.Fatal(err)
	}
	gitIn := func(arguments ...string) {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	write := func(changed bool) {
		var text strings.Builder
		for line := 1; line <= longFileLines; line++ {
			if changed && (line == 2 || line == 70) {
				fmt.Fprintf(&text, "high tide %d\n", line)
				continue
			}
			fmt.Fprintf(&text, "low tide %d\n", line)
		}
		if err := os.WriteFile(filepath.Join(repository, "coast.txt"), []byte(text.String()), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	gitIn("init", "-q", "-b", "main")
	write(false)
	gitIn("add", ".")
	gitIn("commit", "-q", "-m", "start")
	gitIn("checkout", "-q", "-b", "feature/higher")
	write(true)
	gitIn("commit", "-q", "-am", "higher")
}

// press presses a key with the modifiers given held down, and lets the reader answer.
func press(window *uitest.Window, key input.Key, modifiers ...input.Key) {
	for _, modifier := range modifiers {
		window.HoldKey(modifier)
	}
	window.PressKey(key)
	for _, modifier := range modifiers {
		window.ReleaseKey(modifier)
	}
	window.Frames(2)
}

// typeText types text, as a key that gives a character rather than a key code (/ and ?).
func typeText(window *uitest.Window, text string) {
	window.Type(text)
	window.Frames(2)
}

// selectRow selects the sidebar's row with this label.
func selectRow(test *testing.T, application *Window, label string) {
	test.Helper()
	application.sidebar.Rebuild(application)
	for index, row := range application.sidebar.Rows {
		if row.Label == label {
			application.sidebar.Selected = index
			application.focus = sidebarPane
			return
		}
	}
	test.Fatalf("the sidebar has no row %q", label)
}

func openPath(application *Window) string {
	if !application.viewer.IsOpen() || application.viewer.Document == nil {
		return ""
	}
	return application.viewer.Project.Name + "/" + application.viewer.Document.Path
}

// shortcut is one key of the help, the state it is pressed in, and what must be true after it.
type shortcut struct {
	name   string
	open   string // a file to open first, as "project/path"
	review bool   // start in the review mode
	long   bool   // add the repository with a long file
	// prepare runs after the window opens and before the key: to select a row or move the focus
	prepare func(test *testing.T, application *Window, window *uitest.Window)
	press   func(window *uitest.Window)
	// holds says what is wrong after the key, or nothing when it did its job
	holds func(application *Window) string
	// copies is the text the key must leave on the clipboard
	copies string
}

func (key shortcut) run(test *testing.T) {
	folder := makeRepository(test)
	if key.long {
		addLongRepository(test, folder)
	}
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, Open: key.open, Mode: startIn(key.review)})
	window := newWindow(test, application)
	clipboard := &memoryClipboard{}
	application.tap.state.Clipboard = clipboard
	if key.prepare != nil {
		key.prepare(test, application, window)
	}
	key.press(window)
	if key.holds != nil {
		if problem := key.holds(application); problem != "" {
			test.Error(problem)
		}
	}
	if copied := clipboard.text; key.copies != "" && copied != key.copies {
		test.Errorf("the clipboard holds %q, want %q", copied, key.copies)
	}
}

// memoryClipboard stands in for the system's clipboard, which a test window has none of.
type memoryClipboard struct{ text string }

func (clipboard *memoryClipboard) ClipboardText() string { return clipboard.text }

func (clipboard *memoryClipboard) SetClipboardText(text string) { clipboard.text = text }

// addAFileUnseen adds a file to the first project after the reader has read it.
func addAFileUnseen(test *testing.T, application *Window, window *uitest.Window) {
	path := filepath.Join(application.projects[0].Path, "src", "lens.go")
	if err := os.WriteFile(path, []byte("package lamp\n"), 0o644); err != nil {
		test.Fatal(err)
	}
}

func focusEditor(test *testing.T, application *Window, window *uitest.Window) {
	application.focus = viewerPane
}

// pressing is a shortcut's press of a key, with modifiers held.
func pressing(key input.Key, modifiers ...input.Key) func(window *uitest.Window) {
	return func(window *uitest.Window) { press(window, key, modifiers...) }
}

// typing is a shortcut's press of the key that types a text.
func typing(text string) func(window *uitest.Window) {
	return func(window *uitest.Window) { typeText(window, text) }
}

// runShortcuts runs each shortcut as a test of its own.
func runShortcuts(test *testing.T, shortcuts []shortcut) {
	for _, key := range shortcuts {
		test.Run(key.name, key.run)
	}
}

func TestEveryWindowShortcutDoesItsJob(test *testing.T) {
	runShortcuts(test, append(append(openingShortcuts(), settingShortcuts()...), movingShortcuts()...))
}

// openingShortcuts open the window's dialogs and panels.
func openingShortcuts() []shortcut {
	pickerOpen := func(application *Window) string { return unless(application.pickerOpen, "the picker is closed") }
	helpOpen := func(application *Window) string { return unless(application.helpOpen, "the help is closed") }
	return []shortcut{
		{name: "o opens the folder picker", press: pressing(input.KeyO), holds: pickerOpen},
		{name: "⌘O opens the folder picker", press: pressing(input.KeyO, input.KeySuper), holds: pickerOpen},
		{name: "b opens the base dialog", press: pressing(input.KeyB),
			holds: func(application *Window) string { return unless(application.base.open, "the base dialog is closed") }},
		{name: "Shift+P finds a file by name", press: pressing(input.KeyP, input.KeyShift),
			holds: func(application *Window) string {
				return unless(application.finder.Open && application.finder.Scope == finder.NamesScope, "the finder is not open on names")
			}},
		{name: "/ finds a text in the files", press: typing("/"),
			holds: func(application *Window) string {
				return unless(application.finder.Open && application.finder.Scope == finder.ContentScope, "the finder is not open on content")
			}},
		{name: "? opens the help", press: typing("?"), holds: helpOpen},
		{name: "F1 opens the help", press: pressing(input.KeyF1), holds: helpOpen},
	}
}

// settingShortcuts read the projects again and change what the window compares and shows.
func settingShortcuts() []shortcut {
	readAgain := func(application *Window) string {
		return unless(len(application.projects[0].Changes) == 4, "the file added since the start is not among the changes")
	}
	return []shortcut{
		{name: "r reads the projects again", prepare: addAFileUnseen, press: pressing(input.KeyR), holds: readAgain},
		{name: "⌘R reads the projects again", prepare: addAFileUnseen, press: pressing(input.KeyR, input.KeySuper), holds: readAgain},
		{name: "c switches between the two modes", press: pressing(input.KeyC),
			holds: func(application *Window) string {
				return unless(application.mode.Name() == "Review", "the review mode is not on")
			}},
		{name: "1 goes to the files mode", review: true, press: pressing(input.Key1),
			holds: func(application *Window) string {
				return unless(application.mode.Name() == "Files", "the files mode is not on")
			}},
		{name: "w compares the last commit only", press: pressing(input.KeyW),
			holds: func(application *Window) string {
				return unless(!application.settings.WorkingTree, "the changes not committed are still counted")
			}},
		{name: "t switches the theme", press: pressing(input.KeyT),
			holds: func(application *Window) string { return unless(application.dark, "the theme is still light") }},
		{name: "y copies the path of the file open", open: "lighthouse/src/lamp.go", press: pressing(input.KeyY),
			copies: "lighthouse/src/lamp.go"},
	}
}

// movingShortcuts move between projects, panes, files and changes, and change how a file shows.
func movingShortcuts() []shortcut {
	return append([]shortcut{
		{name: "e enters the project of the file open", open: "lighthouse/src/lamp.go", prepare: focusEditor,
			press: pressing(input.KeyE),
			holds: func(application *Window) string {
				return unless(application.sidebar.Entered != "", "no project was entered")
			}},
		{name: "Backspace leaves the project entered", open: "lighthouse/src/lamp.go",
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				focusEditor(test, application, window)
				press(window, input.KeyE)
			},
			press: pressing(input.KeyBackspace),
			holds: func(application *Window) string {
				return unless(application.sidebar.Entered == "", "the project is still entered")
			}},
		{name: "Tab moves between the sidebar and the file open", open: "lighthouse/src/lamp.go",
			prepare: func(test *testing.T, application *Window, window *uitest.Window) { application.focus = sidebarPane },
			press:   pressing(input.KeyTab),
			holds: func(application *Window) string {
				return unless(application.focus == viewerPane, "the focus stayed on the sidebar")
			}},
		{name: "m opens the changed file before", review: true,
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				press(window, input.KeyN)
				press(window, input.KeyN)
			},
			press: pressing(input.KeyM),
			holds: func(application *Window) string {
				return unless(openPath(application) == "lighthouse/src/beam.go", "m opened "+openPath(application)+", want lighthouse/src/beam.go")
			}},
	}, changeShortcuts()...)
}

// changeShortcuts move between a file's changes and change how it shows.
func changeShortcuts() []shortcut {
	return []shortcut{
		{name: "] goes to the next run of changes", open: "tides/coast.txt", long: true, prepare: focusEditor,
			press: pressing(input.KeyRightBracket),
			holds: func(application *Window) string {
				return unless(application.viewer.List.Offset > 0, "the file did not move down to the second change")
			}},
		{name: "[ goes back to the run of changes before", open: "tides/coast.txt", long: true,
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				focusEditor(test, application, window)
				press(window, input.KeyRightBracket)
			},
			press: pressing(input.KeyLeftBracket),
			holds: func(application *Window) string {
				return unless(application.viewer.List.Offset == 0, "the file did not move back up to the first change")
			}},
		{name: "d shows the file open as its diff", open: "lighthouse/src/lamp.go", press: pressing(input.KeyD),
			holds: func(application *Window) string {
				return unless(application.viewer.Mode == viewer.DiffView, "the file is not shown as a diff")
			}},
		{name: "f shows every unchanged line of a diff", open: "tides/coast.txt", long: true, review: true,
			press: pressing(input.KeyF),
			holds: func(application *Window) string {
				return unless(application.viewer.RowCount() == longFileLines+changedLines, fmt.Sprintf("the diff shows %d rows, want all %d lines and the %d old ones", application.viewer.RowCount(), longFileLines, changedLines))
			}},
	}
}

func TestEverySidebarShortcutDoesItsJob(test *testing.T) {
	runShortcuts(test, sidebarShortcuts())
}

// selectTopRow selects the first project's row.
func selectTopRow(test *testing.T, application *Window, window *uitest.Window) {
	selectRow(test, application, "lighthouse")
}

// selectFolderRow selects the folder src.
func selectFolderRow(test *testing.T, application *Window, window *uitest.Window) {
	selectRow(test, application, "src")
}

// selectedLabel is the name on the row selected, if any.
func selectedLabel(application *Window) string {
	sidebar := &application.sidebar
	if sidebar.Selected < 0 || sidebar.Selected >= len(sidebar.Rows) {
		return ""
	}
	return sidebar.Rows[sidebar.Selected].Label
}

// expanded tells whether the row of a name is open, as the sidebar lists it now.
func expanded(application *Window, label string) bool {
	application.sidebar.Rebuild(application)
	for _, row := range application.sidebar.Rows {
		if row.Label == label {
			return row.Expanded
		}
	}
	return false
}

// sidebarShortcuts move the selection through the tree, and open folders and files.
func sidebarShortcuts() []shortcut {
	return append([]shortcut{
		{name: "Down selects the next row", prepare: selectTopRow, press: pressing(input.KeyDown),
			holds: func(application *Window) string {
				return unless(application.sidebar.Selected == 1, "the selection did not move down")
			}},
		{name: "Up selects the row before", prepare: selectFolderRow, press: pressing(input.KeyUp),
			holds: func(application *Window) string {
				return unless(selectedLabel(application) == "lighthouse", "selected "+selectedLabel(application))
			}},
		{name: "End selects the last row", prepare: selectTopRow, press: pressing(input.KeyEnd),
			holds: func(application *Window) string {
				return unless(application.sidebar.Selected == len(application.sidebar.Rows)-1, "the last row is not selected")
			}},
		{name: "Home selects the first row", prepare: selectFolderRow, press: pressing(input.KeyHome),
			holds: func(application *Window) string {
				return unless(application.sidebar.Selected == 0, "the first row is not selected")
			}},
	}, sidebarOpeningShortcuts()...)
}

// sidebarOpeningShortcuts open and close folders, and open files.
func sidebarOpeningShortcuts() []shortcut {
	selectNotes := func(test *testing.T, application *Window, window *uitest.Window) {
		selectRow(test, application, "notes.txt")
	}
	return []shortcut{
		{name: "Right opens a folder", prepare: selectFolderRow, press: pressing(input.KeyRight),
			holds: func(application *Window) string { return unless(expanded(application, "src"), "src is closed") }},
		{name: "Left closes a folder",
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				selectFolderRow(test, application, window)
				press(window, input.KeyRight)
			},
			press: pressing(input.KeyLeft),
			holds: func(application *Window) string { return unless(!expanded(application, "src"), "src is still open") }},
		{name: "Enter opens a file and moves to it", prepare: selectNotes, press: pressing(input.KeyEnter),
			holds: func(application *Window) string {
				return unless(openPath(application) == "lighthouse/notes.txt" && application.focus == viewerPane, "notes.txt is not open with the focus on it")
			}},
		{name: "Space opens a file and stays in the sidebar", prepare: selectNotes, press: pressing(input.KeySpace),
			holds: func(application *Window) string {
				return unless(openPath(application) == "lighthouse/notes.txt" && application.focus == sidebarPane, "notes.txt is not open with the focus left on the sidebar")
			}},
	}
}

func TestEveryFileShortcutDoesItsJob(test *testing.T) {
	runShortcuts(test, append(scrollingShortcuts(), foldingShortcut()))
}

// offset is how far down the file open is scrolled.
func offset(application *Window) float32 { return application.viewer.List.Offset }

// scrolledDown says the file did not scroll down, if it did not.
func scrolledDown(application *Window) string {
	return unless(offset(application) > 0, "the file did not scroll down")
}

// scrolledUp says the file did not scroll up from its bottom, if it did not.
func scrolledUp(application *Window) string {
	return unless(offset(application) < application.viewer.List.ContentHeight()-application.viewer.List.Area.Height, "the file did not scroll up")
}

// scrollDownFirst puts the file open at its bottom, with the keyboard on it.
func scrollDownFirst(test *testing.T, application *Window, window *uitest.Window) {
	focusEditor(test, application, window)
	press(window, input.KeyEnd)
}

// scrollingShortcuts scroll the file open by lines, by pages and to its ends.
func scrollingShortcuts() []shortcut {
	long := func(name string, prepare func(*testing.T, *Window, *uitest.Window), press func(*uitest.Window), holds func(*Window) string) shortcut {
		return shortcut{name: name, open: "tides/coast.txt", long: true, prepare: prepare, press: press, holds: holds}
	}
	return []shortcut{
		long("j scrolls down a line", focusEditor, pressing(input.KeyJ), scrolledDown),
		long("Down scrolls down a line", focusEditor, pressing(input.KeyDown), scrolledDown),
		long("Space scrolls down a page", focusEditor, pressing(input.KeySpace), func(application *Window) string {
			return unless(offset(application) > 10*application.viewer.List.RowHeight, "the file did not scroll a page")
		}),
		long("End goes to the bottom", focusEditor, pressing(input.KeyEnd), func(application *Window) string {
			return unless(scrolledUp(application) != "", "the file is not at its bottom")
		}),
		long("k scrolls up a line", scrollDownFirst, pressing(input.KeyK), scrolledUp),
		long("Shift+Space scrolls up a page", scrollDownFirst, pressing(input.KeySpace, input.KeyShift), scrolledUp),
		long("Home goes to the top", scrollDownFirst, pressing(input.KeyHome), func(application *Window) string {
			return unless(offset(application) == 0, "the file is not at its top")
		}),
	}
}

// foldingShortcut opens the fold in sight of a diff.
func foldingShortcut() shortcut {
	var rowsFolded int
	return shortcut{name: "Enter opens the fold in sight", open: "tides/coast.txt", long: true, review: true,
		prepare: func(test *testing.T, application *Window, window *uitest.Window) {
			focusEditor(test, application, window)
			rowsFolded = application.viewer.RowCount()
		},
		press: pressing(input.KeyEnter),
		holds: func(application *Window) string {
			return unless(application.viewer.RowCount() > rowsFolded, fmt.Sprintf("the diff still shows %d rows", rowsFolded))
		}}
}

// unless gives the problem when a promise does not hold, and nothing when it does.
func unless(holds bool, problem string) string {
	if holds {
		return ""
	}
	return problem
}
