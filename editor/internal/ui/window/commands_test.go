package window

import (
	"slices"
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/ui/controls"
	"github.com/akrck02/coddy/editor/internal/ui/shortcut"
	"github.com/akrck02/coddy/editor/internal/ui/viewer"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

// overlaps tells whether two places can have the keyboard at once: anywhere overlaps every place.
func overlaps(first, second place) bool {
	return first == second || first == anywhere || second == anywhere
}

// stopsEditingFirst tells whether a key shared is Esc in a file being edited, which comes first in
// the commands and stops editing before Esc can leave a project.
func stopsEditingFirst(first, second command, key shortcut.Press) bool {
	return key.Key == input.KeyEscape && first.place == editing && second.place != editing
}

// sameKeys tells whether two key presses are the same keys, so one would hide the other.
func sameKeys(first, second shortcut.Press) bool {
	return first.Key == second.Key && first.Text == second.Text && first.Command == second.Command && first.Shift == second.Shift &&
		first.Alt == second.Alt
}

func TestNoTwoCommandsShareAKeyInOneMode(test *testing.T) {
	commands := newReader(Settings{Synchronous: true}).allCommands()
	for firstIndex, first := range commands {
		for secondIndex := firstIndex; secondIndex < len(commands); secondIndex++ {
			second := commands[secondIndex]
			if !overlaps(first.place, second.place) {
				continue
			}
			for keyIndex, firstKey := range first.keys {
				for otherIndex, secondKey := range second.keys {
					if firstIndex == secondIndex && otherIndex <= keyIndex {
						continue
					}
					if sameKeys(firstKey, secondKey) && !stopsEditingFirst(first, second, firstKey) {
						test.Errorf("%s is the key of both %q and %q", firstKey.Label, first.description, second.description)
					}
				}
			}
		}
	}
}

func TestTheHelpListsEveryCommand(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	// tall enough for the longest topic, which scrolls in a shorter window
	window, err := uitest.New(application.Build, uitest.Driver(application.Driver()), uitest.Size(1200, 2000))
	if err != nil {
		test.Fatal(err)
	}
	press(window, input.KeyF1)
	// the keys screen shows one topic at a time: each chip shows its own
	var texts []string
	for _, section := range application.KeySections() {
		if err := window.Click(section.Title); err != nil {
			test.Fatal(err)
		}
		window.Frames(2)
		texts = append(texts, window.Texts()...)
	}
	shown := strings.Join(strings.Fields(strings.Join(texts, " ")), " ")
	for _, command := range application.allCommands() {
		if command.applies != nil && !command.applies(application) {
			continue // a key of another mode, listed while that mode is on
		}
		if !strings.Contains(shown, command.description) {
			test.Errorf("the help does not say %q", command.description)
		}
		for _, press := range command.keys {
			// a key is drawn as a cap for each key pressed together
			if press.Label == "" || !strings.Contains(shown, controls.KeyCapWords(press.Label)) {
				test.Errorf("the help does not show the key %q of %q", press.Label, command.description)
			}
		}
	}
}

// While editing, the plain keys type: every command there needs Command, an F-key or Esc, and
// every command of the whole window keeps such a key.
func TestNoPlainKeyCommandWhileEditing(test *testing.T) {
	for _, command := range newReader(Settings{Synchronous: true}).allCommands() {
		working := 0
		for _, press := range command.keys {
			if press.WorksWhileEditing() {
				working++
			} else if command.place == editing {
				test.Errorf("%q takes the plain key %s while editing", command.description, press.Label)
			}
		}
		if command.place != inSidebar && command.place != inViewer && working == 0 {
			test.Errorf("%q has no key that works while editing", command.description)
		}
	}
}

// editingAFile opens a file whole with the keyboard in it, ready to edit.
func editingAFile(test *testing.T) (*Window, *uitest.Window) {
	folder := makeRepository(test)
	addLongRepository(test, folder)
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, Open: "tides/coast.txt"})
	window := newWindow(test, application)
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	if !application.editing() {
		test.Fatal("the file open is not being edited")
	}
	return application, window
}

func TestAPlainLetterRunsNoCommandWhileEditing(test *testing.T) {
	application, window := editingAFile(test)
	press(window, input.KeyD)
	if application.viewer.Mode != viewer.FileView {
		test.Error("d while editing went to the diff")
	}
	press(window, input.KeyD, input.KeySuper)
	if application.viewer.Mode != viewer.DiffView {
		test.Error("⌘D while editing did not go to the diff")
	}
}

func TestEscStopsEditing(test *testing.T) {
	application, window := editingAFile(test)
	press(window, input.KeyEscape)
	if application.editing() || application.focus != sidebarPane {
		test.Error("Esc did not give the keyboard to the sidebar")
	}
}

func TestTheCodeScrolledAwayFromTheCaretStaysWhereItWasScrolled(test *testing.T) {
	application, window := editingAFile(test)
	window.Frames(2)
	// as the wheel does: the caret, on the first line, goes out of sight
	application.viewer.List.Offset = 20 * application.viewer.List.RowHeight
	window.Frames(2)
	if offset := application.viewer.List.Offset; offset == 0 {
		test.Error("the code scrolled down went back up to the caret, with no key pressed")
	}
	press(window, input.KeyDown)
	if first, _ := application.viewer.List.Visible(); first > 1 {
		test.Errorf("the caret moved and the code did not come back to it: the first line in sight is %d", first)
	}
}

// navigationKeys move or choose in a pane, and type nothing: they may run a command alone.
var navigationKeys = []input.Key{input.KeyUp, input.KeyDown, input.KeyLeft, input.KeyRight, input.KeyHome, input.KeyEnd,
	input.KeyPageUp, input.KeyPageDown, input.KeyEnter, input.KeySpace, input.KeyTab, input.KeyBackspace, input.KeyEscape}

// No shortcut is a letter, a number or a sign alone, or with Shift only: such a key is typed into a
// field or a file by mistake, and on other keyboards it is another letter. Each has Command or
// Alt, is a function key, Shift pressed twice, or a key that moves.
func TestNoShortcutIsALetterNumberOrSignAlone(test *testing.T) {
	application, _ := readerOfUses(test)
	for _, each := range application.allCommands() {
		for _, press := range each.keys {
			functionKey := press.Key >= input.KeyF1 && press.Key <= input.KeyF12
			safe := press.Text == "" && (press.Command || press.Alt || press.Twice || functionKey || slices.Contains(navigationKeys, press.Key))
			if !safe {
				test.Errorf("%q runs %q alone", press.Label, each.description)
			}
		}
	}
}
