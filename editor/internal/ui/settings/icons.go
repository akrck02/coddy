package settings

import (
	"github.com/akrck02/coddy/editor/internal/ui/controls"
	"github.com/akrck02/coddy/editor/internal/ui/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// iconSampleSize is an icon's size in an icon theme's card, and iconSampleRow how many in a row
	iconSampleSize = 20
	iconSampleRow  = 4
)

// iconSamples are the files an icon theme's card shows, after a folder: kinds most projects have.
var iconSamples = []string{"main.go", "README.md", "package.json", "index.ts", "style.css", "photo.png", "Makefile"}

// buildIconThemes is the part of the appearance that chooses the file icons: Lucide's, tinted by
// the theme, and every icon theme found, each a card of some of its icons.
func buildIconThemes(context *ui.Context, window Window) {
	context.Text("File icons", ui.TextStyle{Weight: draw2d.Bold})
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 16, Wrap: true}, func() {
		if buildIconThemeCard(context, window, "Lucide", nil) {
			window.ChooseIconTheme(context, nil)
		}
		themes := window.IconThemes()
		for index := range themes {
			iconTheme := &themes[index]
			if buildIconThemeCard(context, window, iconTheme.Name, iconTheme) {
				window.ChooseIconTheme(context, iconTheme)
			}
		}
	})
	buildNote(context, "Icon themes of Zed: those installed in Zed, and extension folders with icon_themes/ in "+window.IconThemesFolder()+".")
}

// buildIconThemeCard is an icon theme's card: a folder and some files drawn with it on the
// editor's colour, and its name. It tells whether the card was clicked.
func buildIconThemeCard(context *ui.Context, window Window, name string, iconTheme *theme.IconTheme) bool {
	look := window.Look()
	look.Icons, look.Pictures = iconTheme, nil
	if iconTheme != nil {
		look.Pictures = window.IconPictures(iconTheme)
	}
	samples := []theme.Icon{controls.FolderIcon("src", false, look)}
	for _, file := range iconSamples {
		samples = append(samples, controls.FileIcon(file, look))
	}
	chosen := sameIconTheme(window.IconTheme(), iconTheme)
	layout := ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(themeCardWidth), Padding: ui.Pad(8), Gap: 8}
	label := "File icons: " + name
	clicked := context.Clickable(label, layout, cardLooks(context.Theme, chosen), func() {
		ground := ui.Look{Fill: look.Editor, Radius: 6, Border: look.Theme.Line, BorderWidth: 1}
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(12), Gap: 12}, ground, func() {
			for start := 0; start < len(samples); start += iconSampleRow {
				context.Box(ui.Layout{Width: ui.Grow(), Gap: 12, AlignX: ui.Center}, func() {
					for _, sample := range samples[start:min(start+iconSampleRow, len(samples))] {
						buildIconSample(context, sample, look.Pictures)
					}
				})
			}
		})
		buildCardName(context, name, chosen)
	})
	// read by what it chooses, as colour themes and icon themes can share a name (Catppuccin's do)
	context.Describe(ui.Description{Role: ui.ButtonRole, Label: label, Selected: chosen})
	return clicked
}

// buildIconSample draws an icon in a card: its picture, or Lucide's icon when it has none or the
// picture cannot be drawn.
func buildIconSample(context *ui.Context, icon theme.Icon, pictures *theme.Pictures) {
	if icon.Picture != "" && pictures != nil {
		if picture := pictures.Picture(icon.Picture, iconSampleSize*2); picture != nil {
			context.Image(picture, ui.Layout{Width: ui.Fixed(iconSampleSize), Height: ui.Fixed(iconSampleSize)}, 0)
			return
		}
	}
	context.IconSized(icon.Name, iconSampleSize, icon.Ink)
}

// sameIconTheme tells whether two icon themes are the one theme, nil being Lucide's.
func sameIconTheme(one, other *theme.IconTheme) bool {
	if one == nil || other == nil {
		return one == other
	}
	return one.Name == other.Name
}
