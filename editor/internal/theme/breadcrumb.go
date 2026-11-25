package theme

import ui "github.com/akrck02/go-ui"

// CrumbPadding is the room each side of a crumb's text.
const CrumbPadding = 3

// CrumbChevronSize is the size of the chevron between two crumbs, in pixels.
const CrumbChevronSize = 12

// CrumbText draws a folder of a breadcrumb: its name, quiet, only to inform where a file is.
func CrumbText(context *ui.Context, label string) {
	context.Box(ui.Layout{Padding: ui.PadXY(CrumbPadding, 1), AlignY: ui.Center}, func() {
		context.Text(label, ui.TextStyle{Color: context.Theme.InkQuiet})
	})
}

// CrumbChevron draws the chevron that comes after a crumb, pointing to the next.
func CrumbChevron(context *ui.Context) {
	context.IconSized("chevron-right", CrumbChevronSize, context.Theme.InkQuiet.WithAlpha(0.6))
}
