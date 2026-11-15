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
	lines [][]rune
	// breaks are the line endings, breaks[i] after lines[i], each as read: "\n" or "\r\n".
	// lineEnding is the one most lines end with, which a new line break gets.
	breaks        []string
	lineEnding    string
	caret, anchor Position
	// goalColumn is the column moving up and down aims for, kept across short lines.
	goalColumn int
	history    history
}

// New starts editing a text. Its line endings are kept, each as it was: a file written with
// "\r\n", or with both, is saved as it was read.
func New(content string) *Buffer {
	buffer := &Buffer{lineEnding: "\n"}
	buffer.lines, buffer.breaks = splitLines(content)
	if crlf := strings.Count(content, "\r\n"); crlf > 0 && crlf*2 >= len(buffer.breaks) {
		buffer.lineEnding = "\r\n"
	}
	return buffer
}

// splitLines cuts a text into lines of characters and the line endings between them. A text
// ending in a line break ends with an empty line, so writing the lines back gives the same text.
func splitLines(content string) ([][]rune, []string) {
	parts := strings.Split(content, "\n")
	lines := make([][]rune, len(parts))
	breaks := make([]string, len(parts)-1)
	for index, part := range parts {
		if index < len(breaks) {
			breaks[index] = "\n"
			if strings.HasSuffix(part, "\r") {
				part, breaks[index] = part[:len(part)-1], "\r\n"
			}
		}
		lines[index] = []rune(part)
	}
	return lines, breaks
}

// Text is the whole text, with the line endings it was read with.
func (buffer *Buffer) Text() string {
	var builder strings.Builder
	for index, line := range buffer.lines {
		builder.WriteString(string(line))
		if index < len(buffer.breaks) {
			builder.WriteString(buffer.breaks[index])
		}
	}
	return builder.String()
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
