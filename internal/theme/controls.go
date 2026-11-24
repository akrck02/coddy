package theme

import (
	ui "github.com/akrck02/go-ui"
)

// IconChoice is one icon of a switch: its icon, and the words its tooltip says.
type IconChoice struct {
	Icon, Name string
}

const (
	segmentSize     = 28
	segmentIconSize = 16
	segmentInset    = 2
)

// IconSwitch is a row of icons on a soft pill, the chosen one on a raised fill: a switch between
// two or three views that takes little room. It returns true when the choice changes.
func IconSwitch(context *ui.Context, name string, selected *int, choices []IconChoice) bool {
	theme := context.Theme
	changed := false
	context.Push(name)
	defer context.Pop()
	pill := ui.Look{Fill: theme.Sunken, Radius: 6}
	context.Frame(ui.Layout{Padding: ui.Pad(segmentInset), Gap: 2, AlignY: ui.Center}, pill, func() {
		for index, choice := range choices {
			chosen := index == *selected
			ink, normal := theme.InkQuiet, ui.Look{Radius: 5}
			if chosen {
				ink, normal = theme.Ink, ui.Look{Fill: theme.Chosen, Radius: 5}
			}
			hovered := normal
			if !chosen {
				hovered.Fill = theme.Chosen.WithAlpha(0.5)
			}
			layout := ui.Layout{Width: ui.Fixed(segmentSize), Height: ui.Fixed(segmentSize - 4), AlignX: ui.Center, AlignY: ui.Center}
			if context.Clickable(choice.Name, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
				context.IconSized(choice.Icon, segmentIconSize, ink)
			}) && !chosen {
				*selected = index
				changed = true
			}
			context.Tooltip(choice.Name)
		}
	})
	return changed
}

// BarHeight is the height of a bar over a pane, as the title bar over the file, in pixels, and
// BarPadding the room it keeps at each side.
const (
	BarHeight  = 36
	BarPadding = 12
)

// ToolButton is an icon-only button of a toolbar; one that is on is drawn on the chosen fill and in
// the accent, so a setting's state reads at a glance without words.
func ToolButton(context *ui.Context, name, icon string, on bool) bool {
	theme := context.Theme
	ink, normal := theme.InkSoft, ui.Look{Radius: 6}
	if on {
		ink, normal = theme.Primary, ui.Look{Fill: theme.PrimaryTint, Radius: 6}
	}
	hovered := normal
	if !on {
		hovered.Fill = theme.Sunken
	}
	layout := ui.Layout{Width: ui.Fixed(30), Height: ui.Fixed(28), AlignX: ui.Center, AlignY: ui.Center}
	clicked := context.Clickable(name, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized(icon, 16, ink)
	})
	context.Tooltip(name)
	return clicked
}

// LabelledButton is a button of a toolbar with an icon and a word after it, for one whose icon alone
// would not say what it does.
func LabelledButton(context *ui.Context, name, icon, label string) bool {
	theme := context.Theme
	normal := ui.Look{Radius: 6}
	hovered := ui.Look{Fill: theme.Sunken, Radius: 6}
	layout := ui.Layout{Height: ui.Fixed(28), Padding: ui.PadXY(8, 0), Gap: 6, AlignY: ui.Center}
	clicked := context.Clickable(name, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized(icon, 16, theme.InkSoft)
		context.Text(label, ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkSoft})
	})
	context.Tooltip(name)
	return clicked
}

// FitText shortens a text with an ellipsis until it fits a width; it gives "" when not even its
// first letter and the ellipsis fit, so a narrow pane drops what it cannot show instead of
// pushing its buttons out.
func FitText(context *ui.Context, text string, style ui.TextStyle, width float32) string {
	size := style.Size
	if size == 0 {
		size = context.Theme.TextSize
	}
	measure := func(words string) float32 {
		return context.Measurer().MeasureText(words, size, style.Weight, 1).Width
	}
	if width <= 0 {
		return ""
	}
	if measure(text) <= width {
		return text
	}
	runes := []rune(text)
	for cut := len(runes) - 1; cut > 0; cut-- {
		shortened := string(runes[:cut]) + "…"
		if measure(shortened) <= width {
			return shortened
		}
	}
	return ""
}

// FittedText is a text shortened to fit a width, kept until the text, the style or the width
// change: shortening measures the text once per letter cut, too much to do each frame.
type FittedText struct {
	text, fitted string
	style        ui.TextStyle
	width        float32
	made         bool
}

// Fit is FitText, made again only when what it is made of changed since the last call.
func (kept *FittedText) Fit(context *ui.Context, text string, style ui.TextStyle, width float32) string {
	if kept.made && kept.text == text && kept.style == style && kept.width == width {
		return kept.fitted
	}
	kept.text, kept.style, kept.width, kept.made = text, style, width, true
	kept.fitted = FitText(context, text, style, width)
	return kept.fitted
}

// TextWidth is how wide a text is drawn in a style.
func TextWidth(context *ui.Context, text string, style ui.TextStyle) float32 {
	size := style.Size
	if size == 0 {
		size = context.Theme.TextSize
	}
	return context.Measurer().MeasureText(text, size, style.Weight, 1).Width
}
