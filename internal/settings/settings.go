// Package settings is the reader's settings screen, in place of the sidebar and the file: its
// sections listed on the left (the appearance, the keys), the one chosen on the right. What it
// shows and changes comes from the window, through Window.
package settings

import (
	"github.com/akrck02/code-review/internal/icons"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
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
)

// sections are the settings' parts in the order they are listed: their name and icon.
var sections = []struct {
	section    Section
	name, icon string
}{
	{Appearance, "Appearance", "palette"},
	{Memory, "Memory", "memory-stick"},
	{Keys, "Keys", "keyboard"},
}

// Window is what the settings show and change: the window's look and keys, its themes and icon
// themes, and its commands' keys.
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
	// KeyGroups are the commands' keys, a group for each place they work in
	KeyGroups() []KeyGroup
	// DocumentMegabytes is how much memory the files read may keep, ChooseDocumentMegabytes
	// changes it, and DocumentUse is how many are kept and about how many bytes they take
	DocumentMegabytes() int
	ChooseDocumentMegabytes(megabytes int)
	DocumentUse() (documents, bytes int)
}

// Screen is the settings' screen: whether it shows, and at which section.
type Screen struct {
	Open    bool
	Section Section
	// useWords are the memory section's words on what the files kept take, made when it changes
	useWords pane.Words[documentUse]
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
	// Esc closes them, as a dialog closes; the window's commands wait while they are open
	if window.Keys().Pressed(input.KeyEscape) {
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
					buildKeys(context, window)
				case Memory:
					screen.buildMemory(context, window)
				default:
					buildAppearance(context, window)
				}
			})
		})
	})
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
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(theme.BarHeight), Padding: ui.PadXY(theme.BarPadding, 0), AlignY: ui.Center}, func() {
		context.Text(name, ui.TextStyle{Weight: draw2d.Medium})
		context.Spacer()
		if theme.ToolButton(context, "Close the settings (Esc)", "x", false) {
			screen.Open = false
		}
	})
	context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
}
