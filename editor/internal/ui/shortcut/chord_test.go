package shortcut

import (
	"errors"
	"testing"

	"github.com/akrck02/coddy/editor/internal/ui/pane"
	"github.com/akrck02/go-renderer/input"
)

func TestAChordWrittenIsReadBackAsTheSameKeys(test *testing.T) {
	for _, text := range []string{"Command+Shift+F", "Alt+F7", "F1", "Shift+F3", "Command+1", "Shift twice", "Command+,"} {
		press, err := Parse(text)
		if err != nil {
			test.Fatalf("%q could not be read: %v", text, err)
		}
		if written := Text(press); written != text {
			test.Errorf("%q was read and written back as %q", text, written)
		}
	}
}

func TestAChordIsShownWithTheSignsOfItsModifiers(test *testing.T) {
	cases := map[string]string{"Command+Shift+F": "⌘⇧F", "Alt+F7": "⌥F7", "Shift twice": "⇧⇧", "Ctrl+O": "⌘O"}
	for text, label := range cases {
		press, _ := Parse(text)
		if press.Label != label {
			test.Errorf("%q is shown as %q, want %q", text, press.Label, label)
		}
	}
}

func TestATextThatNamesNoKeysIsNotAChord(test *testing.T) {
	for _, text := range []string{"", "Command+", "Hyper+F", "Command+Wheel", "Alt twice"} {
		if _, err := Parse(text); !errors.Is(err, NotAChord) {
			test.Errorf("%q gave %v, want %v", text, err, NotAChord)
		}
	}
}

// A letter alone, or with Shift only, is typed into a field by mistake: it is never a safe chord.
func TestOnlyChordsThatTypeNothingAreSafe(test *testing.T) {
	cases := map[string]bool{"F": false, "Shift+F": false, "1": false, "Command+F": true, "Alt+V": true,
		"F7": true, "Shift+F7": true, "Shift twice": true, "Down": true}
	for text, safe := range cases {
		press, _ := Parse(text)
		if Safe(press) != safe {
			test.Errorf("%q safe is %v, want %v", text, !safe, safe)
		}
	}
}

func TestTheKeysPressedAreRecordedWithTheModifiersHeld(test *testing.T) {
	state := input.NewState()
	state.Feed().SetKey(input.KeyControl, true)
	state.Feed().SetKey(input.KeyShift, true)
	state.Feed().SetKey(input.KeyK, true)
	press, found := Recorded(pane.KeysOf(state))
	if !found || Text(press) != "Command+Shift+K" {
		test.Errorf("recorded %q (found %v), want Command+Shift+K", Text(press), found)
	}
}

func TestOnlyModifiersHeldRecordNothing(test *testing.T) {
	state := input.NewState()
	state.Feed().SetKey(input.KeyControl, true)
	if _, found := Recorded(pane.KeysOf(state)); found {
		test.Error("Control alone was recorded as a chord")
	}
}
