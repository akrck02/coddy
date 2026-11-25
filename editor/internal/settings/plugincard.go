package settings

import (
	"strings"

	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// cardNarrowest is the narrowest a plugin's card gets: the grid takes as many columns as fit
	// cards at least this wide, in pixels
	cardNarrowest = 300
	// cardPadding is the room inside a card, in pixels
	cardPadding = 16
	// badgeSize is the side of a plugin's badge, in pixels
	badgeSize = 40
	// badgeRadius rounds the corners of a badge, in pixels
	badgeRadius = 10
	// badgeIconSize is the size of a Lucide icon inside its badge, in pixels
	badgeIconSize = 20
	// warningIconSize is the size of the icon that warns of what a plugin needs, in pixels
	warningIconSize = 16
	// verifiedIconSize is the size of the check beside a verified plugin's name, in pixels
	verifiedIconSize = 15
	// pictureDensity is how many pixels of a badge's picture a layout pixel holds: two, so that it
	// stays sharp on a screen of twice the density
	pictureDensity = 2
	// workingDotSize is the side of the dot at a working card's top right corner, in pixels
	workingDotSize = 8
	// restingAlpha is how opaque the description of a resting card is, from 0 to 1: faded
	// further than the soft ink of the working ones, whatever the theme's colours
	restingAlpha = 0.6
)

// warningInk is the soft amber of the icon that warns of a plugin. It is Coddy's own, not the
// theme's: themes give their Warning colour to modified files, blue in some.
var warningInk = draw2d.Color{0.85, 0.6, 0.15, 1}

// initialInk is the colour of the initial on an automatic badge: white, which every badge colour
// holds.
var initialInk = draw2d.Color{1, 1, 1, 1}

// pluginColumns is how many cards fit side by side in a width, one at least.
func pluginColumns(width float32) int {
	return max(1, int((width+pluginsGap)/(cardNarrowest+pluginsGap)))
}

// cardWidthOf is the width of each card when a width holds a number of columns.
func cardWidthOf(width float32, columns int) float32 {
	return (width - pluginsGap*float32(columns-1)) / float32(columns)
}

// fitColumns is how many columns the grid of cards takes, from the width the settings' section
// had in the last frame. Before there is one, it takes one column, and asks for another frame.
func (screen *Screen) fitColumns(context *ui.Context) int {
	area, found := context.Area("settings")
	if !found {
		context.RequestFrame(0)
		return 1
	}
	screen.pluginsWidth = area.Width - 2*settingsPadding
	return pluginColumns(screen.pluginsWidth)
}

// buildPlugin is one plugin's card: its badge, name and author, its description, and at the
// bottom a bar of its controls. The whole card is its power switch: a click turns the plugin on or
// off, and a working one shows a dot of colour at its top right. What needs attention is one quiet
// icon whose note says what, so a screen of plugins stays calm.
func (screen *Screen) buildPlugin(context *ui.Context, window Window, plugin *plugins.Plugin, cardWidth float32) {
	palette := context.Theme
	context.Push(plugin.Name)
	defer context.Pop()
	card := ui.Look{Fill: window.Look().Editor, Border: palette.Line, BorderWidth: 1, Radius: 12}
	ink := palette.Ink
	if !plugin.Ready() {
		// a plugin off, or missing a program, rests: only its outline on the page, with soft ink,
		// so the ones at work stand out. No fill of its own, since a theme's greys may be lighter
		// or darker than its cards, or tinted.
		card.Fill, ink = draw2d.Color{}, palette.InkSoft
	}
	layout := ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), Padding: ui.Pad(cardPadding), Gap: 12}
	content := func() {
		buildPluginTitle(context, window, plugin, ink)
		screen.buildDescription(context, plugin, cardWidth-2*cardPadding, !plugin.Ready())
		context.Spacer()
		screen.buildPluginBar(context, window, plugin)
	}
	if len(plugin.Missing) > 0 {
		// off until what it needs is installed: a click would change nothing
		context.Frame(layout, card, content)
		return
	}
	hovered := card
	hovered.Border = palette.Primary
	if context.Clickable(powerWords(plugin.On), layout, ui.Looks{Normal: card, Hovered: hovered, Pressed: hovered}, content) {
		window.TurnPlugin(plugin, !plugin.On)
		screen.restartWaits = screen.restartWaits || plugin.NeedsRestart()
	}
}

// buildPluginTitle is the top of a card: the badge, the name in ink with its check when it is
// verified, and under it the author, the version, and whether it is built in.
func buildPluginTitle(context *ui.Context, window Window, plugin *plugins.Plugin, ink draw2d.Color) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 12, AlignY: ui.Center}, func() {
		buildBadge(context, window, plugin.Badge)
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 4}, func() {
			context.Box(ui.Layout{Gap: 8, AlignY: ui.Center}, func() {
				context.Text(plugin.Name, ui.TextStyle{Size: palette.TextSize + 1, Weight: draw2d.Bold, Color: ink})
				if plugin.Verified {
					context.IconSized("badge-check", verifiedIconSize, palette.Primary)
				}
			})
			buildPluginSubtitle(context, plugin)
		})
		if plugin.Ready() {
			buildWorkingDot(context, window)
		}
	})
}

// buildWorkingDot is the dot of a working plugin, at the top of its card, in the theme's colour
// for what was added: the same green, or what the theme has instead, in every card.
func buildWorkingDot(context *ui.Context, window Window) {
	context.Box(ui.Layout{Direction: ui.Vertical, Height: ui.Fixed(badgeSize)}, func() {
		dot := ui.Layout{Width: ui.Fixed(workingDotSize), Height: ui.Fixed(workingDotSize)}
		context.Frame(dot, ui.Look{Fill: window.Look().Created, Radius: workingDotSize / 2}, func() {})
	})
}

// buildBadge is the picture that stands for a plugin: its own file, its Lucide icon on a wash of
// its colour, or its initial on its colour.
func buildBadge(context *ui.Context, window Window, badge plugins.Badge) {
	square := ui.Layout{Width: ui.Fixed(badgeSize), Height: ui.Fixed(badgeSize), AlignX: ui.Center, AlignY: ui.Center}
	if badge.Source != nil {
		if picture := window.PluginPictures().SourcePicture(badge.Initial+"builtin", badge.Source, badgeSize*pictureDensity); picture != nil {
			context.Image(picture, square, badgeRadius)
			return
		}
	}
	if badge.File != "" {
		if picture := window.PluginPictures().Picture(badge.File, badgeSize*pictureDensity); picture != nil {
			context.Image(picture, square, badgeRadius)
			return
		}
	}
	if badge.Icon != "" {
		context.Frame(square, ui.Look{Fill: badge.Colour.WithAlpha(0.16), Radius: badgeRadius}, func() {
			context.IconSized(badge.Icon, badgeIconSize, badge.Colour)
		})
		return
	}
	context.Frame(square, ui.Look{Fill: badge.Colour, Radius: badgeRadius}, func() {
		context.Text(badge.Initial, ui.TextStyle{Size: 19, Weight: draw2d.Bold, Color: initialInk})
	})
}

// buildPluginSubtitle is the line under a plugin's name: its author, its version, and "built in"
// for one compiled into Coddy, each a text of its own so that nothing is joined in a frame.
func buildPluginSubtitle(context *ui.Context, plugin *plugins.Plugin) {
	quiet := ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkSoft}
	context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
		separate := false
		for _, part := range [...]string{plugin.Author, plugin.Version, builtInWords(plugin)} {
			if part == "" {
				continue
			}
			if separate {
				context.Text("·", quiet)
			}
			context.Text(part, quiet)
			separate = true
		}
	})
}

// builtInWords say that a plugin is compiled into Coddy, "" for one installed.
func builtInWords(plugin *plugins.Plugin) string {
	if plugin.BuiltIn {
		return "built in"
	}
	return ""
}

// buildPluginBar is the bottom of a card, a row of icons at its right: the button that installs
// what it misses, when Coddy can; a warning icon whose note says what is wrong, and whose click
// shows how to get what it misses; and last, the gear of its settings when it has some.
func (screen *Screen) buildPluginBar(context *ui.Context, window Window, plugin *plugins.Plugin) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 2, AlignY: ui.Center}, func() {
		context.Spacer()
		switch {
		case plugin.Installing:
			context.Text("Installing…", ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft})
		case plugin.Installable:
			if theme.ToolButton(context, "Install what it needs", "download", false) {
				window.InstallMissing(plugin)
			}
		}
		if plugin.Warning != "" && buildWarning(context, plugin.Warning) && len(plugin.Missing) > 0 {
			screen.OpenPrerequisites(plugin)
		}
		if len(plugin.Settings) > 0 && theme.ToolButton(context, "Settings", "settings", false) {
			screen.OpenPluginSettings(plugin)
		}
	})
}

// powerWords say what a click on a card does: turn the plugin off when it is on, on when off.
func powerWords(on bool) string {
	if on {
		return "On: click to turn it off"
	}
	return "Off: click to turn it on"
}

// buildWarning is a quiet amber icon whose note, on hover, says what needs attention. It tells
// whether it was clicked: for a program missing, it opens the plugin's prerequisites.
func buildWarning(context *ui.Context, warning string) bool {
	normal := ui.Look{Radius: 6}
	hovered := ui.Look{Fill: context.Theme.Sunken, Radius: 6}
	layout := ui.Layout{Width: ui.Fixed(30), Height: ui.Fixed(28), AlignX: ui.Center, AlignY: ui.Center}
	clicked := context.Clickable("##warning", layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized("triangle-alert", warningIconSize, warningInk)
	})
	context.Tooltip(warning)
	return clicked
}

// description is a plugin's description broken into the two lines a card shows, kept until the
// description or the card's width change: breaking it measures every word.
type description struct {
	text          string
	width         float32
	first, second string
}

// buildDescription is a plugin's description, in two lines at most, faded when its card rests.
func (screen *Screen) buildDescription(context *ui.Context, plugin *plugins.Plugin, width float32, resting bool) {
	if plugin.Description == "" {
		return
	}
	if screen.descriptions == nil {
		screen.descriptions = map[*plugins.Plugin]*description{}
	}
	kept := screen.descriptions[plugin]
	if kept == nil || kept.text != plugin.Description || kept.width != width {
		size := context.Theme.TextSize
		measure := func(words string) float32 {
			return context.Measurer().MeasureText(words, size, draw2d.Regular, 1).Width
		}
		kept = &description{text: plugin.Description, width: width}
		kept.first, kept.second = splitInTwoLines(plugin.Description, width, measure)
		screen.descriptions[plugin] = kept
	}
	style := ui.TextStyle{Color: context.Theme.InkSoft}
	if resting {
		style.Color[3] *= restingAlpha
	}
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 2}, func() {
		context.Text(kept.first, style)
		if kept.second != "" {
			context.Text(kept.second, style)
		}
	})
}

// splitInTwoLines breaks a text between words into two lines that fit a width, the second cut
// short after a word, with an ellipsis, when the text is longer still. A first word wider than the width is
// left whole, to be cut by the card's edge.
func splitInTwoLines(text string, width float32, measure func(string) float32) (first, second string) {
	words := strings.Fields(text)
	taken := 1
	for taken < len(words) && measure(strings.Join(words[:taken+1], " ")) <= width {
		taken++
	}
	first = strings.Join(words[:min(taken, len(words))], " ")
	if taken >= len(words) {
		return first, ""
	}
	second = strings.Join(words[taken:], " ")
	if measure(second) <= width {
		return first, second
	}
	rest := words[taken:]
	for count := len(rest) - 1; count > 0; count-- {
		shortened := strings.TrimRight(strings.Join(rest[:count], " "), ",;:.") + "…"
		if measure(shortened) <= width {
			return first, shortened
		}
	}
	return first, "…"
}
