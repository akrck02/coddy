package settings

import (
	"strconv"

	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// chipHeight is the height of a category's chip, in pixels
	chipHeight = 32
	// chipIconSize is the size of a chip's icon, in pixels
	chipIconSize = 15
	// pluginsGap is the room between two cards, across and down, in pixels
	pluginsGap = 16
)

// pluginCategory is a chip of the plugins section: its name, its Lucide icon, and whether a
// plugin contributes to it.
type pluginCategory struct {
	name, icon  string
	contributes func(plugins.Contributions) bool
}

// allPlugins is the first chip, which shows every plugin: the one chosen at first.
var allPlugins = pluginCategory{"All", "layout-grid", func(plugins.Contributions) bool { return true }}

// pluginCategories are the chips of the plugins section, in order, All first. A plugin shows in
// each it contributes to.
var pluginCategories = []pluginCategory{
	allPlugins,
	{"Languages", "code", func(added plugins.Contributions) bool { return added.LanguageServer != nil }},
	{"Version control", "git-branch", func(added plugins.Contributions) bool { return added.VersionControl != nil }},
	{"Review", "git-pull-request", func(added plugins.Contributions) bool { return added.ReviewSource != nil }},
	{"Actions", "zap", func(added plugins.Contributions) bool { return len(added.Actions) > 0 }},
	{"Modes", "layout-panel-left", func(added plugins.Contributions) bool { return added.Mode != nil }},
}

// buildPlugins is the plugins section: its header, a chip for each category, and the plugins of
// the one chosen as a grid of cards.
func (screen *Screen) buildPlugins(context *ui.Context, window Window) {
	installed := window.Plugins()
	screen.countPlugins(installed)
	buildPluginsHeader(context, window)
	if screen.restartWaits {
		buildRestartBanner(context, window)
	}
	screen.buildPluginChips(context, window)
	category := pluginCategories[screen.pluginTab]
	columns := screen.fitColumns(context)
	cardWidth := cardWidthOf(screen.pluginsWidth, columns)
	context.Box(ui.Layout{Width: ui.Grow(), Gap: pluginsGap, Columns: columns}, func() {
		for _, plugin := range installed {
			if category.contributes(plugin.Contributes) || contributesNothing(plugin.Contributes) {
				screen.buildPlugin(context, window, plugin, cardWidth)
			}
		}
	})
	if screen.pluginCounts[screen.pluginTab] == "0" {
		context.Text("No plugin installed adds to this yet.", ui.TextStyle{Color: context.Theme.InkSoft})
	}
}

// buildPluginsHeader is what plugins are, and a button that opens the folder they are read from.
func buildPluginsHeader(context *ui.Context, window Window) {
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 12, AlignY: ui.Center}, func() {
		context.Text("Plugins teach Coddy new languages, version controls and tools. Each is on until you turn it off.",
			ui.TextStyle{Color: context.Theme.InkSoft})
		context.Spacer()
		if theme.LabelledButton(context, "Open the folder plugins are installed in", "folder-open", "Open folder") {
			window.OpenPluginsFolder()
		}
	})
}

// buildRestartBanner says that what was just turned on or off joins at the next start, with a
// button that starts Coddy again now.
func buildRestartBanner(context *ui.Context, window Window) {
	palette := context.Theme
	banner := ui.Look{Fill: palette.Primary.WithAlpha(0.1), Border: palette.Primary.WithAlpha(0.25), BorderWidth: 1, Radius: 8}
	context.Frame(ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(12, 10), Gap: 10, AlignY: ui.Center}, banner, func() {
		context.IconSized("rotate-cw", 15, palette.Primary)
		context.Text("Language servers, version controls and modes follow at the next start.", ui.TextStyle{Color: palette.Ink})
		context.Spacer()
		if context.PrimaryIconButton("rotate-cw", "Restart") {
			window.Restart()
		}
	})
}

// contributesNothing tells whether a plugin adds nothing, as one whose manifest could not be read:
// it shows in every category, so its state is seen.
func contributesNothing(added plugins.Contributions) bool {
	for _, category := range pluginCategories[1:] {
		if category.contributes(added) {
			return false
		}
	}
	return true
}

// countPlugins counts the plugins of each category, the words of its chip: again only when the
// plugins listed change, which they do not while Coddy runs.
func (screen *Screen) countPlugins(installed []*plugins.Plugin) {
	if screen.pluginCounts != nil && screen.countedPlugins == len(installed) {
		return
	}
	screen.pluginCounts, screen.countedPlugins = make([]string, len(pluginCategories)), len(installed)
	for index, category := range pluginCategories {
		count := 0
		for _, plugin := range installed {
			if category.contributes(plugin.Contributes) || contributesNothing(plugin.Contributes) {
				count++
			}
		}
		screen.pluginCounts[index] = strconv.Itoa(count)
	}
}

// buildPluginChips is the row of the categories' chips, each its icon, name and count; the one
// chosen is filled with the accent.
func (screen *Screen) buildPluginChips(context *ui.Context, window Window) {
	context.Push("plugin chips")
	defer context.Pop()
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 8, Wrap: true}, func() {
		for index, category := range pluginCategories {
			if buildTabChip(context, window.Look(), category.icon, category.name, screen.pluginCounts[index], index == screen.pluginTab) {
				screen.pluginTab = index
			}
		}
	})
}

// buildTabChip is a chip that chooses what a section shows, with its icon, name and count; the one
// chosen is filled with the accent. It tells whether it was clicked.
func buildTabChip(context *ui.Context, look theme.Look, icon, name, count string, chosen bool) bool {
	palette := context.Theme
	ink, countInk := palette.InkSoft, palette.InkQuiet
	normal := ui.Look{Border: palette.Line, BorderWidth: 1, Radius: chipHeight / 2}
	hovered := ui.Look{Fill: look.Hover, Border: palette.ControlLine, BorderWidth: 1, Radius: chipHeight / 2}
	if chosen {
		ink, countInk = palette.Primary, palette.Primary
		normal = ui.Look{Fill: palette.Primary.WithAlpha(0.14), Border: palette.Primary.WithAlpha(0.35), BorderWidth: 1, Radius: chipHeight / 2}
		hovered = normal
	}
	layout := ui.Layout{Height: ui.Fixed(chipHeight), Padding: ui.PadXY(14, 0), Gap: 8, AlignY: ui.Center}
	clicked := context.Clickable(name, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized(icon, chipIconSize, ink)
		context.Text(name, ui.TextStyle{Weight: draw2d.Medium, Color: ink})
		context.Text(count, ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Medium, Color: countInk})
	})
	context.Describe(ui.Description{Role: ui.TabRole, Label: name, Selected: chosen})
	return clicked
}
