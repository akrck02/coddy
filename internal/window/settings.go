package window

import (
	"github.com/akrck02/code-review/internal/theme"
	ui "github.com/akrck02/go-ui"
)

// settingsSheetWidth is the settings' width: room for a theme's name and the themes folder.
const settingsSheetWidth = 460

// buildSettings shows the settings while they are open: the theme, chosen from a list that paints
// the window as soon as one is picked.
func (application *Window) buildSettings(context *ui.Context) {
	if !application.settingsOpen {
		return
	}
	dismissed := context.Sheet("Settings", ui.SheetOptions{Width: ui.Fixed(settingsSheetWidth)}, func() {
		context.Heading("Theme")
		chosen := application.themeIndex()
		if context.Select("theme", &chosen, application.themeNames()) {
			application.chooseTheme(context, application.themes[chosen])
		}
		folder := application.settings.ThemesFolder
		if folder == "" {
			folder = theme.Folder()
		}
		context.Caption("Your own themes: theme files of Zed (.json) in " + folder + ". They are read when the reader starts.")
		context.Row(func() {
			context.Spacer()
			if context.PrimaryButton("Done") {
				application.settingsOpen = false
			}
		})
	})
	if dismissed {
		application.settingsOpen = false
	}
}

// themeNames are the names the theme's list shows.
func (application *Window) themeNames() []string {
	names := make([]string, len(application.themes))
	for index, each := range application.themes {
		names[index] = each.Name
	}
	return names
}

// themeIndex is the place of the theme chosen in the list.
func (application *Window) themeIndex() int {
	for index, each := range application.themes {
		if each.Name == application.theme.Name {
			return index
		}
	}
	return 0
}

// chooseTheme paints the window in a theme, from this frame on.
func (application *Window) chooseTheme(context *ui.Context, chosen theme.Theme) {
	application.theme = chosen
	application.applyTheme(context)
}
