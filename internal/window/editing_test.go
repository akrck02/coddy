package window

import (
	"os"
	"strings"
	"testing"

	"github.com/akrck02/go-renderer/input"
)

func TestTypingIntoAFileEditsIt(test *testing.T) {
	application, window := editingAFile(test)
	before := application.viewer.Edit.Text()
	typeText(window, "go")
	press(window, input.KeyEnter)
	edited := application.viewer.Edit.Text()
	if !strings.HasPrefix(edited, "go\n") || !strings.HasSuffix(edited, before) {
		test.Fatalf("typing gave %q…", edited[:min(len(edited), 20)])
	}
	press(window, input.KeyZ, input.KeySuper)
	press(window, input.KeyZ, input.KeySuper)
	if application.viewer.Edit.Text() != before {
		test.Errorf("two undos left %q…", application.viewer.Edit.Text()[:20])
	}
	press(window, input.KeyZ, input.KeySuper, input.KeyShift)
	if !strings.HasPrefix(application.viewer.Edit.Text(), "go") {
		test.Error("redo did not type the word again")
	}
}

func TestTheArrowsMoveTheCaretAndShiftSelects(test *testing.T) {
	application, window := editingAFile(test)
	press(window, input.KeyDown)
	press(window, input.KeyRight, input.KeyAlt)
	press(window, input.KeyEnd, input.KeyShift)
	start, end, selected := application.viewer.Edit.Selection()
	if !selected || start.Line != 1 || start.Column == 0 || end.Column != application.viewer.Edit.LineLength(1) {
		test.Errorf("the selection runs from %v to %v", start, end)
	}
	press(window, input.KeyDown, input.KeySuper)
	if application.viewer.Edit.Caret() != application.viewer.Edit.End() {
		test.Errorf("⌘↓ left the caret at %v", application.viewer.Edit.Caret())
	}
}

func TestCuttingAndPastingGoThroughTheClipboard(test *testing.T) {
	application, window := editingAFile(test)
	clipboard := &memoryClipboard{}
	application.tap.state.Clipboard = clipboard
	before := application.viewer.Edit.Text()
	press(window, input.KeyA, input.KeySuper)
	press(window, input.KeyX, input.KeySuper)
	if clipboard.text != before || application.viewer.Edit.Text() != "" {
		test.Fatalf("⌘A ⌘X left %d characters and copied %d", len(application.viewer.Edit.Text()), len(clipboard.text))
	}
	press(window, input.KeyV, input.KeySuper)
	if application.viewer.Edit.Text() != before {
		test.Error("⌘V did not paste the text back")
	}
}

func TestAnEditedLineIsColouredAgain(test *testing.T) {
	application, window := editingAFile(test)
	typeText(window, "x")
	view := &application.viewer
	if view.Edit.Version() == 0 || len(view.Edit.Text()) == 0 {
		test.Fatal("nothing was typed")
	}
	if line := view.Edit.Line(0); !strings.HasPrefix(line, "x") {
		test.Fatalf("the first line is %q", line)
	}
	if got := application.viewer.RowCount(); got != view.Edit.LineCount() {
		test.Errorf("the view shows %d rows for %d lines", got, view.Edit.LineCount())
	}
}

func TestReadOnlyModeTypesNothing(test *testing.T) {
	application, window := editingAFile(test)
	press(window, input.KeyL, input.KeySuper, input.KeyShift)
	if !application.ReadOnly() || !strings.Contains(strings.Join(window.Texts(), " "), "read-only") {
		test.Fatal("⌘⇧L did not turn the read-only mode on, said in the status bar")
	}
	before := application.viewer.Edit.Text()
	typeText(window, "x")
	press(window, input.KeyV, input.KeySuper)
	if application.viewer.Edit.Text() != before {
		test.Error("a read-only window typed")
	}
}

func TestReadOnlyModeSavesNothing(test *testing.T) {
	application, window, path := editingKeeper(test)
	typeText(window, "x")
	press(window, input.KeyL, input.KeySuper, input.KeyShift)
	press(window, input.KeyS, input.KeySuper)
	if application.save() {
		test.Error("a read-only window saved")
	}
	if content, _ := os.ReadFile(path); strings.HasPrefix(string(content), "x") {
		test.Error("a read-only window wrote the file")
	}
	press(window, input.KeyL) // in read-only mode nothing is edited, so the letter runs its command
	if application.ReadOnly() {
		test.Error("l did not turn the read-only mode off again")
	}
}
