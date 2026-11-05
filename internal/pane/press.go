package pane

import (
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
	// Repeats runs the command again while the key is held down, as moving through rows does
	Repeats bool
	Step    int
	Label   string // how the help writes it
	// AnyModifiers runs the command whatever modifiers are held: for Esc, Tab, [ and ], which
	// have always answered with Command or Alt down too
	AnyModifiers bool
}

// Matches tells which of a command's keys the frame's input pressed.
func (press Press) Matches(keys Keys) bool {
	switch {
	case press.AnyModifiers:
		return keys.Pressed(press.Key)
	case press.Text != "":
		return strings.Contains(keys.TypedText(), press.Text)
	case press.Command:
		return keys.Command() && keys.Pressed(press.Key) && (!press.Shift || keys.Shift())
	case press.Shift && !keys.Shift():
		return false
	case press.Repeats:
		return keys.Typed(press.Key) && !keys.Command() && !keys.Down(input.KeyAlt)
	default:
		return keys.Plain(press.Key)
	}
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

// AnyKey is a key that runs its command whatever modifiers are held.
func AnyKey(code input.Key, label string) Press {
	return Press{Key: code, Label: label, AnyModifiers: true}
}
