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

func TestEveryWindowShortcutDoesItsJob(test *testing.T) {
	shortcuts := []shortcut{
		{name: "o opens the folder picker", press: func(window *uitest.Window) { press(window, input.KeyO) },
			holds: func(application *Window) string { return unless(application.pickerOpen, "the picker is closed") }},
		{name: "⌘O opens the folder picker", press: func(window *uitest.Window) { press(window, input.KeyO, input.KeySuper) },
			holds: func(application *Window) string { return unless(application.pickerOpen, "the picker is closed") }},
		{name: "r reads the projects again", prepare: addAFileUnseen,
			press: func(window *uitest.Window) { press(window, input.KeyR) },
			holds: func(application *Window) string {
				return unless(len(application.projects[0].Changes) == 4, "the file added since the start is not among the changes")
			}},
		{name: "⌘R reads the projects again", prepare: addAFileUnseen,
			press: func(window *uitest.Window) { press(window, input.KeyR, input.KeySuper) },
			holds: func(application *Window) string {
				return unless(len(application.projects[0].Changes) == 4, "the file added since the start is not among the changes")
			}},
		{name: "b opens the base dialog", press: func(window *uitest.Window) { press(window, input.KeyB) },
			holds: func(application *Window) string { return unless(application.base.open, "the base dialog is closed") }},
		{name: "Shift+P finds a file by name", press: func(window *uitest.Window) { press(window, input.KeyP, input.KeyShift) },
			holds: func(application *Window) string {
				return unless(application.finder.Open && application.finder.Scope == finder.NamesScope, "the finder is not open on names")
			}},
		{name: "/ finds a text in the files", press: func(window *uitest.Window) { typeText(window, "/") },
			holds: func(application *Window) string {
				return unless(application.finder.Open && application.finder.Scope == finder.ContentScope, "the finder is not open on content")
			}},
		{name: "? opens the help", press: func(window *uitest.Window) { typeText(window, "?") },
			holds: func(application *Window) string { return unless(application.helpOpen, "the help is closed") }},
		{name: "F1 opens the help", press: func(window *uitest.Window) { press(window, input.KeyF1) },
			holds: func(application *Window) string { return unless(application.helpOpen, "the help is closed") }},
		{name: "e enters the project of the file open", open: "lighthouse/src/lamp.go", prepare: focusEditor,
			press: func(window *uitest.Window) { press(window, input.KeyE) },
			holds: func(application *Window) string {
				return unless(application.sidebar.Entered != "", "no project was entered")
			}},
		{name: "Backspace leaves the project entered", open: "lighthouse/src/lamp.go",
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				focusEditor(test, application, window)
				press(window, input.KeyE)
			},
			press: func(window *uitest.Window) { press(window, input.KeyBackspace) },
			holds: func(application *Window) string {
				return unless(application.sidebar.Entered == "", "the project is still entered")
			}},
		{name: "Tab moves between the sidebar and the file open", open: "lighthouse/src/lamp.go",
			prepare: func(test *testing.T, application *Window, window *uitest.Window) { application.focus = sidebarPane },
			press:   func(window *uitest.Window) { press(window, input.KeyTab) },
			holds: func(application *Window) string {
				return unless(application.focus == viewerPane, "the focus stayed on the sidebar")
			}},
		{name: "m opens the changed file before", review: true,
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				press(window, input.KeyN)
				press(window, input.KeyN)
			},
			press: func(window *uitest.Window) { press(window, input.KeyM) },
			holds: func(application *Window) string {
				return unless(openPath(application) == "lighthouse/src/beam.go", "m opened "+openPath(application)+", want lighthouse/src/beam.go")
			}},
		{name: "] goes to the next run of changes", open: "tides/coast.txt", long: true, prepare: focusEditor,
			press: func(window *uitest.Window) { press(window, input.KeyRightBracket) },
			holds: func(application *Window) string {
				return unless(application.viewer.List.Offset > 0, "the file did not move down to the second change")
			}},
		{name: "[ goes back to the run of changes before", open: "tides/coast.txt", long: true,
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				focusEditor(test, application, window)
				press(window, input.KeyRightBracket)
			},
			press: func(window *uitest.Window) { press(window, input.KeyLeftBracket) },
			holds: func(application *Window) string {
				return unless(application.viewer.List.Offset == 0, "the file did not move back up to the first change")
			}},
		{name: "d shows the file open as its diff", open: "lighthouse/src/lamp.go",
			press: func(window *uitest.Window) { press(window, input.KeyD) },
			holds: func(application *Window) string {
				return unless(application.viewer.Mode == viewer.DiffView, "the file is not shown as a diff")
			}},
		{name: "f shows every unchanged line of a diff", open: "tides/coast.txt", long: true, review: true,
			press: func(window *uitest.Window) { press(window, input.KeyF) },
			holds: func(application *Window) string {
				return unless(application.viewer.RowCount() == longFileLines+changedLines, fmt.Sprintf("the diff shows %d rows, want all %d lines and the %d old ones", application.viewer.RowCount(), longFileLines, changedLines))
			}},
		{name: "c switches between the two modes",
			press: func(window *uitest.Window) { press(window, input.KeyC) },
			holds: func(application *Window) string {
				return unless(application.mode.Name() == "Review", "the review mode is not on")
			}},
		{name: "1 goes to the files mode", review: true,
			press: func(window *uitest.Window) { press(window, input.Key1) },
			holds: func(application *Window) string {
				return unless(application.mode.Name() == "Files", "the files mode is not on")
			}},
		{name: "w compares the last commit only",
			press: func(window *uitest.Window) { press(window, input.KeyW) },
			holds: func(application *Window) string {
				return unless(!application.settings.WorkingTree, "the changes not committed are still counted")
			}},
		{name: "t switches the theme",
			press: func(window *uitest.Window) { press(window, input.KeyT) },
			holds: func(application *Window) string { return unless(application.dark, "the theme is still light") }},
		{name: "y copies the path of the file open", open: "lighthouse/src/lamp.go",
			press:  func(window *uitest.Window) { press(window, input.KeyY) },
			copies: "lighthouse/src/lamp.go"},
	}
	for _, key := range shortcuts {
		test.Run(key.name, key.run)
	}
}

func TestEverySidebarShortcutDoesItsJob(test *testing.T) {
	selectTop := func(test *testing.T, application *Window, window *uitest.Window) {
		selectRow(test, application, "lighthouse")
	}
	selectFolder := func(test *testing.T, application *Window, window *uitest.Window) { selectRow(test, application, "src") }
	selected := func(application *Window) string {
		sidebar := &application.sidebar
		if sidebar.Selected < 0 || sidebar.Selected >= len(sidebar.Rows) {
			return ""
		}
		return sidebar.Rows[sidebar.Selected].Label
	}
	expanded := func(application *Window, label string) bool {
		application.sidebar.Rebuild(application)
		for _, row := range application.sidebar.Rows {
			if row.Label == label {
				return row.Expanded
			}
		}
		return false
	}
	shortcuts := []shortcut{
		{name: "Down selects the next row", prepare: selectTop,
			press: func(window *uitest.Window) { press(window, input.KeyDown) },
			holds: func(application *Window) string {
				return unless(application.sidebar.Selected == 1, "the selection did not move down")
			}},
		{name: "Up selects the row before", prepare: selectFolder,
			press: func(window *uitest.Window) { press(window, input.KeyUp) },
			holds: func(application *Window) string {
				return unless(selected(application) == "lighthouse", "selected "+selected(application))
			}},
		{name: "End selects the last row", prepare: selectTop,
			press: func(window *uitest.Window) { press(window, input.KeyEnd) },
			holds: func(application *Window) string {
				return unless(application.sidebar.Selected == len(application.sidebar.Rows)-1, "the last row is not selected")
			}},
		{name: "Home selects the first row", prepare: selectFolder,
			press: func(window *uitest.Window) { press(window, input.KeyHome) },
			holds: func(application *Window) string {
				return unless(application.sidebar.Selected == 0, "the first row is not selected")
			}},
		{name: "Right opens a folder", prepare: selectFolder,
			press: func(window *uitest.Window) { press(window, input.KeyRight) },
			holds: func(application *Window) string { return unless(expanded(application, "src"), "src is closed") }},
		{name: "Left closes a folder",
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				selectFolder(test, application, window)
				press(window, input.KeyRight)
			},
			press: func(window *uitest.Window) { press(window, input.KeyLeft) },
			holds: func(application *Window) string { return unless(!expanded(application, "src"), "src is still open") }},
		{name: "Enter opens a file and moves to it",
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				selectRow(test, application, "notes.txt")
			},
			press: func(window *uitest.Window) { press(window, input.KeyEnter) },
			holds: func(application *Window) string {
				return unless(openPath(application) == "lighthouse/notes.txt" && application.focus == viewerPane, "notes.txt is not open with the focus on it")
			}},
		{name: "Space opens a file and stays in the sidebar",
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				selectRow(test, application, "notes.txt")
			},
			press: func(window *uitest.Window) { press(window, input.KeySpace) },
			holds: func(application *Window) string {
				return unless(openPath(application) == "lighthouse/notes.txt" && application.focus == sidebarPane, "notes.txt is not open with the focus left on the sidebar")
			}},
	}
	for _, key := range shortcuts {
		test.Run(key.name, key.run)
	}
}

func TestEveryFileShortcutDoesItsJob(test *testing.T) {
	offset := func(application *Window) float32 { return application.viewer.List.Offset }
	scrolledDown := func(application *Window) string {
		return unless(offset(application) > 0, "the file did not scroll down")
	}
	scrollDownFirst := func(test *testing.T, application *Window, window *uitest.Window) {
		focusEditor(test, application, window)
		press(window, input.KeyEnd)
	}
	scrolledUp := func(application *Window) string {
		return unless(offset(application) < application.viewer.List.ContentHeight()-application.viewer.List.Area.Height, "the file did not scroll up")
	}
	var rowsFolded int
	shortcuts := []shortcut{
		{name: "j scrolls down a line", open: "tides/coast.txt", long: true, prepare: focusEditor,
			press: func(window *uitest.Window) { press(window, input.KeyJ) }, holds: scrolledDown},
		{name: "Down scrolls down a line", open: "tides/coast.txt", long: true, prepare: focusEditor,
			press: func(window *uitest.Window) { press(window, input.KeyDown) }, holds: scrolledDown},
		{name: "Space scrolls down a page", open: "tides/coast.txt", long: true, prepare: focusEditor,
			press: func(window *uitest.Window) { press(window, input.KeySpace) },
			holds: func(application *Window) string {
				return unless(offset(application) > 10*application.viewer.List.RowHeight, "the file did not scroll a page")
			}},
		{name: "End goes to the bottom", open: "tides/coast.txt", long: true, prepare: focusEditor,
			press: func(window *uitest.Window) { press(window, input.KeyEnd) },
			holds: func(application *Window) string {
				return unless(scrolledUp(application) != "", "the file is not at its bottom")
			}},
		{name: "k scrolls up a line", open: "tides/coast.txt", long: true, prepare: scrollDownFirst,
			press: func(window *uitest.Window) { press(window, input.KeyK) }, holds: scrolledUp},
		{name: "Shift+Space scrolls up a page", open: "tides/coast.txt", long: true, prepare: scrollDownFirst,
			press: func(window *uitest.Window) { press(window, input.KeySpace, input.KeyShift) }, holds: scrolledUp},
		{name: "Home goes to the top", open: "tides/coast.txt", long: true, prepare: scrollDownFirst,
			press: func(window *uitest.Window) { press(window, input.KeyHome) },
			holds: func(application *Window) string {
				return unless(offset(application) == 0, "the file is not at its top")
			}},
		{name: "Enter opens the fold in sight", open: "tides/coast.txt", long: true, review: true,
			prepare: func(test *testing.T, application *Window, window *uitest.Window) {
				focusEditor(test, application, window)
				rowsFolded = application.viewer.RowCount()
			},
			press: func(window *uitest.Window) { press(window, input.KeyEnter) },
			holds: func(application *Window) string {
				return unless(application.viewer.RowCount() > rowsFolded, fmt.Sprintf("the diff still shows %d rows", rowsFolded))
			}},
	}
	for _, key := range shortcuts {
		test.Run(key.name, key.run)
	}
}

// unless gives the problem when a promise does not hold, and nothing when it does.
func unless(holds bool, problem string) string {
	if holds {
		return ""
	}
	return problem
}
