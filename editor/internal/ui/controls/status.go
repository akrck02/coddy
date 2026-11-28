package controls

import (
	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// StatusColour is a status's colour: green for new, amber for changed, red for deleted.
func StatusColour(palette ui.Theme, status git.Status) draw2d.Color {
	switch status {
	case git.Added:
		return palette.Positive
	case git.Deleted:
		return palette.Danger
	case git.Unchanged:
		return palette.Ink
	}
	return palette.Warning
}
