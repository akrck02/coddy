package window

import (
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

// overlaps tells whether two places can have the keyboard at once: anywhere overlaps every place.
func overlaps(first, second place) bool {
	return first == second || first == anywhere || second == anywhere
}

// stopsEditingFirst tells whether a key shared is Esc in a file being edited, which comes first in
// the commands and stops editing before Esc can leave a project.
func stopsEditingFirst(first, second command, key pane.Press) bool {
	return key.Key == input.KeyEscape && first.place == editing && second.place != editing
}

// sameKeys tells whether two key presses are the same keys, so one would hide the other.
func sameKeys(first, second pane.Press) bool {
	return first.Key == second.Key && first.Text == second.Text && first.Command == second.Command && first.Shift == second.Shift
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
	// tall enough for the whole help, which scrolls in a shorter window
	window, err := uitest.New(application.Build, uitest.Driver(application.Driver()), uitest.Size(1200, 1600))
	if err != nil {
		test.Fatal(err)
	}
	press(window, input.KeyF1)
	shown := strings.Join(strings.Fields(strings.Join(window.Texts(), " ")), " ")
	for _, command := range application.allCommands() {
		if !strings.Contains(shown, command.description) {
			test.Errorf("the help does not say %q", command.description)
		}
		for _, press := range command.keys {
			if press.Label == "" || !strings.Contains(shown, press.Label) {
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
