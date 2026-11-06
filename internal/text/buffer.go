// Package text is a file being edited: its lines, a caret and a selection, the edits made to it and
// how to undo them. It knows nothing of the window, the screen or the disk.
package text

import "strings"

// Position is a place in a text: a line and a column, both counted from zero. The column counts
// characters, not bytes, so "ñ" is one step.
type Position struct {
	Line, Column int
}

// Before tells whether a position comes earlier in the text than another.
func (position Position) Before(other Position) bool {
	return position.Line < other.Line || (position.Line == other.Line && position.Column < other.Column)
}

// Buffer is the text of a file being edited, with its caret and selection and what can be undone.
// The selection runs from the anchor to the caret; when they are equal nothing is selected.
type Buffer struct {
	lines         [][]rune
	lineEnding    string
	caret, anchor Position
	// goalColumn is the column moving up and down aims for, kept across short lines.
	goalColumn int
	history    history
}

// New starts editing a text. Its line endings are kept: a file written with "\r\n" is saved with
// them.
func New(content string) *Buffer {
	buffer := &Buffer{lineEnding: "\n"}
	if strings.Contains(content, "\r\n") {
		buffer.lineEnding = "\r\n"
	}
	buffer.lines = splitLines(content)
	return buffer
}

// splitLines cuts a text into lines of characters. A text ending in a line break ends with an
// empty line, so writing the lines back gives the same text.
func splitLines(content string) [][]rune {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	parts := strings.Split(content, "\n")
	lines := make([][]rune, len(parts))
	for index, part := range parts {
		lines[index] = []rune(part)
	}
	return lines
}

// Text is the whole text, with the line endings it was read with.
func (buffer *Buffer) Text() string {
	parts := make([]string, len(buffer.lines))
	for index, line := range buffer.lines {
		parts[index] = string(line)
	}
	return strings.Join(parts, buffer.lineEnding)
}

// LineCount is how many lines the text has; an empty text has one, empty.
func (buffer *Buffer) LineCount() int { return len(buffer.lines) }

// Line is the text of a line, or "" past the last one.
func (buffer *Buffer) Line(index int) string {
	if index < 0 || index >= len(buffer.lines) {
		return ""
	}
	return string(buffer.lines[index])
}

// LineLength is how many characters a line has.
func (buffer *Buffer) LineLength(index int) int {
	if index < 0 || index >= len(buffer.lines) {
		return 0
	}
	return len(buffer.lines[index])
}

// End is the position after the last character of the text.
func (buffer *Buffer) End() Position {
	last := len(buffer.lines) - 1
	return Position{Line: last, Column: len(buffer.lines[last])}
}

// clamp brings a position inside the text: to its last line, and to the end of its line.
func (buffer *Buffer) clamp(position Position) Position {
	position.Line = min(max(position.Line, 0), len(buffer.lines)-1)
	position.Column = min(max(position.Column, 0), len(buffer.lines[position.Line]))
	return position
}
