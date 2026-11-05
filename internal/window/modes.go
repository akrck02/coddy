package window

import (
	"strings"

	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/viewer"
	ui "github.com/akrck02/go-ui"
)

// startModes gives each mode its window and turns on the one named, or the first.
func (application *Window) startModes(modes []mode.Mode, name string) {
	if len(modes) == 0 {
		panic("the reader needs a mode") // main registers them; a reader without one cannot list
	}
	application.modes = modes
	application.mode = modes[0]
	for _, each := range modes {
		each.Start(application)
		if strings.EqualFold(each.Name(), name) {
			application.mode = each
		}
	}
	application.sidebar.Listing = application.mode
}

// setMode turns a mode on. The file open stays open, shown as the mode shows a file: on its
// changes or whole.
func (application *Window) setMode(chosen mode.Mode) {
	if application.mode == chosen {
		return
	}
	application.mode = chosen
	application.sidebar.Listing = chosen
	application.sidebar.Stale = true
	if chosen.ShowsChanges() {
		application.editor.SetMode(viewer.DiffView)
	} else {
		application.editor.SetMode(viewer.FileView)
	}
	if application.editor.IsOpen() {
		application.sidebar.RevealFile(application, application.editor.Reference)
	}
}

// stepMode turns on the mode at a place in the rail, counting from 1; 0 turns on the next one.
func (application *Window) stepMode(place int) {
	if place > 0 {
		if place <= len(application.modes) {
			application.setMode(application.modes[place-1])
		}
		return
	}
	for index, each := range application.modes {
		if each == application.mode {
			application.setMode(application.modes[(index+1)%len(application.modes)])
			return
		}
	}
}

// modeCommands are the keys of every mode, which work whatever mode is on, as each acts on its
// own state.
func (application *Window) modeCommands() []command {
	var result []command
	for _, each := range application.modes {
		for _, declared := range each.Commands() {
			do := declared.Do
			result = append(result, command{
				description: declared.Description,
				keys:        declared.Keys,
				do:          func(_ *Window, _ *ui.Context, step int) { do(step) },
			})
		}
	}
	return result
}

// allCommands are the window's commands, then the modes'.
func (application *Window) allCommands() []command {
	return append(append([]command{}, commands...), application.modeCommands()...)
}

// progress is how far the first mode with work to count has gone, for the status bar.
func (application *Window) progress() (done, total int) {
	for _, each := range application.modes {
		if done, total = each.Progress(); total > 0 {
			return done, total
		}
	}
	return 0, 0
}
