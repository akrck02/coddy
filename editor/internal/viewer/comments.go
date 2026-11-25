package viewer

import "github.com/akrck02/go-renderer/draw2d"

// commentGap is how many characters of room are left between a line's code and its comment.
const commentGap = 3

// TODO(akrck02): a comment longer than the room left is cut at the view's edge; a card on hover
// could show it whole, with its replies.

// paintComment paints the comments on a line of the file as it is now (number from 1, 0 for a
// line removed): a mark in the gutter, at gutterX, and their words after the line's code, quiet.
// The words were made when the comments came, so painting allocates nothing.
func (view *Viewer) paintComment(painter painting, number, columns int, gutterX, baseline float32) {
	words, found := view.Comments[number]
	if !found || number == 0 {
		return
	}
	palette := painter.context.Theme
	top := baseline - view.codeAscent
	painter.drawing.Icon("message-square", gutterX, top+(view.codeAscent+view.codeDescent-12)/2, 12, palette.Primary)
	x := painter.codeArea.X + float32(columns+commentGap)*view.charWidth - view.List.OffsetX
	painter.drawing.PushClip(painter.codeArea)
	painter.drawing.Text(words, x, baseline, palette.SmallTextSize, draw2d.Regular, palette.InkQuiet)
	painter.drawing.PopClip()
}
