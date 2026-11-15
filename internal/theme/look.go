// Package theme is how the reader looks: Zed's One colours, light and dark, the icon of each kind
// of file, and the few controls of its own it draws (tool buttons, icon switches, fitted text).
package theme

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
	BodySize   = 14
	SmallSize  = 12
	LineHeight = 1.5
)

// Look is everything the reader is painted with: go-ui's theme for the controls, and the colours of
// the code it paints itself. The shapes and spacing are Bubble 4's; the colours are Zed's One Dark
// and One Light, because a reviewer spends hours on them: neutral greys with no tint of their own,
// an editor a step darker (or lighter) than the panels around it, and colour kept for what means
// something — green added, amber changed, red removed, blue for where the reader is.
type Look struct {
	Theme ui.Theme

	Editor           draw2d.Color // behind the code
	LineNumber       draw2d.Color
	ActiveLineNumber draw2d.Color
	Hover            draw2d.Color // a row under the pointer
	Chosen           draw2d.Color // the selected row
	FocusEdge        draw2d.Color // the selected row's edge while its pane has the keyboard
	ScrollThumb      draw2d.Color

	Created, Modified, Deleted draw2d.Color
	// the washes behind added and removed lines, and the stronger ones behind the words that
	// changed in them
	CreatedLine, DeletedLine draw2d.Color
	CreatedWord, DeletedWord draw2d.Color

	Syntax *highlight.Palette
}

// Designs are the light and the dark look, made once.
type Designs struct {
	Light, Dark Look
}

func NewDesigns() Designs {
	return Designs{Light: oneLight(), Dark: oneDark()}
}

// oneDark is Zed's One Dark (assets/themes/one/one.json in zed-industries/zed).
func oneDark() Look {
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
func oneLight() Look {
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
		highlight.Property: hex("#d3604f"), highlight.Variable: hex("#242529"), highlight.Punctuation: hex("#4d4f52"),
		highlight.Operator: hex("#3882b7"), highlight.Tag: hex("#5c78e2"), highlight.Title: hex("#d3604f"),
		highlight.Namespace: hex("#242529"), highlight.Preprocess: hex("#a449ab"), highlight.Label: hex("#5c78e2"),
	}
	// Dark letters on a light ground are drawn thinner than light on dark, and Zed's colours,
	// drawn as they are, read grey and faint. Zed makes up for it in its text renderer; the reader
	// makes up for it in the colours: every ink a step deeper, the grounds as Zed has them.
	for role, colour := range syntax {
		syntax[role] = deeper(colour, lightInkDepth)
	}
	for _, colour := range []*draw2d.Color{
		&colours.ink, &colours.inkSoft, &colours.inkQuiet, &colours.lineNumber, &colours.activeLineNumber,
		&colours.created, &colours.modified, &colours.deleted,
	} {
		*colour = deeper(*colour, lightInkDepth)
	}
	return colours.look(bubble.ModeLight, syntax)
}

// lightInkDepth is how far the light theme's inks go towards black.
const lightInkDepth = 0.22

// deeper takes a colour towards black, keeping how see-through it is.
func deeper(colour draw2d.Color, amount float32) draw2d.Color {
	return colour.Mix(draw2d.Color{0, 0, 0, colour[3]}, amount)
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

// bubbleTheme is Bubble's theme for a mode, compact: its shapes, spacing and type.
func bubbleTheme(mode bubble.Mode) ui.Theme {
	options := bubble.DefaultOptions()
	options.Mode = mode
	options.Density = bubble.DensityCompact
	design, err := bubble.NewTheme(options)
	if err != nil {
		panic(err) // Bubble's own options are always valid
	}
	return goui.Theme(design)
}

// paint sets every colour of a theme, and the reader's text sizes.
func (colours palette) paint(theme ui.Theme) ui.Theme {
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

	theme.TextSize, theme.SmallTextSize = BodySize, SmallSize
	theme.LineHeight, theme.HeadingLineHeight = LineHeight, LineHeight
	return theme
}

// look takes Bubble's shapes, spacing and type for a mode and paints them in these colours: every
// colour of the theme is set here, so none of Bubble's paper (its warm beige) is left.
func (colours palette) look(mode bubble.Mode, syntax map[highlight.Role]draw2d.Color) Look {
	theme := colours.paint(bubbleTheme(mode))
	return Look{
		Theme:            theme,
		Editor:           colours.editor,
		LineNumber:       colours.lineNumber,
		ActiveLineNumber: colours.activeLineNumber,
		Hover:            colours.hover,
		Chosen:           colours.chosen,
		FocusEdge:        colours.focus,
		ScrollThumb:      colours.scrollThumb,
		Created:          colours.created,
		Modified:         colours.modified,
		Deleted:          colours.deleted,
		CreatedLine:      colours.created.WithAlpha(colours.lineWash),
		DeletedLine:      colours.deleted.WithAlpha(colours.lineWash),
		CreatedWord:      colours.created.WithAlpha(colours.wordWash),
		DeletedWord:      colours.deleted.WithAlpha(colours.wordWash),
		Syntax:           highlight.NewPalette(syntax),
	}
}
