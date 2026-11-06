package window

import (
	"fmt"
	"strings"

	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// startModes gives each mode its window and turns on the one named, or the first.
func (application *Window) startModes(modes []mode.Mode, name string) {
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
		application.viewer.SetMode(viewer.DiffView)
	} else {
		application.viewer.SetMode(viewer.FileView)
	}
	if application.viewer.IsOpen() {
		application.sidebar.RevealFile(application, application.viewer.Reference)
	}
}

// nextMode turns on the next mode on the rail, going round.
func (application *Window) nextMode() {
	for index, each := range application.modes {
		if each == application.mode {
			application.setMode(application.modes[(index+1)%len(application.modes)])
			return
		}
	}
}

// modeCommands are the keys that turn each mode on, by its place on the rail, and then the keys
// of every mode, which work whatever mode is on, as each acts on its own state.
func (application *Window) modeCommands() []command {
	var result []command
	for index, each := range application.modes {
		if index >= 9 {
			break // the number keys run out; the rail and c still reach the rest
		}
		chosen := each
		result = append(result, command{
			description: chosen.Name() + " mode",
			keys: []pane.Press{pane.Key(input.Key1+input.Key(index), fmt.Sprint(index+1)),
				pane.CommandKey(input.Key1+input.Key(index), fmt.Sprint("⌘", index+1))},
			do: func(application *Window, _ *ui.Context, _ int) { application.setMode(chosen) },
		})
	}
	result = append(result, command{
		description: "The next mode",
		keys:        []pane.Press{pane.Key(input.KeyC, "c"), pane.CommandKey(input.Key0, "⌘0")},
		do:          func(application *Window, _ *ui.Context, _ int) { application.nextMode() },
	})
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

// progress is how far the first mode with work to count has gone, for the status bar, with the
// words the mode says it in.
func (application *Window) progress() (done, total int, words string) {
	for _, each := range application.modes {
		if done, total = each.Progress(); total > 0 {
			return done, total, each.Header()
		}
	}
	return 0, 0, ""
}
