// Package screen is a plugin's mode drawn with go-ui: the plugin writes only its build function,
// and a Screen answers Coddy's mode/frame with what it drew. It is apart from the sdk package so a
// plugin with no screen does not depend on go-ui.
package screen

import (
	"encoding/json"

	"github.com/akrck02/coddy/sdk"
	"github.com/akrck02/coddy/sdk/jsonrpc"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/failure"
	"github.com/akrck02/go-renderer/opengl/overlay"
	ui "github.com/akrck02/go-ui"
	"github.com/akrck02/go-ui/remote"
)

// Params is what a frame of a mode's screen is drawn for: mode/frame's params.
type Params struct {
	Width  float32  `json:"width"`
	Height float32  `json:"height"`
	Theme  ui.Theme `json:"theme"`
	// Input is the pointer and the keys over the screen, from its top left corner.
	Input remote.Input `json:"input"`
	// Folders are the folders of the projects open.
	Folders []string `json:"folders"`
}

// Result is a frame of a mode's screen, and a place to open if the person chose one: mode/frame's
// result.
type Result struct {
	Frame remote.Frame `json:"frame"`
	Open  *Place       `json:"open,omitempty"`
}

// Place is a line of a file, from 1, for Coddy to open.
type Place struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// Screen draws a plugin's mode with go-ui.
type Screen struct {
	surface *remote.Surface
	asked   Params
	open    *Place
}

// New is a screen that draws build, measuring text in go-renderer's default fonts, those Coddy
// draws in, so its lines fit.
func New(build func(context *ui.Context, screen *Screen)) (*Screen, error) {
	measurer, err := overlay.NewMeasurer(overlay.Fonts{})
	if err != nil {
		return nil, err
	}
	return NewWithMeasurer(build, measurer), nil
}

// NewWithMeasurer is New with a measurer of your choice: tests use a fake one, so they need no
// fonts.
func NewWithMeasurer(build func(context *ui.Context, screen *Screen), measurer draw2d.Measurer) *Screen {
	screen := &Screen{}
	screen.surface = remote.NewSurface(func(context *ui.Context) { build(context, screen) }, measurer)
	return screen
}

// Folders are the folders of the projects open in Coddy.
func (screen *Screen) Folders() []string { return screen.asked.Folders }

// Open asks Coddy to open a line of a file, from 1, once this frame is drawn.
func (screen *Screen) Open(path string, line int) { screen.open = &Place{Path: path, Line: line} }

// Frame draws one frame.
func (screen *Screen) Frame(params Params) Result {
	screen.asked, screen.open = params, nil
	frame := screen.surface.Draw(params.Width, params.Height, params.Theme, params.Input)
	return Result{Frame: frame, Open: screen.open}
}

// Answer answers mode/frame: give it to sdk.Handlers.DrawMode.
func (screen *Screen) Answer(raw json.RawMessage) (any, error) {
	var params Params
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, failure.Wrap(jsonrpc.BadMessage, sdk.ModeFrameMethod, err)
	}
	return screen.Frame(params), nil
}
