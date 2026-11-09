package uses

import (
	"fmt"

	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// view is how the uses are shown: as a list, file by file in the sidebar and marked in each file,
// or as one of the two graphs over the viewer.
type view int

const (
	listView view = iota
	nameView
	projectsView
)

// viewNames name the views, for the help and the tabs.
var viewNames = []string{"a list", "a graph of the name", "a graph of the repositories"}

// BuildOverViewer shows the views' tabs over the viewer, and the graph chosen in its place; it
// tells whether a graph took the viewer's place.
func (uses *Mode) BuildOverViewer(context *ui.Context) bool {
	if uses.name == "" {
		return false
	}
	uses.buildTabs(context)
	switch uses.view {
	case nameView:
		uses.buildNameGraph(context)
		return true
	case projectsView:
		uses.buildProjectsGraph(context)
		return true
	}
	return false
}

// buildTabs is the bar over the viewer: the name searched, how far the search is, and the views.
func (uses *Mode) buildTabs(context *ui.Context) {
	palette := context.Theme
	look := uses.window.Look()
	context.Frame(ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(16, 6), Gap: 10, AlignY: ui.Center}, ui.Look{Fill: look.Editor}, func() {
		context.IconSized("scan-search", 14, palette.InkQuiet)
		context.Text(uses.name, ui.TextStyle{Weight: draw2d.Bold})
		context.Text(fmt.Sprintf("%d uses in %d files · %s", len(uses.uses), len(uses.byFile), uses.Header()),
			ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft})
		if uses.note != "" {
			context.Text(uses.note, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Primary})
		}
		context.Spacer()
		chosen := int(uses.view)
		if theme.IconSwitch(context, "uses view", &chosen, []theme.IconChoice{
			{Icon: "list", Name: "The uses, file by file (⇧1)"},
			{Icon: "share-2", Name: "Who uses the name (⇧2)"},
			{Icon: "network", Name: "The repositories and what they depend on (⇧3)"},
		}) {
			uses.view = view(chosen)
		}
	})
	context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: look.Hover}, nil)
}

// BuildInsteadOfAFile shows, while no file is open, how to search, or what the search found.
func (uses *Mode) BuildInsteadOfAFile(context *ui.Context) bool {
	palette := context.Theme
	look := uses.window.Look()
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), Padding: ui.PadXY(40, 32), Gap: 6},
		ui.Look{Fill: look.Editor}, func() {
			context.Text("Uses", ui.TextStyle{Size: palette.HeadingSize, Weight: draw2d.Bold})
			if uses.name == "" {
				context.Text("Put the caret on a name, or click it, and press ⇧F12: its uses across the projects open are listed here.",
					ui.TextStyle{Color: palette.InkSoft})
				return
			}
			context.Text(fmt.Sprintf("%d uses of %s in %d files · %s", len(uses.uses), uses.name, len(uses.byFile), uses.Header()),
				ui.TextStyle{Color: palette.InkSoft})
			context.Caption("n / ⇧N go through the uses · v ticks one as seen · r searches again · ⇧1 ⇧2 ⇧3 the list and the graphs")
		})
	return true
}
