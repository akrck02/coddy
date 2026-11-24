package window

import (
	"fmt"
	"strings"

	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// startModes gives each mode its window and turns on the one named, or the first.
func (application *Window) startModes(modes []mode.Mode, name string) {
	application.modes = modes
	chosen := modes[0]
	for _, each := range modes {
		each.Start(application)
		if strings.EqualFold(each.Name(), name) {
			chosen = each
		}
		if holder, holds := each.(mode.UsesHolder); holds && application.usesMode == nil {
			application.usesMode = holder
		}
	}
	application.turnOn(chosen)
	application.commandTable = append(append(application.declaredCommands(true), commands...), application.modeCommands()...)
}

// setMode turns a mode on. The file open stays open, shown as the mode shows a file: on its
// changes or whole.
func (application *Window) setMode(chosen mode.Mode) {
	if application.mode == chosen {
		return
	}
	application.turnOn(chosen)
	application.sidebar.Stale = true
	application.viewer.SetUses(application.usesOfFile(application.viewer.Reference))
	switch {
	case len(application.viewer.Uses) > 0:
		application.viewer.SetMode(viewer.UsesView)
	case chosen.ShowsChanges():
		application.viewer.SetMode(viewer.DiffView)
	default:
		application.viewer.SetMode(viewer.FileView)
	}
	if application.viewer.IsOpen() {
		application.sidebar.RevealFile(application, application.viewer.Reference)
	}
}

// modePowers are what the mode that is on can do besides what every mode does, each nil when it
// cannot: found once, when the mode is turned on, not at each call or frame.
type modePowers struct {
	button   mode.WithFileButton
	progress mode.WithProgress
	header   sidebar.Headed
	page     mode.WithPage
	cover    mode.Cover
	uses     mode.UsesHolder
}

// turnOn makes a mode the one on, for the window and its sidebar, and finds what it can do.
func (application *Window) turnOn(chosen mode.Mode) {
	application.mode = chosen
	application.sidebar.SetListing(chosen)
	application.powers = modePowers{}
	application.powers.button, _ = chosen.(mode.WithFileButton)
	application.powers.progress, _ = chosen.(mode.WithProgress)
	application.powers.header, _ = chosen.(sidebar.Headed)
	application.powers.page, _ = chosen.(mode.WithPage)
	application.powers.cover, _ = chosen.(mode.Cover)
	application.powers.uses, _ = chosen.(mode.UsesHolder)
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
		keys:        []pane.Press{pane.Key(input.KeyC, "c"), pane.CommandShiftKey(input.Key0, "⌘⇧0")},
		do:          func(application *Window, _ *ui.Context, _ int) { application.nextMode() },
	})
	return append(result, application.declaredCommands(false)...)
}

// declaredCommands are the keys the modes declare: those kept to while their mode is on (own), or
// the others, which work whatever mode is on, as each acts on its own state.
func (application *Window) declaredCommands(own bool) []command {
	var result []command
	for _, each := range application.modes {
		owner := each
		keyed, hasKeys := each.(mode.WithCommands)
		if !hasKeys {
			continue // checked once: the commands are listed when the modes start
		}
		for _, declared := range keyed.Commands() {
			if declared.OnlyWhenOn != own {
				continue
			}
			do := declared.Do
			found := command{description: declared.Description, keys: declared.Keys,
				do: func(_ *Window, _ *ui.Context, step int) { do(step) }}
			if own {
				found.applies = func(application *Window) bool { return application.mode == owner }
			}
			result = append(result, found)
		}
	}
	return result
}

// ownCommands are the keys of the mode on that are kept to it, for the help.
func (application *Window) ownCommands() []command {
	var own []command
	for _, each := range application.declaredCommands(true) {
		if each.applies(application) {
			own = append(own, each)
		}
	}
	return own
}

// allCommands are the keys of the mode that is on kept to it, then the window's, then the modes':
// made once the modes start, as they never change, so reading the keys each frame builds nothing.
func (application *Window) allCommands() []command { return application.commandTable }

// progress is how far the mode on has gone in its work, for the status bar, with the words the
// mode says it in; a mode with no work to count shows none. It is counted again only when the
// sidebar's rows are made again, as what it counts changes only then.
func (application *Window) progress() (done, total int, words string) {
	counted := &application.counted
	rebuilds := application.sidebar.Rebuilds()
	if counted.rebuilds != rebuilds || counted.mode != application.mode || !counted.made {
		counted.done, counted.total, counted.words = 0, 0, ""
		if application.powers.progress != nil {
			counted.done, counted.total = application.powers.progress.Progress()
		}
		if header := application.powers.header; header != nil && counted.total > 0 {
			counted.words = header.Header()
		}
		counted.rebuilds, counted.mode, counted.made = rebuilds, application.mode, true
	}
	return counted.done, counted.total, counted.words
}

// progressCount is the mode's progress as it was counted last, and when.
type progressCount struct {
	made        bool
	rebuilds    int
	mode        mode.Mode
	done, total int
	words       string
}
