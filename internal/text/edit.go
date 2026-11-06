package text

import (
	"strings"
	"unicode/utf8"
)

// Insert types a text at the caret, over the selection if there is one. Typing a word letter by
// letter is undone in one go. It tells whether the text changed.
func (buffer *Buffer) Insert(text string) bool {
	kind := typingEdit
	if _, _, selected := buffer.Selection(); selected || strings.Contains(text, "\n") || utf8.RuneCountInString(text) != 1 {
		kind = otherEdit
	}
	return buffer.replaceSelection(text, kind)
}

// Paste puts a text at the caret, over the selection; it is undone on its own.
func (buffer *Buffer) Paste(text string) bool {
	return buffer.replaceSelection(text, otherEdit)
}

// Cut takes the selection out and returns it, "" when nothing is selected.
func (buffer *Buffer) Cut() string {
	selected := buffer.SelectedText()
	if selected != "" {
		buffer.replaceSelection("", otherEdit)
	}
	return selected
}

// Backspace deletes the selection, or the character before the caret.
func (buffer *Buffer) Backspace() bool {
	if _, _, selected := buffer.Selection(); selected {
		return buffer.replaceSelection("", otherEdit)
	}
	return buffer.change(buffer.stepLeft(buffer.caret), buffer.caret, "", deletingEdit)
}

// BackspaceWord deletes the selection, or back to the start of the word before the caret.
func (buffer *Buffer) BackspaceWord() bool {
	if _, _, selected := buffer.Selection(); selected {
		return buffer.replaceSelection("", otherEdit)
	}
	return buffer.change(buffer.wordLeft(buffer.caret), buffer.caret, "", otherEdit)
}

// Delete deletes the selection, or the character after the caret.
func (buffer *Buffer) Delete() bool {
	if _, _, selected := buffer.Selection(); selected {
		return buffer.replaceSelection("", otherEdit)
	}
	return buffer.change(buffer.caret, buffer.stepRight(buffer.caret), "", deletingEdit)
}

// replaceSelection puts a text where the selection is, or at the caret.
func (buffer *Buffer) replaceSelection(text string, kind editKind) bool {
	start, end, _ := buffer.Selection()
	return buffer.change(start, end, text, kind)
}

// change replaces the text between two positions, leaves the caret after what was put in, and
// keeps the edit to undo it. It tells whether anything changed.
func (buffer *Buffer) change(start, end Position, inserted string, kind editKind) bool {
	inserted = strings.ReplaceAll(inserted, "\r\n", "\n")
	removed := buffer.between(start, end)
	if removed == "" && inserted == "" {
		return false
	}
	record := edit{start: start, removed: removed, inserted: inserted, kind: kind,
		caretBefore: buffer.caret, anchorBefore: buffer.anchor}
	after := buffer.replace(start, end, inserted)
	buffer.Place(after, false)
	record.caretAfter = after
	buffer.history.push(record)
	return true
}

// replace swaps the text between two positions for another and returns where the new text ends.
// It is the only place the lines change.
func (buffer *Buffer) replace(start, end Position, text string) Position {
	before := buffer.lines[start.Line][:start.Column]
	after := buffer.lines[end.Line][end.Column:]
	pieces := splitLines(text)
	replacement := make([][]rune, len(pieces))
	for index, piece := range pieces {
		replacement[index] = append([]rune(nil), piece...)
	}
	last := len(replacement) - 1
	endColumn := len(replacement[last])
	if last == 0 {
		endColumn += len(before)
	}
	replacement[0] = append(append([]rune(nil), before...), replacement[0]...)
	replacement[last] = append(replacement[last], after...)
	lines := make([][]rune, 0, len(buffer.lines)-(end.Line-start.Line)+last)
	lines = append(lines, buffer.lines[:start.Line]...)
	lines = append(lines, replacement...)
	lines = append(lines, buffer.lines[end.Line+1:]...)
	buffer.lines = lines
	return Position{Line: start.Line + last, Column: endColumn}
}

// endOf is where a text put in at a position ends.
func endOf(start Position, text string) Position {
	breaks := strings.Count(text, "\n")
	if breaks == 0 {
		return Position{Line: start.Line, Column: start.Column + utf8.RuneCountInString(text)}
	}
	last := text[strings.LastIndex(text, "\n")+1:]
	return Position{Line: start.Line + breaks, Column: utf8.RuneCountInString(last)}
}
