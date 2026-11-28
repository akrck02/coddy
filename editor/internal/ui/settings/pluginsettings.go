package settings

import (
	"slices"
	"strings"

	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// pluginSettingsWidth is the width of the sheet of a plugin's settings, in pixels: room for a URL
const pluginSettingsWidth = 480

// spaceWidth and monoSpaceWidth are how wide a space is, in ems of its text: about a quarter of
// a letter's height in a proportional font, and a whole letter's cell, six tenths, in a code one
const (
	spaceWidth     = 0.28
	monoSpaceWidth = 0.6
)

// pluginSettings is the sheet of one plugin's settings while it shows: the plugin, the sheet's
// title, made as it opens, and what its fields hold while they are changed, by setting id.
type pluginSettings struct {
	plugin *plugins.Plugin
	title  string
	// drafts are the texts typed in the text and secret fields, saved as each field is left
	drafts map[string]string
	// choices are the index chosen of each choice
	choices map[string]int
	// descriptions are the words of each setting's description, split once as the sheet opens, so
	// that they wrap to the sheet's width instead of running past its edge
	descriptions map[string][]string
}

// OpenPluginSettings shows a plugin's settings, in a sheet over the plugins.
func (screen *Screen) OpenPluginSettings(plugin *plugins.Plugin) {
	sheet := &pluginSettings{plugin: plugin, title: plugin.Name + " settings",
		drafts: map[string]string{}, choices: map[string]int{}, descriptions: map[string][]string{}}
	for _, setting := range plugin.Settings {
		sheet.descriptions[setting.ID] = strings.Fields(setting.Description)
		value, _ := plugin.Values[setting.ID].(string)
		switch setting.Type {
		case plugins.SettingText, plugins.SettingSecret:
			sheet.drafts[setting.ID] = value
		case plugins.SettingChoice:
			sheet.choices[setting.ID] = max(0, slices.Index(setting.Choices, value))
		}
	}
	screen.pluginSettings = sheet
}

// buildPluginSettings is the sheet of a plugin's settings, when one is open: each setting with its
// title, what it is for and its control. Every change is saved as it is made, a text as its field
// is left, so the sheet has nothing to apply.
func (screen *Screen) buildPluginSettings(context *ui.Context, window Window) {
	sheet := screen.pluginSettings
	if sheet == nil {
		return
	}
	closing := false
	dismissed := context.Sheet(sheet.title, ui.SheetOptions{Width: ui.Fixed(pluginSettingsWidth)}, func() {
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 20, Padding: ui.Padding{Top: 4, Bottom: 12}}, func() {
			for _, setting := range sheet.plugin.Settings {
				sheet.buildSetting(context, window, setting)
			}
		})
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 12, AlignY: ui.Center}, func() {
			context.Caption("Saved as you change them. The plugin starts again with them.")
			context.Spacer()
			closing = context.PrimaryButton("Done")
		})
	})
	if dismissed || closing {
		sheet.saveDrafts(window)
		screen.pluginSettings = nil
	}
}

// buildSetting is one setting: a switch for a boolean, with its title beside it, or else its
// title over a field or a list of choices; under either, what it is for in soft ink.
func (sheet *pluginSettings) buildSetting(context *ui.Context, window Window, setting plugins.Setting) {
	context.Push(setting.ID)
	defer context.Pop()
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 8}, func() {
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 2}, func() {
			sheet.buildSettingTitle(context, window, setting)
			sheet.buildDescription(context, setting.ID)
		})
		switch setting.Type {
		case plugins.SettingChoice:
			sheet.buildChoice(context, window, setting)
		case plugins.SettingText, plugins.SettingSecret:
			sheet.buildField(context, window, setting)
		}
	})
}

// buildDescription is what a setting is for, in soft ink, its words wrapping onto as many lines
// as the sheet's width needs.
func (sheet *pluginSettings) buildDescription(context *ui.Context, id string) {
	words := sheet.descriptions[id]
	if len(words) == 0 {
		return
	}
	buildWrapped(context, words, ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkSoft})
}

// buildWrapped draws words as a paragraph that breaks onto as many lines as its box's width needs.
// Go UI draws each text on one line, so a text that may be long is split into words once, as it
// changes, and drawn here.
func buildWrapped(context *ui.Context, words []string, style ui.TextStyle) {
	size, space := style.Size, float32(spaceWidth)
	if size == 0 {
		size = context.Theme.TextSize
	}
	if style.Weight == draw2d.Mono {
		space = monoSpaceWidth
	}
	context.Box(ui.Layout{Width: ui.Grow(), Gap: size * space, LineGap: ui.NoLineGap, Wrap: true}, func() {
		for _, word := range words {
			context.Text(word, style)
		}
	})
}

// buildSettingTitle is a setting's title: for a boolean, a row with its switch at the right,
// saved as it is flipped.
func (sheet *pluginSettings) buildSettingTitle(context *ui.Context, window Window, setting plugins.Setting) {
	if setting.Type != plugins.SettingBoolean {
		context.Text(titleOf(setting), ui.TextStyle{Weight: draw2d.Medium})
		return
	}
	on, _ := sheet.plugin.Values[setting.ID].(bool)
	// the setting's own scope tells its switch from the others, so the title alone names it
	if context.Toggle(titleOf(setting), &on) {
		_ = window.ChoosePluginSetting(sheet.plugin, setting.ID, on) // the window tells what failed
	}
}

// titleOf is what a setting is called: its title, or its id when it has none.
func titleOf(setting plugins.Setting) string {
	if setting.Title == "" {
		return setting.ID
	}
	return setting.Title
}

// buildChoice is the list of a setting's choices, saved as one is chosen.
func (sheet *pluginSettings) buildChoice(context *ui.Context, window Window, setting plugins.Setting) {
	chosen := sheet.choices[setting.ID]
	if context.Select("##choice", &chosen, setting.Choices) {
		if window.ChoosePluginSetting(sheet.plugin, setting.ID, setting.Choices[chosen]) != nil {
			return // the window told what failed; the list shows the value kept again
		}
		sheet.choices[setting.ID] = chosen
	}
}

// buildField is the field of a text or a secret, masked for a secret. What is typed is saved once
// the field is left, not at each key: each save starts the plugin's program again.
func (sheet *pluginSettings) buildField(context *ui.Context, window Window, setting plugins.Setting) {
	draft := sheet.drafts[setting.ID]
	options := ui.TextFieldOptions{Label: titleOf(setting), Secret: setting.Type == plugins.SettingSecret}
	if context.TextFieldWith("##field", &draft, options) {
		sheet.drafts[setting.ID] = draft
	}
	if !context.FieldFocused("##field") {
		sheet.saveDraft(window, setting.ID)
	}
}

// saveDrafts saves the draft of every field, as the sheet closes: the one typed in last may not
// have been left yet.
func (sheet *pluginSettings) saveDrafts(window Window) {
	for id := range sheet.drafts {
		sheet.saveDraft(window, id)
	}
}

// saveDraft saves what a field holds when it differs from the value kept. A draft that could not
// be saved goes back to the value kept, so it is not tried again at each frame.
func (sheet *pluginSettings) saveDraft(window Window, id string) {
	kept, _ := sheet.plugin.Values[id].(string)
	if sheet.drafts[id] == kept {
		return
	}
	if err := window.ChoosePluginSetting(sheet.plugin, id, sheet.drafts[id]); err != nil {
		sheet.drafts[id] = kept
	}
}
