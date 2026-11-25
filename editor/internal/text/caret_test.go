package text

import "testing"

// at is a position, short to write in a table.
func at(line, column int) Position { return Position{Line: line, Column: column} }

func TestEachMotionTakesTheCaretWhereItShould(test *testing.T) {
	const content = "first line\n  second(one)\n\nlast"
	cases := []struct {
		name   string
		from   Position
		motion Motion
		to     Position
	}{
		{"left inside a line", at(0, 3), Left, at(0, 2)},
		{"left at a line's start goes to the end of the one before", at(1, 0), Left, at(0, 10)},
		{"left at the start stays", at(0, 0), Left, at(0, 0)},
		{"right at a line's end goes to the next", at(0, 10), Right, at(1, 0)},
		{"right at the end stays", at(3, 4), Right, at(3, 4)},
		{"up keeps the column where it can", at(1, 5), Up, at(0, 5)},
		{"down to a shorter line stops at its end", at(1, 8), Down, at(2, 0)},
		{"up from the first line goes to the start", at(0, 4), Up, at(0, 0)},
		{"down from the last line goes to the end", at(3, 1), Down, at(3, 4)},
		{"a word left skips the spaces, then the word", at(0, 6), WordLeft, at(0, 0)},
		{"a word right stops where the kind changes", at(1, 2), WordRight, at(1, 8)},
		{"a word right over a symbol", at(1, 8), WordRight, at(1, 9)},
		{"a word left at a line's start goes up", at(1, 0), WordLeft, at(0, 10)},
		{"the line's start", at(1, 5), LineStart, at(1, 0)},
		{"the line's end", at(1, 5), LineEnd, at(1, 13)},
		{"the text's start", at(2, 0), TextStart, at(0, 0)},
		{"the text's end", at(0, 2), TextEnd, at(3, 4)},
	}
	for _, kind := range cases {
		buffer := New(content)
		buffer.Place(kind.from, false)
		buffer.Move(kind.motion, false)
		if buffer.Caret() != kind.to {
			test.Errorf("%s: from %v the caret went to %v, want %v", kind.name, kind.from, buffer.Caret(), kind.to)
		}
	}
}

func TestMovingDownAndUpComesBackToTheSameColumn(test *testing.T) {
	buffer := New("a long line\nab\nanother long line")
	buffer.Place(at(0, 8), false)
	buffer.Move(Down, false)
	buffer.Move(Down, false)
	if buffer.Caret() != at(2, 8) {
		test.Errorf("past a short line the caret is at %v, want column 8 again", buffer.Caret())
	}
}

func TestExtendingAMotionSelects(test *testing.T) {
	buffer := New("hello world")
	buffer.Place(at(0, 6), false)
	buffer.Move(LineEnd, true)
	if buffer.SelectedText() != "world" {
		test.Errorf("selected %q, want %q", buffer.SelectedText(), "world")
	}
	buffer.Move(Left, false)
	if _, _, selected := buffer.Selection(); selected || buffer.Caret() != at(0, 6) {
		test.Errorf("left without Shift left the caret at %v, want the selection's start, closed", buffer.Caret())
	}
}

func TestASelectionReadsTheSameBackwards(test *testing.T) {
	buffer := New("one\ntwo\nthree")
	buffer.Place(at(2, 2), false)
	buffer.Place(at(0, 1), true)
	if got := buffer.SelectedText(); got != "ne\ntwo\nth" {
		test.Errorf("selected %q, want %q", got, "ne\ntwo\nth")
	}
}

func TestSelectingAWordTakesTheRunOfOneKind(test *testing.T) {
	buffer := New("call(first_name, 2)")
	buffer.SelectWord(at(0, 8))
	if buffer.SelectedText() != "first_name" {
		test.Errorf("selected %q, want first_name", buffer.SelectedText())
	}
}

func TestSelectAllTakesTheWholeText(test *testing.T) {
	buffer := New("one\ntwo\n")
	buffer.SelectAll()
	if buffer.SelectedText() != "one\ntwo\n" {
		test.Errorf("selected %q", buffer.SelectedText())
	}
}

func TestMovingByLinesStopsAtTheEnds(test *testing.T) {
	buffer := New("1\n2\n3\n4")
	buffer.MoveLines(10, false)
	if buffer.Caret().Line != 3 {
		test.Errorf("ten lines down from the first is line %d, want the last", buffer.Caret().Line)
	}
	buffer.MoveLines(-2, false)
	if buffer.Caret().Line != 1 {
		test.Errorf("two lines up is line %d, want 1", buffer.Caret().Line)
	}
}
