// Package settings is the reader's settings screen, in place of the sidebar and the file: its
// sections listed on the left (the appearance, the keys), the one chosen on the right. What it
// shows and changes comes from the window, through Window.
package settings

import (
	"github.com/akrck02/coddy/editor/internal/controls"
	"github.com/akrck02/coddy/editor/internal/icons"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// Section is one part of the settings, listed on the left of their screen.
type Section int

const (
	Appearance Section = iota // the themes and the icon themes
	Memory                    // how much memory the files read may keep
	Keys                      // every command with its keys
	Plugins                   // the plugins installed, each turned on or off
)

// sections are the settings' parts in the order they are listed: their name and icon.
var sections = []struct {
	section    Section
	name, icon string
}{
	{Appearance, "Appearance", "palette"},
	{Memory, "Memory", "memory-stick"},
	{Keys, "Keys", "keyboard"},
	{Plugins, "Plugins", "puzzle"},
}

// Window is what the settings show and change: the window's look and keys, its themes and icon
// themes, its commands' keys and its plugins.
type Window interface {
	Look() theme.Look
	Keys() pane.Keys
	// Themes are the colour themes to choose from, ThemeName the one painting the window, and
	// ThemesFolder where a person's own are read from
	Themes() []theme.Theme
	ThemeName() string
	ChooseTheme(context *ui.Context, chosen theme.Theme)
	ThemesFolder() string
	// IconThemes are the icon themes to choose from, IconTheme the one drawing the files (nil for
	// Lucide's), and IconPictures an icon theme's pictures, kept across frames
	IconThemes() []icons.Theme
	IconTheme() *icons.Theme
	ChooseIconTheme(context *ui.Context, icons *icons.Theme)
	IconPictures(icons *icons.Theme) *icons.Pictures
	IconThemesFolder() string
	// KeySections are the actions and their keys, a section for each topic
	KeySections() []KeySection
	// KeyProfiles are the names of the profiles of keys, Coddy's own first; KeyProfile is the one
	// chosen, and ChooseKeyProfile chooses another
	KeyProfiles() []string
	KeyProfile() string
	ChooseKeyProfile(name string)
	// NewKeyProfile makes a profile, a copy of the one chosen, and chooses it; RenameKeyProfile
	// and RemoveKeyProfile rename and delete the one chosen
	NewKeyProfile(name string) error
	RenameKeyProfile(name string) error
	RemoveKeyProfile() error
	// KeyConflict is the description of the action keys run already, where the action of an id
	// works too, "" for none; SetKey makes them that action's key at slot, in the profile chosen,
	// taking them from the other; ResetKeys gives an action Coddy's keys again
	KeyConflict(id string, press pane.Press) string
	SetKey(id string, slot int, press pane.Press) error
	ResetKeys(id string) error
	// DocumentMegabytes is how much memory the files read may keep, ChooseDocumentMegabytes
	// changes it, and DocumentUse is how many are kept and about how many bytes they take
	DocumentMegabytes() int
	ChooseDocumentMegabytes(megabytes int)
	DocumentUse() (documents, bytes int)
	// Plugins are the plugins installed, TurnPlugin turns one on or off, InstallMissing installs
	// the programs one misses, in the background, OpenPluginsFolder shows the folder they are
	// installed in, and PluginPictures keep the pictures of their badges, across frames
	Plugins() []*plugins.Plugin
	PluginPictures() *icons.Pictures
	TurnPlugin(plugin *plugins.Plugin, on bool)
	// ChoosePluginSetting sets one of a plugin's settings, saved at once, and tells what failed
	ChoosePluginSetting(plugin *plugins.Plugin, id string, value any) error
	InstallMissing(plugin *plugins.Plugin)
	// Restart closes the window and starts Coddy again, so plugins turned on or off take part
	Restart()
	OpenPluginsFolder()
}

// Screen is the settings' screen: whether it shows, and at which section.
type Screen struct {
	Open    bool
	Section Section
	// useWords are the memory section's words on what the files kept take, budgetName and
	// budgetFiles on the budget chosen, each made when what it tells changes
	useWords    pane.Words[documentUse]
	budgetName  pane.Words[int]
	budgetFiles pane.Words[int]
	// slidTo is the budget, in megabytes, the slider is being moved to and not let go of yet; 0
	// while it is not moved
	slidTo int
	// pluginTab is the plugins' category shown, an index of pluginCategories: All at first
	pluginTab int
	// pluginsWidth is the width the cards of plugins had in the last frame, in pixels
	pluginsWidth float32
	// pluginCounts are the words of each category's count, made when the plugins listed change
	// (countedPlugins is how many were counted)
	pluginCounts   []string
	countedPlugins int
	// descriptions are the plugins' descriptions broken into lines, kept from frame to frame
	descriptions map[*plugins.Plugin]*description
	// pluginSettings is the sheet of a plugin's settings, nil while none shows
	pluginSettings *pluginSettings
	// prerequisites is the sheet of what a plugin needs before it works, nil while none shows
	prerequisites *prerequisites
	// keys is what the keys section keeps between frames
	keys keysEditor
	// restartWaits is true once a plugin was turned on or off whose change waits for the next
	// start, so the section says so
	restartWaits bool
}

const (
	// settingsListWidth is the width of the list of sections
	settingsListWidth = 220
	// settingsPadding is the room around the section shown
	settingsPadding = 28
)

// Toggle opens the settings at a section, or closes them when they show it already.
func (screen *Screen) Toggle(section Section) {
	if screen.Open && screen.Section == section {
		screen.Open = false
		return
	}
	screen.Open, screen.Section = true, section
}

// Build is the settings' screen, in place of the sidebar and the viewer: the list of its sections
// on the left, the one chosen on the right.
func (screen *Screen) Build(context *ui.Context, window Window) {
	// Esc closes them, as a dialog closes, unless it closes a sheet over them; the window's
	// commands wait while they are open
	if window.Keys().Pressed(input.KeyEscape) && screen.pluginSettings == nil && screen.prerequisites == nil &&
		screen.keys.recording == "" && screen.keys.naming == notNaming {
		screen.Open = false
	}
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(settingsListWidth), Height: ui.Grow(), Padding: ui.Pad(12), Gap: 4}, func() {
		context.Box(ui.Layout{Padding: ui.Padding{Left: 8, Top: 6, Bottom: 10}}, func() {
			context.Text("Settings", ui.TextStyle{Size: context.Theme.TextSize + 4, Weight: draw2d.Bold})
		})
		for _, each := range sections {
			screen.buildSectionButton(context, window, each.section, each.name, each.icon)
		}
	})
	context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Line}, nil)
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, func() {
		screen.buildSectionHead(context)
		context.ScrollArea("settings", ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func() {
			context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(settingsPadding), Gap: 16}, func() {
				switch screen.Section {
				case Keys:
					screen.buildKeys(context, window)
				case Memory:
					screen.buildMemory(context, window)
				case Plugins:
					screen.buildPlugins(context, window)
				default:
					buildAppearance(context, window)
				}
			})
		})
	})
	screen.buildPluginSettings(context, window)
	screen.buildPrerequisites(context, window)
	screen.buildNaming(context, window)
}

// buildSectionButton is a section in the list: its icon and name, filled while it is shown.
func (screen *Screen) buildSectionButton(context *ui.Context, window Window, section Section, name, icon string) {
	palette, look := context.Theme, window.Look()
	chosen := screen.Section == section
	ink, normal := palette.InkSoft, ui.Look{Radius: 6}
	if chosen {
		ink, normal = palette.Ink, ui.Look{Fill: look.Chosen, Radius: 6}
	}
	hovered := normal
	if !chosen {
		hovered.Fill = look.Hover
	}
	layout := ui.Layout{Width: ui.Grow(), Height: ui.Fixed(34), Padding: ui.PadXY(10, 0), Gap: 10, AlignY: ui.Center}
	if context.Clickable(name, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized(icon, 16, ink)
		context.Text(name, ui.TextStyle{Color: ink})
	}) {
		screen.Section = section
	}
}

// buildSectionHead is the bar over the section shown: its name, and the button that closes the
// settings.
func (screen *Screen) buildSectionHead(context *ui.Context) {
	name := ""
	for _, each := range sections {
		if each.section == screen.Section {
			name = each.name
		}
	}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(controls.BarHeight), Padding: ui.PadXY(controls.BarPadding, 0), AlignY: ui.Center}, func() {
		context.Text(name, ui.TextStyle{Weight: draw2d.Medium})
		context.Spacer()
		if controls.ToolButton(context, "Close the settings (Esc)", "x", false) {
			screen.Open = false
		}
	})
	context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
}
