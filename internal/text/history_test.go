package text

import "testing"

func TestAWordTypedIsUndoneInOneGo(test *testing.T) {
	buffer := New("")
	typed(buffer, "hello world")
	buffer.Undo()
	if buffer.Text() != "hello" {
		test.Errorf("one undo left %q, want the first word", buffer.Text())
	}
	buffer.Undo()
	if buffer.Text() != "" || buffer.Undo() {
		test.Errorf("two undos left %q, want nothing more to undo", buffer.Text())
	}
}

func TestRedoMakesTheUndoneEditAgain(test *testing.T) {
	buffer := New("a")
	buffer.Move(TextEnd, false)
	typed(buffer, "bc")
	buffer.Undo()
	buffer.Redo()
	if buffer.Text() != "abc" || buffer.Caret() != at(0, 3) {
		test.Errorf("after undo and redo: %q, caret %v", buffer.Text(), buffer.Caret())
	}
	if buffer.Redo() {
		test.Error("there was a second redo")
	}
}

func TestAnEditAfterUndoingForgetsTheRedo(test *testing.T) {
	buffer := New("")
	typed(buffer, "one")
	buffer.Undo()
	typed(buffer, "two")
	if buffer.Redo() || buffer.Text() != "two" {
		test.Errorf("redo after a new edit gave %q", buffer.Text())
	}
}

func TestDeletingSeveralCharactersIsUndoneInOneGo(test *testing.T) {
	buffer := New("abcdef")
	buffer.Place(at(0, 5), false)
	buffer.Backspace()
	buffer.Backspace()
	buffer.Delete()
	if buffer.Text() != "abc" {
		test.Fatalf("deleting left %q", buffer.Text())
	}
	buffer.Undo()
	if buffer.Text() != "abcdef" || buffer.Caret() != at(0, 5) {
		test.Errorf("one undo gave %q with the caret at %v, want everything back", buffer.Text(), buffer.Caret())
	}
}

func TestUndoingAReplacedSelectionBringsItBack(test *testing.T) {
	buffer := New("one\ntwo\nthree")
	buffer.Place(at(0, 1), false)
	buffer.Place(at(2, 2), true)
	buffer.Paste("X")
	buffer.Undo()
	if buffer.Text() != "one\ntwo\nthree" || buffer.SelectedText() != "ne\ntwo\nth" {
		test.Errorf("undo gave %q with %q selected", buffer.Text(), buffer.SelectedText())
	}
}

func TestATextIsModifiedUntilSavedOrUndoneBack(test *testing.T) {
	buffer := New("x")
	if buffer.Modified() {
		test.Error("a text just read is modified")
	}
	typed(buffer, "y")
	if !buffer.Modified() {
		test.Error("a text typed in is not modified")
	}
	buffer.Undo()
	if buffer.Modified() {
		test.Error("a text undone back to as read is modified")
	}
	typed(buffer, "z")
	buffer.MarkSaved()
	typed(buffer, "w")
	buffer.Undo()
	if buffer.Modified() {
		test.Error("a text undone back to as saved is modified")
	}
}

func TestTheVersionChangesWithEveryChange(test *testing.T) {
	buffer := New("")
	seen := map[int]bool{buffer.Version(): true}
	typed(buffer, "a")
	buffer.Insert("\n")
	for _, step := range []func() bool{buffer.Undo, buffer.Redo} {
		before := buffer.Version()
		step()
		if buffer.Version() == before {
			test.Error("undoing or redoing kept the version")
		}
	}
	if seen[buffer.Version()] {
		test.Error("the text after edits has the version of the text as read")
	}
}

func TestOnlyTheLastStepsAreKept(test *testing.T) {
	buffer := New("")
	for range mostUndoSteps + 10 {
		buffer.Paste("x")
	}
	undone := 0
	for buffer.Undo() {
		undone++
	}
	if undone != mostUndoSteps {
		test.Errorf("undid %d steps, want %d", undone, mostUndoSteps)
	}
}
