package main

import (
	"fmt"

	"github.com/akrck02/coddy/sdk/screen"
	ui "github.com/akrck02/go-ui"
	"github.com/akrck02/go-ui/uitest"
)

// fakeScreen is the fake's mode: a button, and a line that counts the frames drawn and the
// clicks, so a test sees when Coddy asked for a frame and whether a click reached the button.
type fakeScreen struct {
	frames, clicks int
}

// newFakeScreen measures with the fixed measurer Coddy's tests measure with, so no fonts are
// needed.
func newFakeScreen() *screen.Screen {
	counted := &fakeScreen{}
	return screen.NewWithMeasurer(counted.build, uitest.FixedMeasurer{})
}

func (counted *fakeScreen) build(context *ui.Context, _ *screen.Screen) {
	counted.frames++
	if context.Button("Count") {
		counted.clicks++
	}
	context.Text(fmt.Sprintf("frame %d, clicked %d", counted.frames, counted.clicks), ui.TextStyle{})
}
