package shortcut

import (
	"github.com/akrck02/coddy/editor/internal/ui/pane"
	"strings"

	"github.com/akrck02/go-renderer/input"
)

// Press is a key, or a character typed, that runs a command; step tells the command which way
// to go when several keys share it (n is +1, m is −1).
type Press struct {
	Key input.Key
	// Text is a character typed instead of a key: for / and ?, which sit on different keys in
	// different keyboard layouts
	Text    string
	Command bool // with Command, or Control off a Mac
	Shift   bool
	Alt     bool // with Option (Alt off a Mac), as IntelliJ's ⌥F7
	// Repeats runs the command again while the key is held down, as moving through rows does
	Repeats bool
	Step    int
	Label   string // how the help writes it
	// AnyModifiers runs the command whatever modifiers are held: for Esc, Tab, [ and ], which
	// have always answered with Command or Alt down too
	AnyModifiers bool
	// Twice runs the command when the key is pressed twice in a row, quickly: IntelliJ's double
	// Shift. It works while editing, as two Shifts type nothing.
	Twice bool
}

// Matches tells which of a command's keys the frame's input pressed.
func (press Press) Matches(keys pane.Keys) bool {
	switch {
	case press.Twice:
		return keys.PressedTwice(press.Key)
	case press.AnyModifiers:
		return keys.Pressed(press.Key)
	case press.Text != "":
		return strings.Contains(keys.TypedText(), press.Text)
	case press.Command || press.Alt:
		return keys.Command() == press.Command && keys.Down(input.KeyAlt) == press.Alt && keys.Pressed(press.Key) &&
			press.Shift == keys.Shift()
	case press.Shift && !keys.Shift():
		return false
	case press.Repeats:
		return keys.Typed(press.Key) && !keys.Command() && !keys.Down(input.KeyAlt)
	default:
		// Shift held is another key: F7 must not take ⇧F7
		return keys.Plain(press.Key) && press.Shift == keys.Shift()
	}
}

// WorksWhileEditing tells whether the key can run a command while a file is being edited, where
// plain keys type: a key with Command, an F-key, or Esc.
func (press Press) WorksWhileEditing() bool {
	functionKey := press.Key >= input.KeyF1 && press.Key <= input.KeyF12
	return press.Text == "" && (press.Command || press.Alt || press.Twice || functionKey || press.Key == input.KeyEscape)
}

// Key is a plain key with its label.
func Key(code input.Key, label string) Press { return Press{Key: code, Label: label} }

// StepKey is a plain key that moves, by step, and repeats while it is held.
func StepKey(code input.Key, label string, step int) Press {
	return Press{Key: code, Label: label, Step: step, Repeats: true}
}

// CommandKey is a key pressed with Command.
func CommandKey(code input.Key, label string) Press {
	return Press{Key: code, Label: label, Command: true}
}

// CommandShiftKey is a key pressed with Command and Shift.
func CommandShiftKey(code input.Key, label string) Press {
	return Press{Key: code, Label: label, Command: true, Shift: true}
}

// AnyKey is a key that runs its command whatever modifiers are held.
func AnyKey(code input.Key, label string) Press {
	return Press{Key: code, Label: label, AnyModifiers: true}
}

// PageStep and EndStep stand for a page and for the far end, when a key moves a pane by them.
const (
	PageStep = 1 << 16
	EndStep  = 1 << 20
)
