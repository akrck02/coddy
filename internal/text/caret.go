package text

import (
	"strings"
	"unicode"
)

// Motion is a way the caret moves.
type Motion int

const (
	Left Motion = iota
	Right
	Up
	Down
	WordLeft
	WordRight
	LineStart
	LineEnd
	TextStart
	TextEnd
)

// Caret is where typing goes.
func (buffer *Buffer) Caret() Position { return buffer.caret }

// Selection is the text selected, from its first to its last position, and whether there is one.
func (buffer *Buffer) Selection() (start, end Position, selected bool) {
	if buffer.caret == buffer.anchor {
		return buffer.caret, buffer.caret, false
	}
	if buffer.caret.Before(buffer.anchor) {
		return buffer.caret, buffer.anchor, true
	}
	return buffer.anchor, buffer.caret, true
}

// SelectedText is the text selected, "" when nothing is.
func (buffer *Buffer) SelectedText() string {
	start, end, selected := buffer.Selection()
	if !selected {
		return ""
	}
	return buffer.between(start, end)
}

// between is the text from one position to a later one, with "\n" between lines.
func (buffer *Buffer) between(start, end Position) string {
	if start.Line == end.Line {
		return string(buffer.lines[start.Line][start.Column:end.Column])
	}
	parts := []string{string(buffer.lines[start.Line][start.Column:])}
	for line := start.Line + 1; line < end.Line; line++ {
		parts = append(parts, string(buffer.lines[line]))
	}
	parts = append(parts, string(buffer.lines[end.Line][:end.Column]))
	return strings.Join(parts, "\n")
}

// Place puts the caret at a position, kept inside the text; extending keeps the anchor where it is,
// so the selection grows or shrinks.
func (buffer *Buffer) Place(position Position, extending bool) {
	buffer.caret = buffer.clamp(position)
	if !extending {
		buffer.anchor = buffer.caret
	}
	buffer.goalColumn = buffer.caret.Column
}

// SelectAll selects the whole text.
func (buffer *Buffer) SelectAll() {
	buffer.anchor = Position{}
	buffer.caret = buffer.End()
	buffer.goalColumn = buffer.caret.Column
}

// SelectWord selects the word, the run of spaces or the run of symbols at a position.
func (buffer *Buffer) SelectWord(position Position) {
	position = buffer.clamp(position)
	line := buffer.lines[position.Line]
	if len(line) == 0 {
		buffer.Place(position, false)
		return
	}
	at := min(position.Column, len(line)-1)
	class := classOf(line[at])
	start, end := at, at+1
	for start > 0 && classOf(line[start-1]) == class {
		start--
	}
	for end < len(line) && classOf(line[end]) == class {
		end++
	}
	buffer.anchor = Position{Line: position.Line, Column: start}
	buffer.caret = Position{Line: position.Line, Column: end}
	buffer.goalColumn = end
}

// Move moves the caret; extending keeps the anchor, so the selection grows. Moving left or right
// without extending first closes a selection at its side, as editors do.
func (buffer *Buffer) Move(motion Motion, extending bool) {
	start, end, selected := buffer.Selection()
	if selected && !extending && (motion == Left || motion == Right) {
		target := start
		if motion == Right {
			target = end
		}
		buffer.Place(target, false)
		return
	}
	target := buffer.target(motion)
	goal := buffer.goalColumn
	buffer.Place(target, extending)
	if motion == Up || motion == Down {
		buffer.goalColumn = goal
	}
}

// MoveLines moves the caret a number of lines down (or up, when negative), as a page key does.
func (buffer *Buffer) MoveLines(count int, extending bool) {
	goal := buffer.goalColumn
	target := Position{Line: buffer.caret.Line + count, Column: goal}
	buffer.Place(target, extending)
	buffer.goalColumn = goal
}

// target is where a motion takes the caret from where it is.
func (buffer *Buffer) target(motion Motion) Position {
	caret := buffer.caret
	switch motion {
	case Left:
		return buffer.stepLeft(caret)
	case Right:
		return buffer.stepRight(caret)
	case Up:
		return buffer.verticalTarget(-1)
	case Down:
		return buffer.verticalTarget(1)
	case WordLeft:
		return buffer.wordLeft(caret)
	case WordRight:
		return buffer.wordRight(caret)
	case LineStart:
		return Position{Line: caret.Line}
	case LineEnd:
		return Position{Line: caret.Line, Column: len(buffer.lines[caret.Line])}
	case TextStart:
		return Position{}
	}
	return buffer.End()
}

// verticalTarget is the position a line up or down, at the goal column; past the first or last
// line it is the start or end of the text.
func (buffer *Buffer) verticalTarget(step int) Position {
	line := buffer.caret.Line + step
	if line < 0 {
		return Position{}
	}
	if line >= len(buffer.lines) {
		return buffer.End()
	}
	return buffer.clamp(Position{Line: line, Column: buffer.goalColumn})
}

// stepLeft is one character to the left, or the end of the line before.
func (buffer *Buffer) stepLeft(position Position) Position {
	if position.Column > 0 {
		return Position{Line: position.Line, Column: position.Column - 1}
	}
	if position.Line == 0 {
		return position
	}
	return Position{Line: position.Line - 1, Column: len(buffer.lines[position.Line-1])}
}

// stepRight is one character to the right, or the start of the next line.
func (buffer *Buffer) stepRight(position Position) Position {
	if position.Column < len(buffer.lines[position.Line]) {
		return Position{Line: position.Line, Column: position.Column + 1}
	}
	if position.Line == len(buffer.lines)-1 {
		return position
	}
	return Position{Line: position.Line + 1}
}

// wordLeft is the start of the word before: past the spaces, then past the characters of one kind.
// At the start of a line it is the end of the line before.
func (buffer *Buffer) wordLeft(position Position) Position {
	if position.Column == 0 {
		return buffer.stepLeft(position)
	}
	line := buffer.lines[position.Line]
	column := position.Column
	for column > 0 && classOf(line[column-1]) == spaceClass {
		column--
	}
	if column > 0 {
		class := classOf(line[column-1])
		for column > 0 && classOf(line[column-1]) == class {
			column--
		}
	}
	return Position{Line: position.Line, Column: column}
}

// wordRight is the end of the word after: past the spaces, then past the characters of one kind.
// At the end of a line it is the start of the next.
func (buffer *Buffer) wordRight(position Position) Position {
	line := buffer.lines[position.Line]
	if position.Column == len(line) {
		return buffer.stepRight(position)
	}
	column := position.Column
	for column < len(line) && classOf(line[column]) == spaceClass {
		column++
	}
	if column < len(line) {
		class := classOf(line[column])
		for column < len(line) && classOf(line[column]) == class {
			column++
		}
	}
	return Position{Line: position.Line, Column: column}
}

// The kinds of character a word is made of: moving by words stops where the kind changes.
const (
	spaceClass = iota
	wordClass
	symbolClass
)

func classOf(character rune) int {
	switch {
	case unicode.IsSpace(character):
		return spaceClass
	case character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character):
		return wordClass
	}
	return symbolClass
}
