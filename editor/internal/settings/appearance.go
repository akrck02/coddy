package settings

import (
	"strings"

	"github.com/akrck02/coddy/editor/internal/highlight"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// themeCardWidth and themePreviewHeight are a theme's card and the window drawn small in it
	themeCardWidth     = 220
	themePreviewHeight = 120
	// previewSidebarWidth is the sidebar's width in a theme's small window
	previewSidebarWidth = 52
	// previewLineHeight and previewWordHeight are a line of code in a small window, and the bar
	// standing for its words
	previewLineHeight = 13
	previewWordHeight = 5
)

// previewWord is a word of the code a theme's small window shows: its role and its width.
type previewWord struct {
	role  highlight.Role
	width float32
}

// previewCode is the code drawn in every theme's small window, a line a row: enough of each role
// to see how a theme colours code.
var previewCode = [][]previewWord{
	{{highlight.Comment, 96}},
	{{highlight.Keyword, 24}, {highlight.Function, 40}, {highlight.Plain, 16}},
	{{highlight.Keyword, 18}, {highlight.Plain, 22}, {highlight.String, 44}},
	{{highlight.Plain, 30}, {highlight.Number, 12}},
	{{highlight.Keyword, 26}, {highlight.Function, 32}},
	{{highlight.Plain, 10}},
}

// buildAppearance is the appearance section: every theme as a small window painted in it, the one
// painting the window marked; choosing one paints the window at once.
func buildAppearance(context *ui.Context, window Window) {
	context.Text("Theme", ui.TextStyle{Weight: draw2d.Bold})
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 16, Wrap: true}, func() {
		for _, each := range window.Themes() {
			if buildThemeCard(context, each, each.Name == window.ThemeName()) {
				window.ChooseTheme(context, each)
			}
		}
	})
	buildNote(context, "Your own themes: theme files of Zed (.json) in "+window.ThemesFolder()+". They are read when the reader starts.")
	buildIconThemes(context, window)
}

// buildThemeCard is a theme's card: its small window, and its name under it, ringed in the accent
// while it paints the window. It tells whether the card was clicked.
func buildThemeCard(context *ui.Context, shown theme.Theme, chosen bool) bool {
	layout := ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(themeCardWidth), Padding: ui.Pad(8), Gap: 8}
	return context.Clickable(shown.Name, layout, cardLooks(context.Theme, chosen), func() {
		buildThemePreview(context, shown.Look)
		buildCardName(context, shown.Name, chosen)
	})
}

// cardLooks are how a card of the settings looks: ringed in the accent while it is the one chosen.
func cardLooks(palette ui.Theme, chosen bool) ui.Looks {
	normal := ui.Look{Fill: palette.Sunken, Radius: 10, Border: palette.Line, BorderWidth: 1}
	if chosen {
		normal.Border, normal.BorderWidth = palette.Primary, 2
	}
	hovered := normal
	if !chosen {
		hovered.Border = palette.ControlLine
	}
	return ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}
}

// buildCardName is a card's name under its picture, with a check while it is the one chosen. A
// name too long for the card ends in "…" (the box clips), as Catppuccin's Monochrome ones would.
func buildCardName(context *ui.Context, name string, chosen bool) {
	layout := ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(4, 0), Gap: 6, AlignY: ui.Center}
	context.Frame(layout, ui.Look{Clip: true}, func() {
		context.Text(name, ui.TextStyle{Weight: draw2d.Medium})
		context.Spacer()
		if chosen {
			context.IconSized("circle-check", 16, context.Theme.Primary)
		}
	})
}

// buildThemePreview draws the window small in a theme's colours: the sidebar with a file chosen
// and one changed, and the editor with a few lines of coloured code.
func buildThemePreview(context *ui.Context, look theme.Look) {
	window := ui.Look{Fill: look.Editor, Radius: 6, Border: look.Theme.Line, BorderWidth: 1}
	context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(themePreviewHeight)}, window, func() {
		sidebar := ui.Look{Fill: look.Theme.Background, Radius: 6}
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(previewSidebarWidth), Height: ui.Grow(), Padding: ui.Pad(6), Gap: 5}, sidebar, func() {
			for _, row := range []draw2d.Color{look.Theme.InkSoft, look.Chosen, look.Created, look.Theme.InkSoft, look.Modified} {
				previewBar(context, previewSidebarWidth-16, row)
			}
		})
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(8), Gap: previewLineHeight - previewWordHeight}, func() {
			for _, line := range previewCode {
				context.Box(ui.Layout{Gap: 4}, func() {
					for _, word := range line {
						previewBar(context, word.width, look.Syntax.RoleColour(word.role))
					}
				})
			}
		})
	})
}

// previewBar is a rounded bar standing for a word, or a row, in a theme's small window.
func previewBar(context *ui.Context, width float32, colour draw2d.Color) {
	bar := ui.Layout{Width: ui.Fixed(width), Height: ui.Fixed(previewWordHeight)}
	context.Frame(bar, ui.Look{Fill: colour, Radius: previewWordHeight / 2}, nil)
}

// buildNote is a small note under a part of the settings, its words broken into lines at the
// page's width; a word too long for a line, as a deep folder can be, ends in "…".
func buildNote(context *ui.Context, note string) {
	style := ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkSoft}
	space := context.Measurer().MeasureText(" ", style.Size, style.Weight, 0).Width
	context.Frame(ui.Layout{Width: ui.Grow(), Gap: space, Wrap: true}, ui.Look{Clip: true}, func() {
		for _, word := range strings.Fields(note) {
			context.Text(word, style)
		}
	})
}
