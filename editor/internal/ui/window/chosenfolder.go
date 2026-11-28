package window

import (
	"strings"

	"github.com/akrck02/coddy/editor/internal/ui/controls"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// chosenFolderIconSize is the size of the search icon over a chosen folder's hint, in pixels
const chosenFolderIconSize float32 = 40

// chosenFolderHint is the line under the search icon: the title names the folder already
const chosenFolderHint = "Choose a file, or find one with ⇧⇧"

// chosenFolder is a folder chosen in the sidebar, shown in the file's place. The sidebar lists
// what it holds already, so the place only says what to do next; the title names the folder.
type chosenFolder struct {
	shown   bool
	project int    // the folder's project, its place in the window's projects
	path    string // the folder's path in its project, "" for its top
	// crumbs are the names of the title: the project, then each folder down to this one
	crumbs []string
}

// show chooses a folder of a project, with the name of its project.
func (folder *chosenFolder) show(project int, projectName, folderPath string) {
	folder.shown, folder.project, folder.path = true, project, folderPath
	folder.crumbs = append(folder.crumbs[:0], projectName)
	if folderPath != "" {
		folder.crumbs = append(folder.crumbs, strings.Split(folderPath, "/")...)
	}
}

// build is the hint: a search icon over a line that says what to do, in the middle of the place.
func (folder *chosenFolder) build(context *ui.Context) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), AlignX: ui.Center, AlignY: ui.Center, Gap: 14}, func() {
		context.IconSized("search", chosenFolderIconSize, palette.InkQuiet.WithAlpha(0.5))
		context.Text(chosenFolderHint, ui.TextStyle{Color: palette.InkSoft})
	})
}

// buildTitle names the folder as a breadcrumb that only informs: its project, then each folder
// down to it, the last one in ink. A path too long for the bar is cut at its end.
func (folder *chosenFolder) buildTitle(context *ui.Context) {
	context.Frame(ui.Layout{Width: ui.Grow(), Gap: 6, AlignY: ui.Center}, ui.Look{Clip: true}, func() {
		last := len(folder.crumbs) - 1
		for index, name := range folder.crumbs {
			if index == last {
				context.Text(name, ui.TextStyle{Weight: draw2d.Medium})
				return
			}
			controls.CrumbText(context, name)
			controls.CrumbChevron(context)
		}
	})
}
