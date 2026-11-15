package text

import "testing"

// typed is a text typed one character at a time.
func typed(buffer *Buffer, text string) {
	for _, character := range text {
		buffer.Insert(string(character))
	}
}

func TestTypingPutsTheTextAtTheCaret(test *testing.T) {
	buffer := New("fun main")
	buffer.Place(at(0, 3), false)
	typed(buffer, "c")
	if buffer.Text() != "func main" || buffer.Caret() != at(0, 4) {
		test.Errorf("got %q with the caret at %v", buffer.Text(), buffer.Caret())
	}
}

func TestALineBreakSplitsTheLine(test *testing.T) {
	buffer := New("onetwo")
	buffer.Place(at(0, 3), false)
	buffer.Insert("\n")
	if buffer.Text() != "one\ntwo" || buffer.Caret() != at(1, 0) {
		test.Errorf("got %q with the caret at %v", buffer.Text(), buffer.Caret())
	}
}

func TestTypingOverASelectionReplacesIt(test *testing.T) {
	buffer := New("hello world")
	buffer.SelectWord(at(0, 8))
	typed(buffer, "there")
	if buffer.Text() != "hello there" {
		test.Errorf("got %q", buffer.Text())
	}
}

func TestPastingSeveralLinesEndsAfterThem(test *testing.T) {
	buffer := New("ab")
	buffer.Place(at(0, 1), false)
	buffer.Paste("1\r\n2\n3")
	if buffer.Text() != "a1\n2\n3b" || buffer.Caret() != at(2, 1) {
		test.Errorf("got %q with the caret at %v", buffer.Text(), buffer.Caret())
	}
}

func TestDeletingJoinsLines(test *testing.T) {
	buffer := New("one\ntwo")
	buffer.Place(at(1, 0), false)
	buffer.Backspace()
	if buffer.Text() != "onetwo" || buffer.Caret() != at(0, 3) {
		test.Errorf("Backspace at a line's start gave %q, caret %v", buffer.Text(), buffer.Caret())
	}
	buffer.Delete()
	if buffer.Text() != "onewo" {
		test.Errorf("Delete gave %q", buffer.Text())
	}
}

func TestDeletingAtTheEndsChangesNothing(test *testing.T) {
	buffer := New("x")
	if buffer.Backspace() || buffer.Text() != "x" {
		test.Errorf("Backspace at the start changed the text to %q", buffer.Text())
	}
	buffer.Move(TextEnd, false)
	if buffer.Delete() || buffer.Text() != "x" {
		test.Errorf("Delete at the end changed the text to %q", buffer.Text())
	}
}

func TestDeletingAWordBackTakesTheWord(test *testing.T) {
	buffer := New("go build now")
	buffer.Place(at(0, 8), false)
	buffer.BackspaceWord()
	if buffer.Text() != "go  now" {
		test.Errorf("got %q", buffer.Text())
	}
}

func TestCuttingTakesTheSelectionOut(test *testing.T) {
	buffer := New("keep this")
	buffer.SelectWord(at(0, 6))
	if cut := buffer.Cut(); cut != "this" || buffer.Text() != "keep " {
		test.Errorf("cut %q, left %q", cut, buffer.Text())
	}
}
