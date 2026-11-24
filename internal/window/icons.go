package window

import (
	"github.com/akrck02/code-review/internal/settings"
	"github.com/akrck02/code-review/internal/theme"
	ui "github.com/akrck02/go-ui"
)

// findIconTheme is the icon theme of a name among some; nil, Lucide's icons, when it is not there.
func findIconTheme(themes []theme.IconTheme, name string) *theme.IconTheme {
	for index := range themes {
		if themes[index].Name == name {
			return &themes[index]
		}
	}
	return nil
}

// IconPictures are an icon theme's pictures, kept from the first time it is drawn: in the window,
// or in its card in the settings.
func (application *Window) IconPictures(icons *theme.IconTheme) *theme.IconPictures {
	if pictures, found := application.iconPictures[icons.Name]; found {
		return pictures
	}
	pictures := theme.NewIconPictures(func(problem error) {
		if application.tell != nil {
			application.tell(problem)
		}
	})
	application.iconPictures[icons.Name] = pictures
	return pictures
}

// ChooseIconTheme draws files and folders with an icon theme, nil for Lucide's, from this frame on.
func (application *Window) ChooseIconTheme(context *ui.Context, icons *theme.IconTheme) {
	application.iconTheme = icons
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
func (application *Window) IconThemes() []theme.IconTheme { return application.iconThemes }

// IconTheme is the icon theme drawing the files, nil for Lucide's.
func (application *Window) IconTheme() *theme.IconTheme { return application.iconTheme }

// IconThemesFolder is where a person's own icon themes are read from.
func (application *Window) IconThemesFolder() string {
	if application.settings.IconThemesFolder != "" {
		return application.settings.IconThemesFolder
	}
	return theme.IconThemesFolder()
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

// KeyGroups are every command's keys, grouped by where they work, for the settings.
func (application *Window) KeyGroups() []settings.KeyGroup {
	groups := []settings.KeyGroup{
		{Title: "Anywhere", Rows: helpRows(commands, anywhere)},
		{Title: "Modes", Rows: helpRows(application.modeCommands(), anywhere)},
	}
	if own := application.ownCommands(); len(own) > 0 {
		groups = append(groups, settings.KeyGroup{Title: "In the " + application.mode.Name() + " mode", Rows: helpRows(own, anywhere)})
	}
	return append(groups,
		settings.KeyGroup{Title: "In the sidebar", Rows: helpRows(commands, inSidebar)},
		settings.KeyGroup{Title: "In the file", Rows: helpRows(commands, inViewer)},
		settings.KeyGroup{Title: "While editing", Rows: append(helpRows(commands, editing), editingKeyRows()...)},
	)
}
