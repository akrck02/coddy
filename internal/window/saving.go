package window

import (
	"errors"

	"github.com/akrck02/code-review/internal/workspace"
	ui "github.com/akrck02/go-ui"
	platform "github.com/akrck02/go-ui/window"
)

// questions are the two dialogs about edits: what to do with edits not saved before they would be
// lost (then is what was asked for, run once they are saved or discarded), and what to do when the
// file changed on disk since it was read.
type questions struct {
	unsavedOpen  bool
	then         func()
	conflictOpen bool
}

// Unsaved tells whether a file has edits not saved: only the file open can.
func (application *Window) Unsaved(file workspace.FileReference) bool {
	return application.viewer.Unsaved() && application.viewer.Reference == file
}

// keepingEdits does what was asked for once the edits of the file open are safe: at once when
// there are none, else after asking to save or discard them.
func (application *Window) keepingEdits(then func()) {
	if !application.viewer.Unsaved() {
		then()
		return
	}
	application.questions.unsavedOpen, application.questions.then = true, then
}

// quit closes the window, asking first about edits not saved.
func (application *Window) quit() { application.keepingEdits(platform.Quit) }

// save writes the file being edited, unless it changed on disk since it was read: then it asks.
func (application *Window) save() bool {
	view := &application.viewer
	if view.Edit == nil || application.settings.ReadOnly {
		return false
	}
	return application.write(view.Document.Stamp)
}

// write saves the file being edited over the file on disk as read describes it, and reads its
// project again so the tree, the counts and the diff show the change.
func (application *Window) write(read workspace.Stamp) bool {
	view := &application.viewer
	content := view.Edit.Text()
	stamp, err := workspace.Save(view.Project, view.Reference.Path, content, read)
	if errors.Is(err, workspace.ChangedOnDisk) {
		application.questions.conflictOpen = true
		return false
	}
	if err != nil {
		application.toast = "Not saved: " + err.Error()
		return false
	}
	view.Saved(content, stamp)
	application.toast = "Saved " + view.Reference.Path
	if index := application.projectIndex(view.Project.Path); index >= 0 {
		application.reload(index)
	}
	return true
}

// keepMine saves over the file changed on disk.
func (application *Window) keepMine() {
	view := &application.viewer
	if now, err := workspace.StampOf(view.Project, view.Reference.Path); err == nil {
		application.write(now)
	}
}

// takeTheirs drops the edits and reads the file again as it is on disk.
func (application *Window) takeTheirs() {
	view := &application.viewer
	view.Edit = nil
	delete(application.documentCache, view.Reference)
	view.Document = nil
	application.openFile(view.Reference)
}

// buildQuestions shows the dialog about edits that is open, if one is.
func (application *Window) buildQuestions(context *ui.Context) {
	asked := &application.questions
	name := baseName(application.viewer.Reference.Path)
	if asked.unsavedOpen {
		application.ask(context, "Save the changes to "+name+"?", "Your changes are lost if you don't save them.",
			[2]string{"Save", "Don't save"}, &asked.unsavedOpen, application.answerUnsaved)
	}
	if asked.conflictOpen {
		application.ask(context, name+" changed on disk", "Someone changed the file since it was read.",
			[2]string{"Keep mine", "Take theirs"}, &asked.conflictOpen, func(first bool) {
				if first {
					application.keepMine()
				} else {
					application.takeTheirs()
				}
			})
	}
}

// answerUnsaved saves or discards the edits, then does what was asked for, unless saving failed.
func (application *Window) answerUnsaved(save bool) {
	then := application.questions.then
	application.questions.then = nil
	if save && !application.save() {
		return
	}
	if !save {
		application.viewer.Discard()
	}
	if then != nil {
		then()
	}
}

// ask shows a question with two answers and Cancel; answer hears whether the first was chosen.
func (application *Window) ask(context *ui.Context, title, caption string, answers [2]string, open *bool, answer func(first bool)) {
	chosen := 0
	dismissed := context.Sheet(title, ui.SheetOptions{}, func() {
		context.Caption(caption)
		context.Row(func() {
			context.Spacer()
			if context.Button("Cancel") {
				*open = false
			}
			if context.Button(answers[1]) {
				chosen = 2
			}
			if context.PrimaryButton(answers[0]) {
				chosen = 1
			}
		})
	})
	if dismissed {
		*open = false
	}
	if chosen > 0 {
		*open = false
		answer(chosen == 1)
	}
}
