package app

import (
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// inputTap keeps the frame's input for the program's own shortcuts and painted views. go-ui's
// window hands its driver the input state before the interface runs, and that is the only place
// a program built with window.Run can see the keys itself.
type inputTap struct {
	state *input.State
}

func (tap *inputTap) Drive(state *input.State, context *ui.Context) { tap.state = state }
func (tap *inputTap) Show(drawing *draw2d.List)                     {}

// keys answers what the keyboard and mouse did in this frame; with no input yet it answers no.
type keys struct {
	state *input.State
}

func (keys keys) pressed(key input.Key) bool { return keys.state != nil && keys.state.Pressed(key) }

// typed is pressed, and again while the key is held down: for moving through rows.
func (keys keys) typed(key input.Key) bool { return keys.state != nil && keys.state.Typed(key) }

func (keys keys) down(key input.Key) bool { return keys.state != nil && keys.state.Down(key) }

// command tells whether the Command key (or Control, off a Mac) is held.
func (keys keys) command() bool { return keys.down(input.KeySuper) || keys.down(input.KeyControl) }

func (keys keys) shift() bool { return keys.down(input.KeyShift) }

// plain is a key pressed with no Command, Control or Alt: a one-letter shortcut.
func (keys keys) plain(key input.Key) bool {
	return keys.pressed(key) && !keys.command() && !keys.down(input.KeyAlt)
}

func (keys keys) clicked() bool {
	return keys.state != nil && keys.state.ButtonPressed(input.MouseLeft)
}

func (keys keys) holding() bool { return keys.state != nil && keys.state.Button(input.MouseLeft) }

func (keys keys) wheel() (horizontal, vertical float32) {
	if keys.state == nil {
		return 0, 0
	}
	horizontal, vertical = float32(keys.state.ScrollDeltaX), float32(keys.state.ScrollDelta)
	if keys.shift() && horizontal == 0 {
		horizontal, vertical = vertical, 0 // a mouse wheel with Shift scrolls sideways
	}
	return horizontal, vertical
}

func (keys keys) typedText() string {
	if keys.state == nil {
		return ""
	}
	return keys.state.TypedText()
}
