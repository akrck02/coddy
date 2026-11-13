package viewer

import (
	"math"
	"path/filepath"
	"strings"

	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/markdown"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// IsMarkdown tells whether the file open is written in Markdown, and so can be shown formatted.
func (view *Viewer) IsMarkdown() bool {
	switch strings.ToLower(filepath.Ext(view.Reference.Path)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}

// Formatted tells whether the file is shown formatted, as its Markdown reads, instead of as text.
func (view *Viewer) Formatted() bool {
	return view.formatted && view.IsMarkdown() && view.Document != nil && !view.Document.Binary
}

// ToggleFormatted shows a Markdown file formatted, or as its text.
func (view *Viewer) ToggleFormatted() {
	view.formatted = !view.formatted
	view.List.Offset, view.List.OffsetX = 0, 0
}

// buildFormatSwitch is the header's eye: on, the file shows formatted; off, as its text.
func (view *Viewer) buildFormatSwitch(context *ui.Context) {
	tip := "Show it formatted"
	if view.Formatted() {
		tip = "Show its text, to edit it"
	}
	if theme.ToolButton(context, tip, "eye", view.Formatted()) {
		view.ToggleFormatted()
	}
}

// formattedMargin is the room around a formatted text, and formattedWidth the widest it gets, so
// its lines stay easy to read.
const (
	formattedMargin = 32
	formattedWidth  = 820
)

// shape is a piece of a formatted text: a run of words, or a filled box (code's background, a
// quote's bar, a rule), placed from the text's top-left.
type shape struct {
	area      draw2d.Rectangle
	text      string
	size      float32
	weight    draw2d.Weight
	colour    draw2d.Color
	radius    float32
	strike    bool
	underline bool
}

// formatting is a text laid out formatted, kept while its text, width and colours stay the same.
type formatting struct {
	source    string
	width     float32
	look      draw2d.Color
	charWidth float32
	shapes    []shape
	height    float32
}

// source is the text of the file open: as it is edited, or as it was read.
func (view *Viewer) source() string {
	if view.Edit != nil {
		lines := make([]string, view.Edit.LineCount())
		for index := range lines {
			lines[index] = view.Edit.Line(index)
		}
		return strings.Join(lines, "\n")
	}
	if view.Document.Editable {
		return view.Document.Text
	}
	lines := make([]string, len(view.Document.NewLines))
	for index, line := range view.Document.NewLines {
		lines[index] = line.Text()
	}
	return strings.Join(lines, "\n")
}

// layOutFormatted lays the text out for the width it has, unless it was already.
func (view *Viewer) layOutFormatted(context *ui.Context, window Window, area draw2d.Rectangle) *formatting {
	width := min(area.Width-2*formattedMargin-pane.ScrollBarWidth, formattedWidth)
	source, look := view.source(), window.Look()
	if cached := view.formatting; cached != nil && cached.source == source && cached.width == width && cached.look == look.Editor &&
		cached.charWidth == view.charWidth {
		return cached
	}
	layout := &formatter{context: context, palette: context.Theme, look: look, width: max(width, 120),
		codeSize: view.codeSize, charWidth: view.charWidth}
	blocks := markdown.Parse(source)
	for index, block := range blocks {
		layout.block(block)
		layout.gapAfter(blocks, index)
	}
	view.formatting = &formatting{source: source, width: width, look: look.Editor, charWidth: view.charWidth, shapes: layout.shapes, height: layout.y}
	return view.formatting
}

// buildFormatted shows the text formatted, scrolling as the code does.
func (view *Viewer) buildFormatted(context *ui.Context, window Window) {
	view.List.RowHeight = view.rowHeight
	if formatted := view.formatting; formatted != nil {
		view.List.Count = int(math.Ceil(float64((formatted.height + 2*formattedMargin) / view.rowHeight)))
	}
	view.List.Width = 0
	if !window.ModalOpen() {
		keys := window.Keys()
		view.List.Follow(context, keys, context.Theme.ScrollStep)
		if x, y := context.Pointer(); keys.Clicked() && view.List.Area.Contains(x, y) {
			window.FocusViewer()
		}
	}
	context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		view.List.Place(area)
		formatted := view.layOutFormatted(context, window, area)
		view.List.Count = int(math.Ceil(float64((formatted.height + 2*formattedMargin) / view.rowHeight)))
		view.paintFormatted(drawing, window.Look(), formatted, area)
	})
}

// paintFormatted paints the shapes in sight.
func (view *Viewer) paintFormatted(drawing *draw2d.List, look theme.Look, formatted *formatting, area draw2d.Rectangle) {
	left := area.X + max((area.Width-pane.ScrollBarWidth-formatted.width)/2, formattedMargin)
	top := area.Y + formattedMargin - view.List.Offset
	drawing.PushClip(area)
	for _, piece := range formatted.shapes {
		placed := piece.area
		placed.X, placed.Y = placed.X+left, placed.Y+top
		if placed.Bottom() < area.Y || placed.Y > area.Bottom() {
			continue
		}
		paintShape(drawing, piece, placed)
	}
	view.List.PaintBar(drawing, look.ScrollThumb)
	drawing.PopClip()
}

// paintShape paints a run of words, with its line under or through it, or a box.
func paintShape(drawing *draw2d.List, piece shape, placed draw2d.Rectangle) {
	if piece.text == "" {
		drawing.Rectangle(placed, piece.radius, piece.colour, draw2d.Color{}, 0)
		return
	}
	baseline := placed.Y + placed.Height/2 + piece.size*0.32
	drawing.Text(piece.text, placed.X, baseline, piece.size, piece.weight, piece.colour)
	width := strings.TrimRight(piece.text, " ")
	line := draw2d.Rectangle{X: placed.X, Width: placed.Width * float32(len(width)) / float32(max(len(piece.text), 1)), Height: 1}
	switch {
	case piece.underline:
		line.Y = baseline + 2
		drawing.Rectangle(line, 0, piece.colour.WithAlpha(0.5), draw2d.Color{}, 0)
	case piece.strike:
		line.Y = baseline - piece.size*0.3
		drawing.Rectangle(line, 0, piece.colour, draw2d.Color{}, 0)
	}
}

// codeLines are a block of code's lines, coloured as its language reads when it is known.
func codeLines(block markdown.Block) []highlight.Line {
	text := strings.Join(block.Lines, "\n")
	if block.Language == "" {
		return highlight.Lines("block.txt", text)
	}
	return highlight.Lines("block."+block.Language, text)
}

// FormattedWords are the words the formatted text paints, in order, for tests.
func (view *Viewer) FormattedWords() string {
	if view.formatting == nil {
		return ""
	}
	var words strings.Builder
	for _, piece := range view.formatting.shapes {
		words.WriteString(piece.text)
	}
	return words.String()
}
