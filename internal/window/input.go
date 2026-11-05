package window

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
