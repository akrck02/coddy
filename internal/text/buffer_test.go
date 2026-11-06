package text

import "testing"

func TestATextIsWrittenBackAsItWasRead(test *testing.T) {
	for _, content := range []string{"", "one", "one\n", "one\ntwo", "one\r\ntwo\r\n", "\n\n", "ñandú\n"} {
		if got := New(content).Text(); got != content {
			test.Errorf("read %q, wrote back %q", content, got)
		}
	}
}

func TestAnEmptyTextHasOneEmptyLine(test *testing.T) {
	buffer := New("")
	if buffer.LineCount() != 1 || buffer.Line(0) != "" || buffer.End() != (Position{}) {
		test.Errorf("an empty text has %d lines, the first %q, ending at %v", buffer.LineCount(), buffer.Line(0), buffer.End())
	}
}

func TestATextEndingInALineBreakEndsOnAnEmptyLine(test *testing.T) {
	buffer := New("one\ntwo\n")
	if buffer.LineCount() != 3 || buffer.End() != (Position{Line: 2}) {
		test.Errorf("got %d lines ending at %v, want 3 ending at the start of the third", buffer.LineCount(), buffer.End())
	}
}

func TestAPositionPlacedOutsideTheTextIsKeptInside(test *testing.T) {
	buffer := New("one\ntwo")
	cases := []struct{ asked, placed Position }{
		{Position{Line: 9, Column: 9}, Position{Line: 1, Column: 3}},
		{Position{Line: -1, Column: -1}, Position{}},
		{Position{Line: 0, Column: 7}, Position{Line: 0, Column: 3}},
	}
	for _, kind := range cases {
		buffer.Place(kind.asked, false)
		if buffer.Caret() != kind.placed {
			test.Errorf("placing at %v put the caret at %v, want %v", kind.asked, buffer.Caret(), kind.placed)
		}
	}
}

func TestColumnsCountCharactersNotBytes(test *testing.T) {
	buffer := New("ñandú")
	buffer.Move(TextEnd, false)
	if buffer.Caret().Column != 5 {
		test.Errorf("the end of ñandú is at column %d, want 5", buffer.Caret().Column)
	}
}

func TestMixedLineEndingsAreKeptEachAsItWas(test *testing.T) {
	const content = "a\nb\r\nc\n"
	buffer := New(content)
	if buffer.Text() != content {
		test.Fatalf("read and written back: %q", buffer.Text())
	}
	buffer.Place(Position{Line: 1, Column: 1}, false)
	buffer.Delete() // joins b and c, taking out the "\r\n"
	buffer.Undo()
	if buffer.Text() != content {
		test.Errorf("undoing a line break taken out gave %q", buffer.Text())
	}
}

func TestANewLineBreakTakesTheTextsUsualEnding(test *testing.T) {
	buffer := New("one\r\ntwo\r\n")
	buffer.Place(Position{Line: 0, Column: 3}, false)
	buffer.Insert("\n")
	if buffer.Text() != "one\r\n\r\ntwo\r\n" {
		test.Errorf("got %q", buffer.Text())
	}
}
