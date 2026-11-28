// Package shortcut names combinations of keys, and keeps profiles of them: which keys run each of
// Coddy's actions. Coddy's own profile is the keys its commands are given; a profile of the
// person's changes some of them, and is kept as a file.
package shortcut

import (
	"slices"
	"strings"

	"github.com/akrck02/coddy/editor/internal/ui/pane"
	"github.com/akrck02/go-renderer/failure"
	"github.com/akrck02/go-renderer/input"
)

// namedKey is a key as a chord's text writes it, and as the keys section shows it.
type namedKey struct {
	key         input.Key
	name, label string
}

// namedKeys are the keys a chord can hold besides its modifiers, by their names.
var namedKeys = keyNames()

// keyNames lists the letters, the numbers, the function keys and the keys that move or edit.
func keyNames() []namedKey {
	var names []namedKey
	for letter := input.KeyA; letter <= input.KeyZ; letter++ {
		name := string(rune('A' + letter - input.KeyA))
		names = append(names, namedKey{letter, name, name})
	}
	for digit := input.Key0; digit <= input.Key9; digit++ {
		name := string(rune('0' + digit - input.Key0))
		names = append(names, namedKey{digit, name, name})
	}
	for function := input.KeyF1; function <= input.KeyF12; function++ {
		name := "F" + string(rune('1'+function-input.KeyF1))
		if function > input.KeyF9 {
			name = "F1" + string(rune('0'+function-input.KeyF10))
		}
		names = append(names, namedKey{function, name, name})
	}
	return append(names,
		namedKey{input.KeyUp, "Up", "↑"}, namedKey{input.KeyDown, "Down", "↓"},
		namedKey{input.KeyLeft, "Left", "←"}, namedKey{input.KeyRight, "Right", "→"},
		namedKey{input.KeySpace, "Space", "Space"}, namedKey{input.KeyTab, "Tab", "Tab"},
		namedKey{input.KeyEnter, "Enter", "Enter"}, namedKey{input.KeyEscape, "Esc", "Esc"},
		namedKey{input.KeyPlus, "Plus", "+"}, namedKey{input.KeyMinus, "Minus", "-"},
		namedKey{input.KeyPageUp, "PageUp", "PgUp"}, namedKey{input.KeyPageDown, "PageDown", "PgDn"},
		namedKey{input.KeyLeftBracket, "[", "["}, namedKey{input.KeyRightBracket, "]", "]"},
		namedKey{input.KeyComma, ",", ","}, namedKey{input.KeyPeriod, ".", "."},
		namedKey{input.KeyBackspace, "Backspace", "Backspace"}, namedKey{input.KeyDelete, "Delete", "Delete"},
		namedKey{input.KeyHome, "Home", "Home"}, namedKey{input.KeyEnd, "End", "End"},
	)
}

// movingKeys move or choose in a pane, and type nothing: they may run an action alone.
var movingKeys = []input.Key{input.KeyUp, input.KeyDown, input.KeyLeft, input.KeyRight, input.KeyHome, input.KeyEnd,
	input.KeyPageUp, input.KeyPageDown, input.KeyEnter, input.KeySpace, input.KeyTab, input.KeyBackspace, input.KeyEscape}

// Safe tells whether a chord can run an action without being typed by mistake: it holds Command
// or Alt, is a function key, Shift pressed twice, or a key that moves. A letter, a number or a
// sign alone, or with Shift only, is typed into a field, and is another letter on other keyboards.
func Safe(press Press) bool {
	functionKey := press.Key >= input.KeyF1 && press.Key <= input.KeyF12
	return press.Text == "" && (press.Command || press.Alt || press.Twice || functionKey || slices.Contains(movingKeys, press.Key))
}

// Text writes a chord as a profile's file keeps it: "Command+Shift+F", "Alt+F7", "Shift twice".
func Text(press Press) string {
	if press.Twice {
		return nameOf(press.Key) + " twice"
	}
	var parts []string
	if press.Command {
		parts = append(parts, "Command")
	}
	if press.Alt {
		parts = append(parts, "Alt")
	}
	if press.Shift {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, nameOf(press.Key)), "+")
}

// Label writes a chord as the keys section shows it: "⌘⇧F", "⌥F7", "⇧⇧".
func Label(press Press) string {
	if press.Twice {
		return "⇧⇧"
	}
	label := ""
	if press.Command {
		label += "⌘"
	}
	if press.Alt {
		label += "⌥"
	}
	if press.Shift {
		label += "⇧"
	}
	for _, each := range namedKeys {
		if each.key == press.Key {
			return label + each.label
		}
	}
	return label + "?"
}

// Parse reads a chord written as Text writes it. The modifiers come in any order, and Control,
// Ctrl and Cmd are Command, as Coddy's Command is Control off a Mac.
func Parse(text string) (Press, error) {
	if name, twice := strings.CutSuffix(text, " twice"); twice {
		if name != "Shift" {
			return Press{}, failure.Wrap(NotAChord, text, nil)
		}
		return Press{Key: input.KeyShift, Twice: true, Label: "⇧⇧"}, nil
	}
	parts := strings.Split(text, "+")
	var press Press
	for _, modifier := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(modifier)) {
		case "command", "cmd", "control", "ctrl":
			press.Command = true
		case "alt", "option":
			press.Alt = true
		case "shift":
			press.Shift = true
		default:
			return Press{}, failure.Wrap(NotAChord, text, nil)
		}
	}
	key, found := keyNamed(strings.TrimSpace(parts[len(parts)-1]))
	if !found {
		return Press{}, failure.Wrap(NotAChord, text, nil)
	}
	press.Key = key
	press.Label = Label(press)
	return press, nil
}

// keyNamed is the key of a name, its case not minded for letters.
func keyNamed(name string) (input.Key, bool) {
	for _, each := range namedKeys {
		if strings.EqualFold(each.name, name) {
			return each.key, true
		}
	}
	return input.KeyUnknown, false
}

// nameOf is the name of a key in a chord's text.
func nameOf(key input.Key) string {
	if key == input.KeyShift {
		return "Shift"
	}
	for _, each := range namedKeys {
		if each.key == key {
			return each.name
		}
	}
	return "?"
}

// Recorded is the chord pressed in this frame, for a key being changed: the first key pressed
// that is not a modifier, with the modifiers held. It tells whether one was.
func Recorded(keys pane.Keys) (Press, bool) {
	for _, each := range namedKeys {
		if !keys.Pressed(each.key) {
			continue
		}
		press := Press{Key: each.key, Command: keys.Command(), Alt: keys.Alt(), Shift: keys.Shift()}
		press.Label = Label(press)
		return press, true
	}
	return Press{}, false
}
