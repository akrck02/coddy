package uses

import (
	ui "github.com/akrck02/go-ui"
)

// graphs are what the two graphs keep between frames.
type graphs struct{}

// buildNameGraph shows who uses the name.
func (uses *Mode) buildNameGraph(context *ui.Context) {
	context.Caption("The graph of the name comes next.")
}

// buildProjectsGraph shows the repositories and what they depend on.
func (uses *Mode) buildProjectsGraph(context *ui.Context) {
	context.Caption("The graph of the repositories comes next.")
}
