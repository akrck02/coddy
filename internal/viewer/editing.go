package viewer

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/text"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
)

// KeyHelp is a key the view answers itself while a file is edited, as the help lists it.
type KeyHelp struct {
	Keys        []string
	Description string
}

// EditingKeys are the keys the view answers while a file is edited; the window runs the rest.
var EditingKeys = []KeyHelp{
	{[]string{"←", "→", "↑", "↓"}, "Move the caret; with Shift, select"},
	{[]string{"⌥←", "⌥→"}, "Move by a word"},
	{[]string{"⌘←", "⌘→", "Home", "End"}, "Go to the start / end of the line"},
	{[]string{"⌘↑", "⌘↓"}, "Go to the start / end of the file"},
	{[]string{"PgUp", "PgDn"}, "Move the caret a page"},
	{[]string{"⌥Backspace"}, "Delete the word before the caret"},
	{[]string{"Double click"}, "Select a word"},
}

// colourPause is how long typing must stop before the file is coloured again.
const colourPause = 300 * time.Millisecond

// startEditing makes the buffer of a document that can be edited, or drops it.
func (view *Viewer) startEditing() {
	document := view.Document
	if !document.Editable {
		view.Edit = nil
		return
	}
	view.Edit = text.New(document.Text)
	view.editedFrom = document.Stamp
	view.coloured, view.colouredVersion = document.NewLines, view.Edit.Version()
	view.indent = "    "
	if strings.HasPrefix(document.Text, "\t") || strings.Contains(document.Text, "\n\t") {
		view.indent = "\t"
	}
}

// keepEditing keeps the buffer when the file it edits is read again: its edits not saved, or its
// caret when the text read is the same. Otherwise the buffer starts again from what was read.
func (view *Viewer) keepEditing() {
	buffer, document := view.Edit, view.Document
	switch {
	case buffer != nil && buffer.Modified():
		// the edits stay, still measured against the file as they started from it
	case buffer == nil || !document.Editable:
		view.startEditing()
	case buffer.Text() == document.Text:
		view.coloured, view.colouredVersion = document.NewLines, buffer.Version()
	default:
		caret := buffer.Caret()
		view.startEditing()
		view.Edit.Place(caret, false)
	}
}

// Unsaved tells whether the file open has edits not saved.
func (view *Viewer) Unsaved() bool { return view.showing && view.Edit != nil && view.Edit.Modified() }

// EditedFrom is what the file was when the edits started from it, for saving over it safely.
func (view *Viewer) EditedFrom() workspace.Stamp { return view.editedFrom }

// Saved records that the buffer was written: what it wrote, and the file's stamp now.
func (view *Viewer) Saved(content string, stamp workspace.Stamp) {
	view.Edit.MarkSaved()
	view.editedFrom = stamp
	if view.Document != nil {
		view.Document.Text, view.Document.Stamp = content, stamp
	}
}

// Discard drops the edits not saved, going back to the file as read.
func (view *Viewer) Discard() {
	if view.Document != nil {
		view.startEditing()
	}
}

// Editable tells whether the file shown can take the keyboard to be edited: whole, and read from
// disk as text.
func (view *Viewer) Editable() bool {
	return view.showing && view.Edit != nil && view.Mode == FileView && !view.Formatted()
}

// fileLine is a line of the file view: of the buffer when the file is edited, coloured as it was
// last coloured while its text is the same, plain otherwise.
func (view *Viewer) fileLine(index int) highlight.Line {
	if view.Edit == nil {
		return view.lines()[index]
	}
	if view.colouredVersion == view.Edit.Version() && index < len(view.coloured) {
		return view.coloured[index]
	}
	plain := highlight.Uncoloured(view.Edit.Line(index))
	if index < len(view.coloured) && view.coloured[index].Text() == plain.Text() {
		return view.coloured[index]
	}
	return plain
}

// fileLineCount is how many lines the file view shows.
func (view *Viewer) fileLineCount() int {
	if view.Edit != nil {
		return view.Edit.LineCount()
	}
	return len(view.lines())
}

// Apply runs a change on the buffer, as a command of the window does (undo, paste…), then keeps
// the caret in sight and colours the file again once typing stops.
func (view *Viewer) Apply(window Window, change func(buffer *text.Buffer)) {
	if view.Edit == nil {
		return
	}
	version := view.Edit.Version()
	change(view.Edit)
	view.revealCaret()
	if view.Edit.Version() != version {
		view.colourLater(window)
	}
}

// Type answers the keys of a frame while the file is edited: what was typed, the keys that delete
// and those that move the caret.
func (view *Viewer) Type(window Window, keys pane.Keys) {
	view.Apply(window, func(buffer *text.Buffer) {
		if !view.moveByKeys(buffer, keys) {
			view.changeByKeys(buffer, keys)
		}
	})
}

// moveByKeys moves the caret by the arrows, Home, End and the page keys, selecting with Shift.
func (view *Viewer) moveByKeys(buffer *text.Buffer, keys pane.Keys) bool {
	extending := keys.Shift()
	switch {
	case keys.Typed(input.KeyPageUp):
		buffer.MoveLines(-view.List.PageRows(), extending)
	case keys.Typed(input.KeyPageDown):
		buffer.MoveLines(view.List.PageRows(), extending)
	default:
		for _, key := range []input.Key{input.KeyLeft, input.KeyRight, input.KeyUp, input.KeyDown, input.KeyHome, input.KeyEnd} {
			if keys.Typed(key) {
				buffer.Move(motionOf(key, keys.Command(), keys.Alt()), extending)
				return true
			}
		}
		return false
	}
	return true
}

// motionOf is where a key moves the caret: Alt goes by words, Command to the ends.
func motionOf(key input.Key, command, alt bool) text.Motion {
	switch key {
	case input.KeyLeft:
		return pick(alt, text.WordLeft, pick(command, text.LineStart, text.Left))
	case input.KeyRight:
		return pick(alt, text.WordRight, pick(command, text.LineEnd, text.Right))
	case input.KeyUp:
		return pick(command, text.TextStart, text.Up)
	case input.KeyDown:
		return pick(command, text.TextEnd, text.Down)
	case input.KeyHome:
		return text.LineStart
	}
	return text.LineEnd
}

func pick(condition bool, chosen, otherwise text.Motion) text.Motion {
	if condition {
		return chosen
	}
	return otherwise
}

// changeByKeys types what the keyboard typed, and answers Backspace, Delete, Enter and Tab. Enter
// keeps the line's indentation.
func (view *Viewer) changeByKeys(buffer *text.Buffer, keys pane.Keys) {
	switch {
	case keys.Typed(input.KeyBackspace) && keys.Alt():
		buffer.BackspaceWord()
	case keys.Typed(input.KeyBackspace):
		buffer.Backspace()
	case keys.Typed(input.KeyDelete):
		buffer.Delete()
	case keys.Typed(input.KeyEnter):
		buffer.Insert("\n" + indentBefore(buffer))
	case keys.Typed(input.KeyTab) && !keys.Command():
		buffer.Insert(view.indent)
	}
	if keys.Command() {
		return // a key with Command types nothing, even where the system gives its character
	}
	for _, character := range keys.TypedText() {
		if unicode.IsPrint(character) {
			buffer.Insert(string(character))
		}
	}
}

// indentBefore is the indentation of the caret's line, as far as the caret: Enter at the start of
// a line or inside its indent keeps only what is before the caret.
func indentBefore(buffer *text.Buffer) string {
	caret := buffer.Caret()
	before := string([]rune(buffer.Line(caret.Line))[:caret.Column])
	return before[:len(before)-len(strings.TrimLeft(before, " \t"))]
}

// colourLater colours the buffer again in the background, once typing has stopped for a moment;
// a newer edit replaces the job waiting.
func (view *Viewer) colourLater(window Window) {
	buffer, content, version, path := view.Edit, view.Edit.Text(), view.Edit.Version(), view.Reference.Path
	pause := view.ColourPause
	window.Run("colour the file edited", func(stop context.Context) func() {
		if pause > 0 {
			select {
			case <-stop.Done():
				return nil
			case <-time.After(pause):
			}
		}
		lines := highlight.Lines(path, content)
		return func() {
			if view.Edit == buffer && buffer.Version() == version {
				view.coloured, view.colouredVersion = lines, version
			}
		}
	})
}

// revealCaret scrolls as little as possible to show the caret, a few characters in from the side.
func (view *Viewer) revealCaret() {
	caret := view.Edit.Caret()
	view.List.Reveal(caret.Line)
	if view.charWidth <= 0 || view.codeWidth <= 0 {
		return
	}
	x := float32(highlight.Column(view.Edit.Line(caret.Line), caret.Column)) * view.charWidth
	margin := view.charWidth * sidewaysCharacters
	switch {
	case x < view.List.OffsetX+margin:
		view.List.OffsetX = max(x-margin, 0)
	case x > view.List.OffsetX+view.codeWidth-margin:
		view.List.OffsetX = x - view.codeWidth + margin
		view.List.Width = max(view.List.Width, x+margin)
	}
}

// positionAt is the place in the buffer under a point of the code.
func (view *Viewer) positionAt(x, y float32) text.Position {
	line := int((y - view.List.Area.Y + view.List.Offset) / view.List.RowHeight)
	line = max(min(line, view.Edit.LineCount()-1), 0)
	place := (x - view.codeLeft + view.List.OffsetX) / max(view.charWidth, 1)
	return text.Position{Line: line, Column: highlight.CharacterAt(view.Edit.Line(line), place)}
}

// followPointer places the caret where the code is clicked, selects a word on a double click,
// and selects while the pointer drags.
func (view *Viewer) followPointer(keys pane.Keys, x, y float32, clicked bool) {
	switch {
	case clicked && x >= view.codeLeft:
		position := view.positionAt(x, y)
		if keys.Clicks() == 2 {
			view.Edit.SelectWord(position)
		} else {
			view.Edit.Place(position, keys.Shift())
		}
		view.selecting = true
	case view.selecting && keys.Holding():
		view.Edit.Place(view.positionAt(x, y), true)
		view.revealCaret()
	default:
		view.selecting = false
	}
}

// paintSelection paints the part of a line the selection covers, and past its end when the
// selection goes on to the next line.
func (view *Viewer) paintSelection(painter painting, index int, top float32) {
	start, end, selected := view.Edit.Selection()
	if !selected || index < start.Line || index > end.Line {
		return
	}
	line := view.Edit.Line(index)
	from, to := 0, highlight.Column(line, len([]rune(line)))
	if index == start.Line {
		from = highlight.Column(line, start.Column)
	}
	extra := float32(0)
	if index == end.Line {
		to = highlight.Column(line, end.Column)
	} else {
		extra = view.charWidth / 2 // the line break selected
	}
	x := painter.codeArea.X + float32(from)*view.charWidth - view.List.OffsetX
	area := draw2d.Rectangle{X: x, Y: top, Width: float32(to-from)*view.charWidth + extra, Height: view.rowHeight}
	painter.drawing.PushClip(painter.codeArea)
	painter.drawing.Rectangle(area, 0, painter.context.Theme.Selection, draw2d.Color{}, 0)
	painter.drawing.PopClip()
}

// paintCaret paints the caret on its line, as a thin bar before the character it stands at.
func (view *Viewer) paintCaret(painter painting, index int, top float32) {
	caret := view.Edit.Caret()
	if caret.Line != index {
		return
	}
	column := highlight.Column(view.Edit.Line(index), caret.Column)
	x := painter.codeArea.X + float32(column)*view.charWidth - view.List.OffsetX
	if x < painter.codeArea.X-1 || x > painter.codeArea.Right() {
		return
	}
	bar := draw2d.Rectangle{X: x - 1, Y: top + 2, Width: 2, Height: view.rowHeight - 4}
	painter.drawing.Rectangle(bar, 0, painter.context.Theme.Primary, draw2d.Color{}, 0)
}
