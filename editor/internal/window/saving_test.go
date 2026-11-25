package window

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/viewer"
	"github.com/akrck02/coddy/editor/internal/workspace"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

// editingKeeper opens the file the development left alone, whole, with the keyboard in it.
func editingKeeper(test *testing.T) (*Window, *uitest.Window, string) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/keeper.go"})
	window := newWindow(test, application)
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	if !application.editing() {
		test.Fatal("keeper.go is not being edited")
	}
	return application, window, filepath.Join(application.session.Projects[0].Path, "src", "keeper.go")
}

func TestASavedFileShowsAsChanged(test *testing.T) {
	application, window, path := editingKeeper(test)
	typeText(window, "// ")
	press(window, input.KeyS, input.KeySuper)
	content, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(content), "// package lamp") {
		test.Fatalf("the file on disk holds %q", content)
	}
	if application.viewer.Unsaved() {
		test.Error("the file still has edits not saved")
	}
	if _, changed := application.session.Projects[0].ChangeOf("src/keeper.go"); !changed {
		test.Error("the project does not count the file saved as changed")
	}
	if application.viewer.Edit == nil || application.viewer.Edit.Caret().Column != 3 {
		test.Error("saving lost the caret")
	}
}

func TestSavingAFileChangedOnDiskAsks(test *testing.T) {
	application, window, path := editingKeeper(test)
	typeText(window, "x")
	os.WriteFile(path, []byte("theirs\n"), 0o644)
	press(window, input.KeyS, input.KeySuper)
	if !application.questions.conflictOpen {
		test.Fatal("saving over a file changed on disk did not ask")
	}
	if content, _ := os.ReadFile(path); string(content) != "theirs\n" {
		test.Fatalf("the file was written before the answer: %q", content)
	}
	answer(test, window, "Keep mine")
	if content, _ := os.ReadFile(path); !strings.HasPrefix(string(content), "xpackage") {
		test.Errorf("keeping mine wrote %q", content)
	}
}

func TestTakingTheirsReadsTheFileAgain(test *testing.T) {
	application, window, path := editingKeeper(test)
	typeText(window, "x")
	os.WriteFile(path, []byte("theirs\n"), 0o644)
	press(window, input.KeyS, input.KeySuper)
	answer(test, window, "Take theirs")
	if application.viewer.Edit == nil || application.viewer.Edit.Text() != "theirs\n" || application.viewer.Unsaved() {
		test.Error("taking theirs did not show the file as it is on disk")
	}
}

func TestClosingAnEditedFileAsks(test *testing.T) {
	application, window, path := editingKeeper(test)
	typeText(window, "x")
	other := workspace.FileReference{Project: 0, Path: "src/lamp.go"}
	application.openFile(other)
	window.Frames(2)
	if !application.questions.unsavedOpen || application.viewer.Reference == other {
		test.Fatal("opening another file did not ask about the edits")
	}
	answer(test, window, "Cancel")
	if application.viewer.Reference == other || !application.viewer.Unsaved() {
		test.Fatal("cancelling lost the file or its edits")
	}
	application.openFile(other)
	window.Frames(2)
	answer(test, window, "Don't save")
	if application.viewer.Reference != other {
		test.Error("not saving did not open the other file")
	}
	if content, _ := os.ReadFile(path); strings.HasPrefix(string(content), "x") {
		test.Error("not saving saved")
	}
}

func TestQuittingWithEditsAsks(test *testing.T) {
	application, window, _ := editingKeeper(test)
	typeText(window, "x")
	press(window, input.KeyQ, input.KeySuper)
	if !application.questions.unsavedOpen {
		test.Error("⌘Q with edits not saved did not ask")
	}
}

func TestAnEditedFileShowsADot(test *testing.T) {
	application, window, _ := editingKeeper(test)
	typeText(window, "x")
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "keeper.go ●") {
		test.Errorf("the header shows no dot after the name: %q", shown)
	}
	if !application.Unsaved(application.viewer.Reference) {
		test.Error("the tree is not told the file is not saved")
	}
}

// answer presses a button of the question open.
func answer(test *testing.T, window *uitest.Window, button string) {
	test.Helper()
	if err := window.Click(button); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
}

// The review of proposal 3 found these: each test keeps one from coming back.

func TestSavingAfterReadingAgainStillAsksAboutAChangeOnDisk(test *testing.T) {
	application, window, path := editingKeeper(test)
	typeText(window, "x")
	os.WriteFile(path, []byte("theirs\n"), 0o644)
	press(window, input.KeyR, input.KeySuper)
	press(window, input.KeyS, input.KeySuper)
	if !application.questions.conflictOpen {
		test.Error("after ⌘R, saving overwrote a change on disk without asking")
	}
	if content, _ := os.ReadFile(path); string(content) != "theirs\n" {
		test.Errorf("the change on disk was overwritten: %q", content)
	}
}

func TestCountingOnlyCommitsAsksBeforeLosingEdits(test *testing.T) {
	application, window, _ := editingKeeper(test)
	typeText(window, "x")
	press(window, input.KeyW, input.KeySuper, input.KeyShift)
	if !application.questions.unsavedOpen || !application.session.WorkingTree {
		test.Fatal("⌘⇧W read the files again without asking about the edits")
	}
	answer(test, window, "Cancel")
	if !application.viewer.Unsaved() {
		test.Error("cancelling lost the edits")
	}
}

func TestSavingWhileTheFileIsReadAgainDoesNotCrash(test *testing.T) {
	application, window, _ := editingKeeper(test)
	typeText(window, "x")
	application.viewer.Document = nil // as Reopen leaves it until the file is read again
	typeText(window, "d")             // types, rather than switching to the diff
	if !strings.HasPrefix(application.viewer.Edit.Text(), "xd") || application.viewer.Mode != viewer.FileView {
		test.Error("a letter typed while the file was read again ran a command")
	}
	if !application.save() {
		test.Error("the edits could not be saved while the file was read again")
	}
}

func TestSavingInTheReviewModeKeepsTheWholeFile(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Mode: "Review"})
	window := newWindow(test, application)
	application.focus = viewerPane
	press(window, input.KeyD, input.KeySuper)
	typeText(window, "// ")
	press(window, input.KeyS, input.KeySuper)
	if application.viewer.Mode != viewer.FileView || !application.editing() {
		test.Error("saving went back to the diff and stopped editing")
	}
}

func TestSavingFromTheQuestionAfterAConflictStillOpensTheOtherFile(test *testing.T) {
	application, window, path := editingKeeper(test)
	typeText(window, "x")
	os.WriteFile(path, []byte("theirs\n"), 0o644)
	other := workspace.FileReference{Project: 0, Path: "src/lamp.go"}
	application.openFile(other)
	window.Frames(2)
	answer(test, window, "Save")
	answer(test, window, "Keep mine")
	if application.viewer.Reference != other {
		test.Error("after keeping mine, the file asked for did not open")
	}
}
