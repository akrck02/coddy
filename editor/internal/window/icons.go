package window

import (
	"github.com/akrck02/coddy/editor/internal/icons"
	"github.com/akrck02/coddy/editor/internal/theme"
	ui "github.com/akrck02/go-ui"
)

// findIconTheme is the icon theme of a name among some; nil, Lucide's icons, when it is not there.
func findIconTheme(themes []icons.Theme, name string) *icons.Theme {
	for index := range themes {
		if themes[index].Name == name {
			return &themes[index]
		}
	}
	return nil
}

// IconPictures are an icon theme's pictures, kept from the first time it is drawn: in the window,
// or in its card in the settings.
func (application *Window) IconPictures(iconTheme *icons.Theme) *icons.Pictures {
	if pictures, found := application.iconPictures[iconTheme.Name]; found {
		return pictures
	}
	pictures := icons.NewPictures(application.tellProblem)
	application.iconPictures[iconTheme.Name] = pictures
	return pictures
}

// ChooseIconTheme draws files and folders with an icon theme, nil for Lucide's, from this frame on.
func (application *Window) ChooseIconTheme(context *ui.Context, iconTheme *icons.Theme) {
	application.iconTheme = iconTheme
	application.applyTheme(context)
}

// IconThemeName is the name of the icon theme chosen, "" for Lucide's, to start in it the next time.
func (application *Window) IconThemeName() string {
	if application.iconTheme == nil {
		return ""
	}
	return application.iconTheme.Name
}

// IconThemes are the icon themes to choose from in the settings.
func (application *Window) IconThemes() []icons.Theme { return application.iconThemes }

// IconTheme is the icon theme drawing the files, nil for Lucide's.
func (application *Window) IconTheme() *icons.Theme { return application.iconTheme }

// IconThemesFolder is where a person's own icon themes are read from.
func (application *Window) IconThemesFolder() string {
	if application.settings.IconThemesFolder != "" {
		return application.settings.IconThemesFolder
	}
	return icons.ThemesFolder()
}

// Themes are the colour themes to choose from in the settings.
func (application *Window) Themes() []theme.Theme { return application.themes }

// ThemesFolder is where a person's own colour themes are read from.
func (application *Window) ThemesFolder() string {
	if application.settings.ThemesFolder != "" {
		return application.settings.ThemesFolder
	}
	return theme.Folder()
}

// ChooseTheme paints the window in a theme, from this frame on.
func (application *Window) ChooseTheme(context *ui.Context, chosen theme.Theme) {
	application.theme = chosen
	application.applyTheme(context)
}

// PluginPictures keep the pictures of the plugins' badges, for the settings.
func (application *Window) PluginPictures() *icons.Pictures { return application.pluginPictures }

// tellProblem tells of a picture that cannot be drawn, when the window was given how.
func (application *Window) tellProblem(problem error) {
	if application.tell != nil {
		application.tell(problem)
	}
}
