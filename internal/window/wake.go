package window

import (
	"sync/atomic"

	platform "github.com/akrck02/go-ui/window"
)

// windowOpen is set once the window has drawn its first frame and cleared when it closes: the
// window can only be woken in between. Work that finishes before the first frame needs no waking,
// as that frame takes its result.
var windowOpen atomic.Bool

// wake asks the window for a frame, when there is a window to ask.
func wake() {
	if !windowOpen.Load() {
		return
	}
	defer func() {
		// TODO(akrck02): go-renderer's Wake panics once GLFW is terminated; fix it there and drop
		// this recover. The window closed between the check and the call: there is nothing to wake.
		_ = recover() // see the TODO above
	}()
	platform.Wake()
}
