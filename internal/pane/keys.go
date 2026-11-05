// Package pane holds what every pane of the reader shares: the keys of the frame, and a list of rows
// that scrolls from the keyboard as well as by hand.
package pane

import (
	"github.com/akrck02/go-renderer/input"
)

// Keys answers what the keyboard and mouse did in this frame; with no input yet it answers no.
type Keys struct {
	state *input.State
}

// KeysOf answers for the input of one frame; nil is a frame with no input yet.
func KeysOf(state *input.State) Keys { return Keys{state: state} }

func (keys Keys) Pressed(key input.Key) bool { return keys.state != nil && keys.state.Pressed(key) }

// Typed is pressed, and again while the key is held down: for moving through rows.
func (keys Keys) Typed(key input.Key) bool { return keys.state != nil && keys.state.Typed(key) }

func (keys Keys) Down(key input.Key) bool { return keys.state != nil && keys.state.Down(key) }

// Command tells whether the Command key (or Control, off a Mac) is held.
func (keys Keys) Command() bool { return keys.Down(input.KeySuper) || keys.Down(input.KeyControl) }

func (keys Keys) Shift() bool { return keys.Down(input.KeyShift) }

// Plain is a key pressed with no Command, Control or Alt: a one-letter shortcut.
func (keys Keys) Plain(key input.Key) bool {
	return keys.Pressed(key) && !keys.Command() && !keys.Down(input.KeyAlt)
}

func (keys Keys) Clicked() bool {
	return keys.state != nil && keys.state.ButtonPressed(input.MouseLeft)
}

func (keys Keys) Holding() bool { return keys.state != nil && keys.state.Button(input.MouseLeft) }

func (keys Keys) Wheel() (horizontal, vertical float32) {
	if keys.state == nil {
		return 0, 0
	}
	horizontal, vertical = float32(keys.state.ScrollDeltaX), float32(keys.state.ScrollDelta)
	if keys.Shift() && horizontal == 0 {
		horizontal, vertical = vertical, 0 // a mouse wheel with Shift scrolls sideways
	}
	return horizontal, vertical
}

func (keys Keys) TypedText() string {
	if keys.state == nil {
		return ""
	}
	return keys.state.TypedText()
}

// PageStep and EndStep stand for a page and for the far end, when a key moves a pane by them.
const (
	PageStep = 1 << 16
	EndStep  = 1 << 20
)
