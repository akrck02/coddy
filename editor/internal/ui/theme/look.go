// Package theme is how the reader looks: its colour themes, read from files in Zed's format (Zed's
// One and IntelliJ's come with it), and what each file and folder is drawn as: Lucide's icon for its
// kind, or the picture an icon theme of Zed's names for it, read from SVG and drawn once per size.
package theme

import (
	"github.com/akrck02/bubble"
	"github.com/akrck02/bubble/goui"
	"github.com/akrck02/coddy/editor/internal/format/highlight"
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
// the code it paints itself. The shapes and spacing are Bubble 4's; the colours are a theme's.
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

	// Icons is the icon theme files and folders are drawn with, and Pictures its pictures, drawn
	// once; nil for Lucide's icons, tinted by the code's colours.
	Icons    *IconTheme
	Pictures *Pictures
}

// deepenInks takes every ink of a light theme a step deeper. Dark letters on a light ground are
// drawn thinner than light on dark, and the colours of an editor's light theme, drawn as they
// are, read grey and faint. The editors make up for it in their text renderer; the reader makes up
// for it in the colours: every ink a step deeper, the grounds as the theme has them.
func (colours *palette) deepenInks(syntax map[highlight.Role]draw2d.Color) {
	for role, colour := range syntax {
		syntax[role] = deeper(colour, lightInkDepth)
	}
	for _, colour := range []*draw2d.Color{
		&colours.ink, &colours.inkSoft, &colours.inkQuiet, &colours.lineNumber, &colours.activeLineNumber,
		&colours.created, &colours.modified, &colours.deleted,
	} {
		*colour = deeper(*colour, lightInkDepth)
	}
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
