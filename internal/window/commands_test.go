package window

import (
	"strings"
	"testing"

	"github.com/akrck02/go-renderer/input"
)

// overlaps tells whether two places can have the keyboard at once: anywhere overlaps every place.
func overlaps(first, second place) bool {
	return first == second || first == anywhere || second == anywhere
}

// sameKeys tells whether two key presses are the same keys, so one would hide the other.
func sameKeys(first, second keyPress) bool {
	return first.key == second.key && first.text == second.text && first.command == second.command && first.shift == second.shift
}

func TestNoTwoCommandsShareAKeyInOneMode(test *testing.T) {
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
						test.Errorf("%s is the key of both %q and %q", firstKey.label, first.description, second.description)
					}
				}
			}
		}
	}
}

func TestTheHelpListsEveryCommand(test *testing.T) {
	application := New(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true})
	window := newWindow(test, application)
	press(window, input.KeyF1)
	shown := strings.Join(strings.Fields(strings.Join(window.Texts(), " ")), " ")
	for _, command := range commands {
		if !strings.Contains(shown, command.description) {
			test.Errorf("the help does not say %q", command.description)
		}
		for _, press := range command.keys {
			if press.label == "" || !strings.Contains(shown, press.label) {
				test.Errorf("the help does not show the key %q of %q", press.label, command.description)
			}
		}
	}
}
