package theme

import ui "github.com/akrck02/go-ui"

// CrumbPadding is the room each side of a crumb's text, where its hover shows.
const CrumbPadding = 3

// CrumbChevronSize is the size of the chevron between two crumbs, in pixels.
const CrumbChevronSize = 12

// Crumb draws a folder of a breadcrumb: a quiet button that shows label and is named name, for
// the screen reader and its tooltip. It tells whether it was pressed.
func Crumb(context *ui.Context, label, name string) bool {
	hovered := ui.Look{Fill: context.Theme.Sunken, Radius: 4}
	layout := ui.Layout{Padding: ui.PadXY(CrumbPadding, 1), AlignY: ui.Center}
	pressed := context.Clickable(name, layout, ui.Looks{Hovered: hovered, Pressed: hovered}, func() {
		context.Text(label, ui.TextStyle{Color: context.Theme.InkQuiet})
	})
	context.Tooltip(name)
	return pressed
}

// CrumbChevron draws the chevron that comes after a crumb, pointing to the next.
func CrumbChevron(context *ui.Context) {
	context.IconSized("chevron-right", CrumbChevronSize, context.Theme.InkQuiet.WithAlpha(0.6))
}
