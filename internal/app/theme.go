package app

import (
	"github.com/akrck02/bubble"
	"github.com/akrck02/bubble/goui"
	ui "github.com/akrck02/go-ui"
)

// Accent is the reader's colour: a quiet blue, so the green and red of the changes are what
// stands out.
const Accent = "#3B5B7E"

// The type of the reader, in pixels, over Bubble's: a body of 14 on lines one and a half times as
// tall.
const (
	bodySize   = 14
	smallSize  = 12
	lineHeight = 1.5
	focusShade = 0.55 // how far a focused field's line goes towards the accent
)

// designs are Bubble 4's paper theme in light and dark, on the reader's accent, made once: compact,
// as a reader of long lists of files wants as many rows in sight as can be read comfortably.
type designs struct {
	light, dark ui.Theme
}

func newDesigns() designs {
	return designs{light: designFor(bubble.ModeLight), dark: designFor(bubble.ModeDark)}
}

func designFor(mode bubble.Mode) ui.Theme {
	options := bubble.DefaultOptions()
	options.Mode = mode
	options.Density = bubble.DensityCompact
	if accent, err := bubble.ParseColor(Accent); err == nil {
		options.Accent = accent
	}
	theme, err := bubble.NewTheme(options)
	if err != nil {
		// Bubble refuses an accent it cannot make readable; its own accent always is
		options.Accent = bubble.DefaultAccent
		theme, _ = bubble.NewTheme(options)
	}
	return pageTheme(goui.Theme(theme))
}

// pageTheme is Bubble's theme with the reader's own text, and fields and buttons bordered by the
// soft line of cards, lit towards the accent when typed in instead of ringed.
func pageTheme(theme ui.Theme) ui.Theme {
	theme.TextSize, theme.SmallTextSize = bodySize, smallSize
	theme.LineHeight, theme.HeadingLineHeight = lineHeight, lineHeight
	theme.FieldLine, theme.ControlLine = theme.Line, theme.Line
	theme.FocusRing = theme.Line.Mix(theme.Primary, focusShade)
	return theme
}

// iconChoice is one segment of an icon switch: its icon, and the words its tooltip says.
type iconChoice struct {
	icon, name string
}

const (
	segmentSize     = 30
	segmentIconSize = 16
	segmentInset    = 3
)

// iconSwitch is a row of icons on a soft pill, the chosen one raised on the surface:
// a switch between two or three views that takes little room. It returns true
// when the choice changes.
func iconSwitch(context *ui.Context, name string, selected *int, choices []iconChoice) bool {
	theme := context.Theme
	changed := false
	context.Push(name)
	defer context.Pop()
	pill := ui.Look{Fill: theme.Sunken, Radius: (segmentSize + 2*segmentInset) / 2}
	context.Frame(ui.Layout{Padding: ui.Pad(segmentInset), Gap: 2, AlignY: ui.Center}, pill, func() {
		for index, choice := range choices {
			chosen := index == *selected
			ink, normal := theme.InkSoft, ui.Look{Radius: segmentSize / 2}
			if chosen {
				ink, normal = theme.Ink, ui.Look{Fill: theme.Surface, Radius: segmentSize / 2, Border: theme.Line, BorderWidth: 1}
			}
			hovered := normal
			if !chosen {
				hovered.Fill = theme.Surface.WithAlpha(0.5)
			}
			layout := ui.Layout{Width: ui.Fixed(segmentSize), Height: ui.Fixed(segmentSize), AlignX: ui.Center, AlignY: ui.Center}
			if context.Clickable(choice.name, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
				context.IconSized(choice.icon, segmentIconSize, ink)
			}) && !chosen {
				*selected = index
				changed = true
			}
			context.Tooltip(choice.name)
		}
	})
	return changed
}

// toggleIcon is a flat icon button that shows a setting's state by its icon and colour.
func toggleIcon(context *ui.Context, onIcon, offIcon, label string, on bool) bool {
	icon := offIcon
	if on {
		icon = onIcon
	}
	return context.FlatIconButton(icon, label)
}
