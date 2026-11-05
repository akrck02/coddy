package app

import (
	"github.com/akrck02/bubble"
	"github.com/akrck02/bubble/goui"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// The type of the reader, in pixels, over Bubble's: a body of 14 on lines one and a half times as
// tall.
const (
	bodySize   = 14
	smallSize  = 12
	lineHeight = 1.5
)

// look is everything the reader is painted with: go-ui's theme for the controls, and the colours of
// the code it paints itself. The shapes and spacing are Bubble 4's; the colours are Zed's One Dark
// and One Light, because a reviewer spends hours on them: neutral greys with no tint of their own,
// an editor a step darker (or lighter) than the panels around it, and colour kept for what means
// something — green added, amber changed, red removed, blue for where the reader is.
type look struct {
	theme ui.Theme

	editor           draw2d.Color // behind the code
	lineNumber       draw2d.Color
	activeLineNumber draw2d.Color
	hover            draw2d.Color // a row under the pointer
	chosen           draw2d.Color // the selected row
	focusEdge        draw2d.Color // the selected row's edge while its pane has the keyboard
	scrollThumb      draw2d.Color

	created, modified, deleted draw2d.Color
	// the washes behind added and removed lines, and the stronger ones behind the words that
	// changed in them
	createdLine, deletedLine draw2d.Color
	createdWord, deletedWord draw2d.Color

	syntax *highlight.Palette
}

// designs are the light and the dark look, made once.
type designs struct {
	light, dark look
}

func newDesigns() designs {
	return designs{light: oneLight(), dark: oneDark()}
}

// oneDark is Zed's One Dark (assets/themes/one/one.json in zed-industries/zed).
func oneDark() look {
	hex := draw2d.MustHex
	colours := palette{
		background: hex("#3b414d"), surface: hex("#2f343e"), editor: hex("#282c33"),
		hover: hex("#363c46"), chosen: hex("#454a56"),
		border: hex("#464b57"), borderSoft: hex("#363c46"), focus: hex("#47679e"),
		ink: hex("#dce0e5"), inkSoft: hex("#a9afbc"), inkQuiet: hex("#878a98"),
		lineNumber: hex("#4e5a5f"), activeLineNumber: hex("#d0d4da"),
		accent: hex("#74ade8"), onAccent: hex("#282c33"), accentTint: hex("#293b5b"),
		created: hex("#a1c181"), modified: hex("#dec184"), deleted: hex("#d07277"),
		selection: hex("#74ade83d"), scrollThumb: hex("#c8ccd44c"),
		shadow: hex("#00000066"), scrim: hex("#0000004d"),
		lineWash: 0.12, wordWash: 0.32,
	}
	syntax := map[highlight.Role]draw2d.Color{
		highlight.Plain: hex("#dce0e5"), highlight.Keyword: hex("#b477cf"), highlight.String: hex("#a1c181"),
		highlight.Escape: hex("#878e98"), highlight.Function: hex("#73ade9"), highlight.TypeName: hex("#6eb4bf"),
		highlight.Comment: hex("#5d636f"), highlight.DocComment: hex("#878e98"), highlight.Number: hex("#bf956a"),
		highlight.Constant: hex("#dfc184"), highlight.Boolean: hex("#bf956a"), highlight.Attribute: hex("#74ade8"),
		highlight.Property: hex("#d07277"), highlight.Variable: hex("#acb2be"), highlight.Punctuation: hex("#acb2be"),
		highlight.Operator: hex("#6eb4bf"), highlight.Tag: hex("#74ade8"), highlight.Title: hex("#d07277"),
		highlight.Namespace: hex("#dce0e5"), highlight.Preprocess: hex("#b477cf"), highlight.Label: hex("#74ade8"),
	}
	return colours.look(bubble.ModeDark, syntax)
}

// oneLight is Zed's One Light.
func oneLight() look {
	hex := draw2d.MustHex
	colours := palette{
		background: hex("#dcdcdd"), surface: hex("#ebebec"), editor: hex("#fafafa"),
		hover: hex("#dfdfe0"), chosen: hex("#cacaca"),
		border: hex("#c9c9ca"), borderSoft: hex("#dfdfe0"), focus: hex("#7d82e8"),
		ink: hex("#242529"), inkSoft: hex("#58585a"), inkQuiet: hex("#7e8086"),
		lineNumber: hex("#b4b4bb"), activeLineNumber: hex("#44454b"),
		accent: hex("#5c78e2"), onAccent: hex("#fafafa"), accentTint: hex("#cbcdf6"),
		created: hex("#669f59"), modified: hex("#a48819"), deleted: hex("#d36151"),
		selection: hex("#5c78e23d"), scrollThumb: hex("#383a414c"),
		shadow: hex("#0000001f"), scrim: hex("#00000026"),
		lineWash: 0.14, wordWash: 0.30,
	}
	syntax := map[highlight.Role]draw2d.Color{
		highlight.Plain: hex("#242529"), highlight.Keyword: hex("#a449ab"), highlight.String: hex("#649f57"),
		highlight.Escape: hex("#7c7e86"), highlight.Function: hex("#5b79e3"), highlight.TypeName: hex("#3882b7"),
		highlight.Comment: hex("#a2a3a7"), highlight.DocComment: hex("#7c7e86"), highlight.Number: hex("#ad6e25"),
		highlight.Constant: hex("#c18401"), highlight.Boolean: hex("#ad6e25"), highlight.Attribute: hex("#5c78e2"),
		highlight.Property: hex("#d3604f"), highlight.Variable: hex("#242529"), highlight.Punctuation: hex("#242529"),
		highlight.Operator: hex("#3882b7"), highlight.Tag: hex("#5c78e2"), highlight.Title: hex("#d3604f"),
		highlight.Namespace: hex("#242529"), highlight.Preprocess: hex("#a449ab"), highlight.Label: hex("#5c78e2"),
	}
	return colours.look(bubble.ModeLight, syntax)
}

// palette is the handful of colours a look is made from, named as Zed's themes name them.
type palette struct {
	background, surface, editor, hover, chosen draw2d.Color
	border, borderSoft, focus                  draw2d.Color
	ink, inkSoft, inkQuiet                     draw2d.Color
	lineNumber, activeLineNumber               draw2d.Color
	accent, onAccent, accentTint               draw2d.Color
	created, modified, deleted                 draw2d.Color
	selection, scrollThumb, shadow, scrim      draw2d.Color
	lineWash, wordWash                         float32
}

// look takes Bubble's shapes, spacing and type for a mode and paints them in these colours: every
// colour of the theme is set here, so none of Bubble's paper (its warm beige) is left.
func (colours palette) look(mode bubble.Mode, syntax map[highlight.Role]draw2d.Color) look {
	options := bubble.DefaultOptions()
	options.Mode = mode
	options.Density = bubble.DensityCompact
	design, err := bubble.NewTheme(options)
	if err != nil {
		panic(err) // Bubble's own options are always valid
	}
	theme := goui.Theme(design)

	theme.Background, theme.Surface, theme.Sunken = colours.background, colours.surface, colours.hover
	theme.Ink, theme.InkSoft, theme.InkQuiet = colours.ink, colours.inkSoft, colours.inkQuiet
	theme.Placeholder = colours.inkQuiet.WithAlpha(0.8)
	theme.Line, theme.ControlLine, theme.FieldLine = colours.border, colours.border, colours.border
	theme.Primary, theme.OnPrimary, theme.PrimaryTint = colours.accent, colours.onAccent, colours.accentTint
	theme.Positive, theme.Warning, theme.Danger = colours.created, colours.modified, colours.deleted
	theme.DangerTint = colours.surface.Mix(colours.deleted, 0.18)
	theme.FocusRing, theme.Selection = colours.focus, colours.selection
	theme.Shadow, theme.Scrim = colours.shadow, colours.scrim
	theme.FlatInk, theme.ItemIconInk = colours.inkSoft, colours.inkSoft
	theme.Chosen, theme.ChosenInk = colours.chosen, colours.ink
	theme.CheckLine, theme.CheckedBorder = colours.inkQuiet, draw2d.Color{}
	theme.PrimaryEdge, theme.EdgeWidth = draw2d.Color{}, 0 // flat, as Zed's buttons
	theme.BackdropBlur = 0
	theme.FieldFocusEdge = true

	theme.TextSize, theme.SmallTextSize = bodySize, smallSize
	theme.LineHeight, theme.HeadingLineHeight = lineHeight, lineHeight

	return look{
		theme:            theme,
		editor:           colours.editor,
		lineNumber:       colours.lineNumber,
		activeLineNumber: colours.activeLineNumber,
		hover:            colours.hover,
		chosen:           colours.chosen,
		focusEdge:        colours.focus,
		scrollThumb:      colours.scrollThumb,
		created:          colours.created,
		modified:         colours.modified,
		deleted:          colours.deleted,
		createdLine:      colours.created.WithAlpha(colours.lineWash),
		deletedLine:      colours.deleted.WithAlpha(colours.lineWash),
		createdWord:      colours.created.WithAlpha(colours.wordWash),
		deletedWord:      colours.deleted.WithAlpha(colours.wordWash),
		syntax:           highlight.NewPalette(syntax),
	}
}

// iconChoice is one icon of a switch: its icon, and the words its tooltip says.
type iconChoice struct {
	icon, name string
}

const (
	segmentSize     = 28
	segmentIconSize = 16
	segmentInset    = 2
)

// iconSwitch is a row of icons on a soft pill, the chosen one on a raised fill: a switch between
// two or three views that takes little room. It returns true when the choice changes.
func iconSwitch(context *ui.Context, name string, selected *int, choices []iconChoice) bool {
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

// toolButton is an icon-only button of a toolbar; one that is on is drawn on the chosen fill and in
// the accent, so a setting's state reads at a glance without words.
func toolButton(context *ui.Context, name, icon string, on bool) bool {
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
