package window

import (
	"github.com/akrck02/code-review/internal/text"
	ui "github.com/akrck02/go-ui"
)

// Editing tells whether the keyboard is in the file being edited.
func (application *Window) Editing() bool { return application.editing() }

// ReadOnly tells whether editing is off for every file.
func (application *Window) ReadOnly() bool { return application.settings.ReadOnly }

// setReadOnly turns editing off for every file, or on again. Edits not saved stay, to save once
// editing is on again.
func (application *Window) setReadOnly(on bool) {
	application.settings.ReadOnly = on
	application.toast = "Editing on: type in a file opened whole"
	if on {
		application.toast = "Read-only: files are only read"
	}
}

// The steps of the command that cuts, copies and pastes, one per key.
const (
	cutting = iota + 1
	copying
	pasting
)

// clipboardEdit cuts or copies the selection to the clipboard, or pastes the clipboard over it.
func (application *Window) clipboardEdit(context *ui.Context, step int) {
	application.viewer.Apply(application, func(buffer *text.Buffer) {
		switch step {
		case cutting:
			if cut := buffer.Cut(); cut != "" {
				context.Copy(cut)
			}
		case copying:
			if selected := buffer.SelectedText(); selected != "" {
				context.Copy(selected)
			}
		case pasting:
			buffer.Paste(context.ClipboardText())
		}
	})
}

// typeInTheFile gives the frame's keys to the file being edited, when no command took them.
func (application *Window) typeInTheFile(context *ui.Context, commandRan bool) {
	if commandRan || !application.editing() || application.ModalOpen() || context.WantsKeyboard() {
		return
	}
	application.viewer.Type(application, application.keys)
}
