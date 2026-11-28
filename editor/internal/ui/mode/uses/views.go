package uses

import (
	"fmt"

	"github.com/akrck02/coddy/editor/internal/ui/controls"
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

// Covers tells whether a graph takes the viewer's place.
func (uses *Mode) Covers() bool { return uses.name != "" && uses.view != listView }

// buildTabs is the bar over the viewer: the name searched, how far the search is, and the views.
func (uses *Mode) buildTabs(context *ui.Context) {
	palette := context.Theme
	look := uses.window.Look()
	context.Frame(ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(16, 6), Gap: 10, AlignY: ui.Center}, ui.Look{Fill: look.Editor}, func() {
		context.IconSized("scan-search", 14, palette.InkQuiet)
		context.Text(uses.name, ui.TextStyle{Weight: draw2d.Bold})
		context.Text(uses.tabWords.Of(uses.tally(), tabTitle), ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft})
		uses.buildNote(context)
		chosen := int(uses.view)
		if controls.IconSwitch(context, "uses view", &chosen, []controls.IconChoice{
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
				context.Text("Put the caret on a name, or click it, and press ⌥F7: its uses across the projects open are listed here.",
					ui.TextStyle{Color: palette.InkSoft})
				return
			}
			context.Text(uses.pageWords.Of(uses.tally(), pageTitle), ui.TextStyle{Color: palette.InkSoft})
			context.Caption("n / ⇧N go through the uses · v ticks one as seen · r searches again · ⇧1 ⇧2 ⇧3 the list and the graphs")
		})
	return true
}

// buildNote says how the search goes in the room left before the views' buttons, cut with "…"
// when it is longer, its whole in a tooltip: it grows with each server's progress and pushed the
// buttons out of the window.
func (uses *Mode) buildNote(context *ui.Context) {
	if uses.note == "" {
		context.Spacer()
		return
	}
	palette := context.Theme
	style := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Primary}
	height := palette.SmallTextSize * 1.6
	context.Clickable("uses note", ui.Layout{Width: ui.Grow(), Height: ui.Fixed(height)}, ui.Looks{}, func() {
		context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(height)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			if fitted := uses.fittedNote.Fit(context, uses.note, style, area.Width); fitted != "" {
				drawing.Text(fitted, area.X, area.Y+(area.Height+style.Size*0.7)/2, style.Size, draw2d.Regular, style.Color)
			}
		})
	})
	context.Tooltip(uses.note)
}

// usesTally is what the titles of the uses count, so they are written again only when it changes.
type usesTally struct {
	name                         string
	uses, files, seen, probables int
}

// tally counts the uses as the titles say them.
func (uses *Mode) tally() usesTally {
	return usesTally{name: uses.name, uses: len(uses.uses), files: len(uses.byFile), seen: uses.seenCount(), probables: uses.probableCount()}
}

// tabTitle is the tab's count of the uses: "4 uses in 3 files · 1 of 4 seen".
func tabTitle(tally usesTally) string {
	return fmt.Sprintf("%d uses in %d files · %s", tally.uses, tally.files, headerOf(tally))
}

// pageTitle is the page's count of the uses, with the name searched.
func pageTitle(tally usesTally) string {
	return fmt.Sprintf("%d uses of %s in %d files · %s", tally.uses, tally.name, tally.files, headerOf(tally))
}
