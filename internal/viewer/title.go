package viewer

import (
	"strings"

	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// BuildTitle names the file open as a breadcrumb to it, the folders quiet and the name plain, and
// says how it changed, in room pixels at most: a compact title drops the project and the status's
// word, and the folders take what is left, those nearest the file first. A folder's crumb shows
// what that folder holds.
func (view *Viewer) BuildTitle(context *ui.Context, window Window, room float32, compact bool) {
	view.buildPath(context, window, room, compact)
	if view.Document == nil {
		context.Caption("Reading…")
		return
	}
	if view.Document.Changed && view.showsStatus(window) {
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
func (view *Viewer) buildPath(context *ui.Context, window Window, room float32, compact bool) {
	palette := context.Theme
	document := view.Document
	folder, name := splitPath(view.Reference.Path)
	quiet := ui.TextStyle{Color: palette.InkQuiet}
	room = view.titleRoom(context, room, name, compact)
	if !compact {
		context.Text(view.Project.Name, quiet)
		theme.CrumbChevron(context)
	}
	for _, crumb := range view.crumbsOf(context, folder, room) {
		view.buildCrumb(context, window, crumb)
		theme.CrumbChevron(context)
	}
	nameInk := palette.Ink
	if document != nil && document.Changed && view.showsStatus(window) {
		nameInk = theme.StatusColour(palette, document.Change.Status)
	}
	context.Text(name, ui.TextStyle{Weight: draw2d.Medium, Color: nameInk})
	if view.Unsaved() {
		context.Text("●", ui.TextStyle{Color: palette.InkSoft}) // changes not saved
	}
}

// buildCrumb is a folder of the path, a button that shows what the folder holds in the viewer's
// place.
func (view *Viewer) buildCrumb(context *ui.Context, window Window, crumb crumb) {
	if theme.Crumb(context, crumb.label, crumb.name) {
		window.ShowFolder(view.Reference.Project, crumb.path)
	}
}

// crumbWidth is what a crumb takes besides its text: its padding, the chevron after it and the gaps
// around it.
const crumbWidth = 2*theme.CrumbPadding + theme.CrumbChevronSize + 2*6

// gathered stands for the folders of a breadcrumb left out for room.
const gathered = "…"

// crumb is a folder of a breadcrumb: the words it shows, the folder's path in its project, and
// the name of its button.
type crumb struct{ label, path, name string }

// keptCrumbs are the crumbs shown last, and what they were laid out for.
type keptCrumbs struct {
	laidOut    bool
	folder     string
	room, size float32
	shown      []crumb
}

// crumbsOf are the crumbs of a folder for room pixels, laid out again only when the folder, the
// room or the text's size changed: the title is drawn every frame, the path changes far less.
func (view *Viewer) crumbsOf(context *ui.Context, folder string, room float32) []crumb {
	kept := &view.crumbs
	size := context.Theme.TextSize
	if kept.laidOut && kept.folder == folder && kept.room == room && kept.size == size {
		return kept.shown
	}
	quiet := ui.TextStyle{Color: context.Theme.InkQuiet}
	width := func(text string) float32 { return theme.TextWidth(context, text, quiet) }
	*kept = keptCrumbs{laidOut: true, folder: folder, room: room, size: size, shown: crumbs(folder, width, room)}
	return kept.shown
}

// crumbs are the folders of a path, a crumb each, that fit in room pixels: those nearest the file,
// the ones before them gathered as one "…", which stands for the last of them; none for a file at
// the top of its project.
func crumbs(folder string, width func(string) float32, room float32) []crumb {
	if folder == "" {
		return nil
	}
	folders := strings.Split(strings.Trim(folder, "/"), "/")
	used, first := float32(0), len(folders)
	for first > 0 {
		next := used + width(folders[first-1]) + crumbWidth
		reserve := float32(0)
		if first > 1 {
			reserve = width(gathered) + crumbWidth // the others must still be told of
		}
		if next+reserve > room {
			break
		}
		used, first = next, first-1
	}
	var shown []crumb
	if first > 0 {
		path := strings.Join(folders[:first], "/")
		shown = append(shown, crumb{gathered, path, crumbName(path)})
	}
	for index := first; index < len(folders); index++ {
		path := strings.Join(folders[:index+1], "/")
		shown = append(shown, crumb{folders[index], path, crumbName(path)})
	}
	return shown
}

// crumbName is the name of a folder's crumb, which says what it does.
func crumbName(path string) string { return "Show the folder " + path }

// showsStatus tells whether the file says how it changed: always as a diff, which is its changes,
// and as a whole file unless the window hides git's status.
func (view *Viewer) showsStatus(window Window) bool {
	return view.Mode == DiffView || window.ShowsStatus()
}
