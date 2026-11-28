package viewer

import (
	"strings"

	"github.com/akrck02/coddy/editor/internal/format/markdown"
	"github.com/akrck02/coddy/editor/internal/ui/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// formatter lays a Markdown text's blocks out, one under the other, in a column as wide as width.
type formatter struct {
	context *ui.Context
	palette ui.Theme
	look    theme.Look
	width   float32
	// codeSize and charWidth are the code's size and the width of its characters, as the file's
	// code is painted
	codeSize, charWidth float32
	y                   float32
	shapes              []shape
}

// wordStyle is how a run of words is painted.
type wordStyle struct {
	size      float32
	weight    draw2d.Weight
	colour    draw2d.Color
	code      bool
	strike    bool
	underline bool
}

// bodySize is the size of a formatted text's words, a little larger than the interface's.
func (layout *formatter) bodySize() float32 { return layout.palette.TextSize + 2 }

// headingSizes are how much larger than the words each level of heading is.
var headingSizes = []float32{1.9, 1.5, 1.25, 1.1, 1, 0.95}

// block lays a block out at the bottom of what was laid out.
func (layout *formatter) block(block markdown.Block) {
	base := wordStyle{size: layout.bodySize(), colour: layout.palette.Ink}
	switch block.Kind {
	case markdown.Heading:
		layout.heading(block)
	case markdown.Item:
		layout.item(block, base)
	case markdown.Quote:
		layout.quote(block, base)
	case markdown.Code:
		layout.code(block)
	case markdown.Rule:
		layout.box(draw2d.Rectangle{Y: layout.y + 8, Width: layout.width, Height: 1}, layout.palette.Line, 0)
		layout.y += 17
	case markdown.Table:
		layout.table(block, base)
	default:
		layout.y = layout.flow(block.Spans, 0, layout.width, layout.y, base)
	}
}

// gapAfter is the room under a block: little between the items of a list, more between blocks.
func (layout *formatter) gapAfter(blocks []markdown.Block, index int) {
	next := markdown.Paragraph
	if index+1 < len(blocks) {
		next = blocks[index+1].Kind
	}
	switch {
	case blocks[index].Kind == markdown.Item && next == markdown.Item:
		layout.y += 4
	case blocks[index].Kind == markdown.Quote && next == markdown.Quote:
		layout.y += 6
	default:
		layout.y += 14
	}
}

// heading lays a heading out, with room above it, and a line under the first two levels.
func (layout *formatter) heading(block markdown.Block) {
	level := min(max(block.Level, 1), len(headingSizes))
	style := wordStyle{size: layout.bodySize() * headingSizes[level-1], weight: draw2d.Bold, colour: layout.palette.Ink}
	if layout.y > 0 {
		layout.y += style.size * 0.6
	}
	layout.y = layout.flow(block.Spans, 0, layout.width, layout.y, style)
	if level <= 2 {
		layout.y += 6
		layout.box(draw2d.Rectangle{Y: layout.y, Width: layout.width, Height: 1}, layout.palette.Line, 0)
		layout.y += 4
	}
}

// item lays a list's item out: its bullet, number or box, and its words beside it.
func (layout *formatter) item(block markdown.Block, style wordStyle) {
	indent := float32(block.Level+1) * 22
	mark := style
	mark.colour = layout.palette.InkSoft
	if block.Mark == "☑" {
		mark.colour = layout.palette.Positive
	}
	markWidth := layout.measure(block.Mark, mark)
	layout.text(block.Mark, indent-8-markWidth, layout.y, layout.lineHeight(style), markWidth, mark)
	layout.y = layout.flow(block.Spans, indent, layout.width, layout.y, style)
}

// quote lays a quote's paragraph out, softer, beside a bar.
func (layout *formatter) quote(block markdown.Block, style wordStyle) {
	top := layout.y
	style.colour = layout.palette.InkSoft
	layout.y = layout.flow(block.Spans, 16, layout.width, layout.y, style)
	layout.box(draw2d.Rectangle{Y: top, Width: 3, Height: layout.y - top}, layout.palette.Line, 1.5)
}

// code lays a block of code out on its background, coloured as its language reads, its lines as
// they are.
func (layout *formatter) code(block markdown.Block) {
	const padding, lineHeight = 12, 20
	lines := codeLines(block)
	height := float32(len(lines))*lineHeight + 2*padding
	layout.box(draw2d.Rectangle{Y: layout.y, Width: layout.width, Height: height}, layout.look.Hover, 6)
	columns := int((layout.width - 2*padding) / max(layout.charWidth, 1))
	for index, line := range lines {
		top := layout.y + padding + float32(index)*lineHeight
		for _, span := range line.Spans {
			span.Text = cutAt(span.Text, columns-span.Column)
			if strings.TrimSpace(span.Text) == "" {
				continue
			}
			style := wordStyle{size: layout.codeSize, weight: draw2d.Mono, colour: layout.look.Syntax.Colour(span.Kind)}
			x := padding + float32(span.Column)*layout.charWidth
			layout.text(span.Text, x, top, lineHeight, float32(len([]rune(span.Text)))*layout.charWidth, style)
		}
	}
	layout.y += height
}

// codePadding is the room on each side of a code span's words, inside its shade
const codePadding = 4

// flow lays words out from left to right between two edges, going on to the next line where they
// do not fit, and tells where the last line ends. A word wider than the room between the edges is
// cut, so nothing reaches past them.
func (layout *formatter) flow(spans []markdown.Span, left, right, top float32, base wordStyle) float32 {
	lineHeight := layout.lineHeight(base)
	x, y := left, top
	for _, span := range spans {
		style := layout.styleOf(span, base)
		shade := noShade
		for _, word := range layout.fitted(layout.words(span, style, right-left), style, right-left) {
			width := layout.measure(word, style)
			if x+layout.wordWidth(word, style) > right && x > left {
				x, y, shade = left, y+lineHeight, noShade
				if word == " " {
					continue
				}
			}
			if style.code && strings.TrimSpace(word) != "" {
				x, shade = layout.codeShade(word, style, x, y, lineHeight, shade)
				width += codePadding // room after it, inside its shade
			}
			layout.text(word, x, y, lineHeight, width, style)
			x += width
		}
	}
	return y + lineHeight
}

// noShade is a code span with no shade drawn yet on the line.
const noShade = -1

// codeShade shades a word of code at x: a shade of its own for the first word of a span on a
// line, the shade before it stretched for the next ones, so code cut at its spaces reads as one
// run, not as a chip a word. It returns where the word is written, and the shade's place.
func (layout *formatter) codeShade(word string, style wordStyle, x, y, lineHeight float32, shade int) (float32, int) {
	trimmed := layout.measure(strings.TrimRight(word, " "), style)
	if shade == noShade {
		layout.box(draw2d.Rectangle{X: x + 1, Y: y + 3, Width: trimmed + 2*codePadding - 2, Height: lineHeight - 6}, layout.look.Hover, 4)
		return x + codePadding, len(layout.shapes) - 1
	}
	x -= codePadding // the word before left room for the shade's end: this word takes it
	area := &layout.shapes[shade].area
	area.Width = x + trimmed + codePadding - 1 - area.X
	return x, shade
}

// words are the pieces a span may go on to the next line between: its words, or a code span
// whole while it fits in the room.
func (layout *formatter) words(span markdown.Span, style wordStyle, room float32) []string {
	if style.code && layout.wordWidth(span.Text, style) <= room {
		return []string{span.Text} // code is cut only when it is longer than a line
	}
	return strings.SplitAfter(span.Text, " ")
}

// wordWidth is how wide a word is drawn, without the space after it, its shade's room included.
func (layout *formatter) wordWidth(word string, style wordStyle) float32 {
	width := layout.measure(strings.TrimRight(word, " "), style)
	if style.code {
		width += 2 * codePadding
	}
	return width
}

// fitted cuts the words wider than the room into pieces that fit, a character at least each.
func (layout *formatter) fitted(words []string, style wordStyle, room float32) []string {
	var pieces []string
	for _, word := range words {
		for layout.wordWidth(word, style) > room {
			characters := []rune(word)
			cut := 1
			for cut < len(characters) && layout.wordWidth(string(characters[:cut+1]), style) <= room {
				cut++
			}
			if cut == len(characters) {
				break // only its space was too wide: it fits once at the end of a line
			}
			pieces = append(pieces, string(characters[:cut]))
			word = string(characters[cut:])
		}
		pieces = append(pieces, word)
	}
	return pieces
}

// styleOf is how a span's words look, from the style of the block around them.
func (layout *formatter) styleOf(span markdown.Span, base wordStyle) wordStyle {
	style := base
	switch {
	case span.Code:
		style.weight, style.size, style.code = draw2d.Mono, base.size-1, true
	case span.Bold:
		style.weight = draw2d.Bold
	case span.Italic:
		style.weight, style.colour = draw2d.Medium, layout.palette.InkSoft
	}
	if span.Link != "" {
		style.colour, style.underline = layout.palette.Primary, true
	}
	style.strike = span.Strike
	return style
}

// lineHeight is how tall a line of words in a style is.
func (layout *formatter) lineHeight(style wordStyle) float32 { return style.size * 1.6 }

// measure is how wide words are in a style.
func (layout *formatter) measure(text string, style wordStyle) float32 {
	return layout.context.Measurer().MeasureText(text, style.size, style.weight, 1).Width
}

// text keeps a run of words.
func (layout *formatter) text(words string, x, top, height, width float32, style wordStyle) {
	if words == "" {
		return
	}
	layout.shapes = append(layout.shapes, shape{
		area: draw2d.Rectangle{X: x, Y: top, Width: width, Height: height}, text: words, size: style.size,
		weight: style.weight, colour: style.colour, strike: style.strike, underline: style.underline,
	})
}

// box keeps a filled box.
func (layout *formatter) box(area draw2d.Rectangle, colour draw2d.Color, radius float32) {
	layout.shapes = append(layout.shapes, shape{area: area, colour: colour, radius: radius})
}

// cutAt is a text cut to some characters, ending in "…" when it was longer; a line of code past
// its block's edge is cut rather than run over the page.
func cutAt(text string, characters int) string {
	runes := []rune(text)
	if len(runes) <= characters {
		return text
	}
	if characters <= 0 {
		return ""
	}
	return string(runes[:characters-1]) + "…"
}
