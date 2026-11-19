package viewer

import (
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// BuildTitle names the file open as a path to it, the folders quiet and the name plain, and says
// how it changed, in room pixels at most: a compact title drops the project and the status's
// word, and the folder takes what is left, cut from its start.
func (view *Viewer) BuildTitle(context *ui.Context, room float32, compact bool) {
	view.buildPath(context, room, compact)
	if view.Document == nil {
		context.Caption("Reading…")
		return
	}
	if view.Document.Changed {
		view.buildChange(context, compact)
	}
}

// BuildButtons are the ways to show the file open: its language, Markdown formatted or not, and,
// in a mode of changes (reviewing), for a changed file the changes' arrows, folding, the view and
// the mode's button.
func (view *Viewer) BuildButtons(context *ui.Context, window Window, compact, reviewing bool) {
	document := view.Document
	if document == nil {
		return
	}
	if !compact {
		context.Text(document.Language, ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkQuiet})
	}
	if view.IsMarkdown() && !document.Binary {
		view.buildFormatSwitch(context)
	}
	if reviewing && document.Changed && !view.Formatted() {
		view.buildViewButtons(context, window, compact)
	}
}

// ButtonsWidth is about how wide BuildButtons is, for the title to leave it room.
func (view *Viewer) ButtonsWidth(context *ui.Context, compact, reviewing bool) float32 {
	document := view.Document
	if document == nil {
		return 0
	}
	width := float32(0)
	if !compact {
		width += theme.TextWidth(context, document.Language, ui.TextStyle{Size: context.Theme.SmallTextSize}) + 12
	}
	if view.IsMarkdown() && !document.Binary {
		width += 40
	}
	if reviewing && document.Changed && !view.Formatted() {
		width += 260 // the arrows, the fold, the view and the viewed buttons
	}
	return width
}

// titleRoom is the width left for the file's folder once the rest of the title has its room.
func (view *Viewer) titleRoom(context *ui.Context, room float32, name string, compact bool) float32 {
	room -= theme.TextWidth(context, name, ui.TextStyle{Weight: draw2d.Medium}) + 40
	if !compact {
		room -= theme.TextWidth(context, view.Project.Name, ui.TextStyle{Color: context.Theme.InkQuiet}) + 20
	}
	if view.Document != nil && view.Document.Changed {
		room -= 70 // the counts
		if !compact {
			room -= 70 // the status's word
		}
	}
	return room
}

// buildPath writes the project, the folder and the file's name, coloured by how it changed.
func (view *Viewer) buildPath(context *ui.Context, room float32, compact bool) {
	palette := context.Theme
	document := view.Document
	folder, name := splitPath(view.Reference.Path)
	quiet := ui.TextStyle{Color: palette.InkQuiet}
	room = view.titleRoom(context, room, name, compact)
	if !compact {
		context.Text(view.Project.Name, quiet)
		context.IconSized("chevron-right", 12, palette.InkQuiet.WithAlpha(0.6))
	}
	if folder = theme.FitTextEnd(context, folder, quiet, room); len([]rune(folder)) > 3 {
		context.Text(folder, quiet)
		context.IconSized("chevron-right", 12, palette.InkQuiet.WithAlpha(0.6))
	}
	nameInk := palette.Ink
	if document != nil && document.Changed {
		nameInk = theme.StatusColour(palette, document.Change.Status)
	}
	context.Text(name, ui.TextStyle{Weight: draw2d.Medium, Color: nameInk})
	if view.Unsaved() {
		context.Text("●", ui.TextStyle{Color: palette.InkSoft}) // changes not saved
	}
}
