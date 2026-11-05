package window

import (
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/go-renderer/input"
)

// overlaps tells whether two places can have the keyboard at once: anywhere overlaps every place.
func overlaps(first, second place) bool {
	return first == second || first == anywhere || second == anywhere
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
					if sameKeys(firstKey, secondKey) {
						test.Errorf("%s is the key of both %q and %q", firstKey.Label, first.description, second.description)
					}
				}
			}
		}
	}
}

func TestTheHelpListsEveryCommand(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
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
