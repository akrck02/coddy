// Package pluginmode is a mode a plugin adds: its screen, drawn by the plugin's program with go-ui,
// takes the viewer's place. Coddy asks the plugin for a frame only when the pointer did something
// over the screen or the plugin's last frame asked for another, and paints the last frame it got
// at every frame of its own, so a still screen costs the plugin nothing and a slow plugin shows its
// last picture.
package pluginmode

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/akrck02/coddy/editor/internal/mode"
	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/sidebar"
	"github.com/akrck02/coddy/editor/internal/workspace"
	"github.com/akrck02/coddy/sdk/screen"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
	"github.com/akrck02/go-ui/remote"
)

// defaultIcon is the rail's icon of a mode whose plugin names none.
const defaultIcon = "puzzle"

// TODO(akrck02): the keys and the typed text could reach the screen too, once a plugin needs a
// field; they are Coddy's shortcuts now. The screen has no accessibility tree yet either.

// Mode is a plugin's mode.
type Mode struct {
	plugin *plugins.Plugin
	window mode.Window
	// frame is the last frame the plugin drew, shown until it draws another
	frame remote.Frame
	// area is where the screen was painted last, in window pixels
	area draw2d.Rectangle
	// sentWidth and sentHeight are the size the last frame was asked for
	sentWidth, sentHeight float32
	// waiting are the inputs still to send, in order: moves are merged, a press or a release is
	// kept apart so a click made while the plugin draws is not lost
	waiting []remote.Input
	// last is the input sent or waiting last, to see what changed
	last remote.Input
	// asking tells whether a frame is being asked for now
	asking bool
}

// New is the mode of a plugin that contributes one.
func New(plugin *plugins.Plugin) *Mode { return &Mode{plugin: plugin} }

func (pluginMode *Mode) Name() string { return pluginMode.plugin.Contributes.Mode.Name }

// Icon is the plugin's icon for its mode, or a puzzle piece.
func (pluginMode *Mode) Icon() string {
	if icon := pluginMode.plugin.Contributes.Mode.Icon; icon != "" {
		return icon
	}
	return defaultIcon
}

func (pluginMode *Mode) Description() string {
	return pluginMode.Name() + ": a mode of the " + pluginMode.plugin.Name + " plugin"
}

func (pluginMode *Mode) Start(window mode.Window) { pluginMode.window = window }

func (pluginMode *Mode) ShowsChanges() bool { return false }

func (pluginMode *Mode) ChangesOnly() bool { return false }

// Decorate adds nothing to the sidebar's rows: the mode's own things are on its screen.
func (pluginMode *Mode) Decorate(sidebar.Row) sidebar.Decoration { return sidebar.Decoration{} }

// Covers is always true: the plugin's screen takes the viewer's place while the mode is on.
func (pluginMode *Mode) Covers() bool { return true }

// BuildOverViewer paints the plugin's last frame where the viewer is, and asks for a new one when
// the pointer did something over it, its size changed, or the plugin asked for another.
func (pluginMode *Mode) BuildOverViewer(context *ui.Context) bool {
	pluginMode.noteInput(context)
	if pluginMode.area.Width != pluginMode.sentWidth || pluginMode.area.Height != pluginMode.sentHeight {
		pluginMode.wait(pluginMode.last, true)
	}
	pluginMode.askNext()
	if pluginMode.area.Width == 0 {
		context.RequestFrame(0) // the first frame lays the screen out; the next one asks for it
	}
	context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		pluginMode.area = area
		remote.Show(pluginMode.frame, area, drawing)
	})
	return true
}

// noteInput keeps what the pointer did over the screen since the last frame, if anything.
func (pluginMode *Mode) noteInput(context *ui.Context) {
	x, y := context.Pointer()
	area := pluginMode.area
	inside := x >= area.X && y >= area.Y && x < area.X+area.Width && y < area.Y+area.Height
	given := remote.Input{PointerX: x - area.X, PointerY: y - area.Y, PointerDown: context.Pressing() && (inside || pluginMode.last.PointerDown)}
	if inside {
		_, given.Wheel = pluginMode.window.Keys().Wheel()
	}
	wasInside := pluginMode.last.PointerX >= 0 && pluginMode.last.PointerY >= 0 && pluginMode.last.PointerX < area.Width && pluginMode.last.PointerY < area.Height
	moved := given.PointerX != pluginMode.last.PointerX || given.PointerY != pluginMode.last.PointerY
	if !inside && !wasInside && given.PointerDown == pluginMode.last.PointerDown {
		return // the pointer is elsewhere, and was
	}
	if moved || given.Wheel != 0 || given.PointerDown != pluginMode.last.PointerDown {
		pluginMode.wait(given, given.PointerDown != pluginMode.last.PointerDown)
	}
}

// wait queues an input to send: kept apart when apart is true, else merged with the last one
// waiting.
func (pluginMode *Mode) wait(given remote.Input, apart bool) {
	pluginMode.last = given
	if count := len(pluginMode.waiting); count > 0 && !apart {
		given.Wheel += pluginMode.waiting[count-1].Wheel
		pluginMode.waiting[count-1] = given
		return
	}
	pluginMode.waiting = append(pluginMode.waiting, given)
}

// askNext asks the plugin for the frame of the next input waiting, off the frame, unless it is
// drawing one already.
func (pluginMode *Mode) askNext() {
	if pluginMode.asking || len(pluginMode.waiting) == 0 || pluginMode.area.Width == 0 {
		return
	}
	params := screen.Params{
		Width: pluginMode.area.Width, Height: pluginMode.area.Height,
		Theme: pluginMode.window.Look().Theme, Input: pluginMode.waiting[0], Folders: pluginMode.folders(),
	}
	pluginMode.waiting = pluginMode.waiting[1:]
	pluginMode.asking, pluginMode.sentWidth, pluginMode.sentHeight = true, params.Width, params.Height
	plugin := pluginMode.plugin
	pluginMode.window.Run("plugin mode "+plugin.Name, func(stop context.Context) func() {
		var result screen.Result
		err := plugin.DrawMode(stop, params, &result)
		return func() { pluginMode.take(result, err) }
	})
}

// take shows a frame the plugin drew, opens the place it chose, and asks again if it wants.
func (pluginMode *Mode) take(result screen.Result, err error) {
	pluginMode.asking = false
	pluginMode.plugin.Tell(err)
	if err != nil {
		pluginMode.window.Toast(err.Error())
		return // the last frame stays
	}
	pluginMode.frame = result.Frame
	if result.Open != nil {
		pluginMode.open(*result.Open)
	}
	if result.Frame.Again {
		pluginMode.wait(remote.Input{PointerX: pluginMode.last.PointerX, PointerY: pluginMode.last.PointerY, PointerDown: pluginMode.last.PointerDown}, false)
	}
}

// folders are the folders of the projects open, for the plugin.
func (pluginMode *Mode) folders() []string {
	var folders []string
	for _, project := range pluginMode.window.Projects() {
		folders = append(folders, project.Path)
	}
	return folders
}

// open opens a place the plugin chose, if it is in a project open.
func (pluginMode *Mode) open(place screen.Place) {
	for index, project := range pluginMode.window.Projects() {
		relative, err := filepath.Rel(project.Path, place.Path)
		if err != nil || strings.HasPrefix(relative, "..") {
			continue // not in this project
		}
		pluginMode.window.RevealLine(workspace.FileReference{Project: index, Path: filepath.ToSlash(relative)}, place.Line)
		return
	}
	pluginMode.window.Toast(place.Path + " is in no project open")
}
