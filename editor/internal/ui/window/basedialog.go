package window

import (
	"strings"

	"github.com/akrck02/coddy/editor/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// baseDialog chooses what the projects are compared against.
type baseDialog struct {
	open       bool
	focusNext  bool
	ref        string
	forAll     int // 0: the project of the file open (or selected), 1: every project
	forProject string
}

func (application *Window) openBaseDialog() {
	dialog := &application.base
	dialog.open = true
	dialog.focusNext = true
	dialog.forProject = ""
	if application.viewer.IsOpen() {
		dialog.forProject = application.viewer.Project.Path
	} else if row, ok := application.sidebar.SelectedRow(); ok && row.Project >= 0 {
		dialog.forProject = application.workspace.Projects[row.Project].Path
	}
	dialog.ref = application.workspace.Base
	if dialog.forProject != "" {
		if base, chosen := application.workspace.BaseOf(dialog.forProject); chosen {
			dialog.ref = base
		}
	}
	if dialog.forProject == "" {
		dialog.forAll = 1
	}
}

func (application *Window) buildBaseDialog(context *ui.Context) {
	dialog := &application.base
	if !dialog.open {
		return
	}
	apply := false
	dismissed := context.Sheet("Compare against", ui.SheetOptions{}, func() {
		apply = application.buildBaseSheet(context)
	})
	if dismissed {
		dialog.open = false
	}
	if apply {
		dialog.open = false
		application.applyBase()
	}
}

// buildBaseSheet is what the base dialog holds: the field, which projects, and its buttons. It
// tells whether the base was asked for.
func (application *Window) buildBaseSheet(context *ui.Context) bool {
	dialog := &application.base
	apply := false
	context.Caption("A branch, a tag or a commit. Empty: each project's default branch (origin/main, master…).")
	if dialog.focusNext {
		context.FocusField("base")
		dialog.focusNext = false
	}
	if context.EntryField("base", &dialog.ref, ui.TextFieldOptions{Placeholder: "origin/master", Weight: draw2d.Mono}) {
		apply = true
	}
	if dialog.forProject != "" {
		context.Tabs("base scope", &dialog.forAll, workspace.BaseName(dialog.forProject), "Every project")
	}
	context.Row(func() {
		context.Spacer()
		if context.Button("Cancel") {
			dialog.open = false
		}
		if context.PrimaryButton("Compare") {
			apply = true
		}
	})
	return apply
}

// applyBase compares every project, or the one the dialog was opened for, against the base typed,
// leaving the pull request reviewed, if any.
func (application *Window) applyBase() {
	dialog := &application.base
	application.review = pullRequest{}
	ref := strings.TrimSpace(dialog.ref)
	if dialog.forAll == 1 || dialog.forProject == "" {
		application.workspace.SetBaseForAll(ref)
		application.reloadAll()
		return
	}
	application.workspace.SetBase(dialog.forProject, ref)
	for index, project := range application.workspace.Projects {
		if project.Path == dialog.forProject {
			application.reload(index)
		}
	}
	application.projectsChanged()
}
