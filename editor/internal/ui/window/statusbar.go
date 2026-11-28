package window

import (
	"fmt"
	"strconv"

	"github.com/akrck02/coddy/editor/internal/format"
	"github.com/akrck02/coddy/editor/internal/ui/pane"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// buildStatusBar says where the development stands: its projects and changed files, how far a
// mode's work has gone, and where the open file's branch stands against its base.
func (application *Window) buildStatusBar(context *ui.Context) {
	palette := context.Theme
	small := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(28), Padding: ui.PadXY(14, 0), Gap: 16, AlignY: ui.Center}, func() {
		application.buildCounts(context, small)
		if application.loading > 0 {
			context.Text(application.status.loading.Of(application.loading, readingWords), small)
		}
		context.Spacer()
		if application.viewer.IsOpen() {
			application.buildBranch(context, small)
		}
		if !application.workspace.WorkingTree {
			context.Text("commits only", ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Warning})
		}
		application.buildServersButton(context)
		if application.settings.ReadOnly {
			context.Text("read-only", small)
		}
	})
}

// buildCounts says, each with its icon, how many files changed, in how many of the projects, and
// how far the mode's work has gone; their tooltips say it in words.
func (application *Window) buildCounts(context *ui.Context, small ui.TextStyle) {
	counted := func(icon, text, words string, more func()) {
		// a place the pointer can rest on, which a tooltip needs: one after a plain box would
		// name the control before it, the menu's button
		context.Clickable(words, ui.Layout{Gap: 5, AlignY: ui.Center}, ui.Looks{}, func() {
			context.IconSized(icon, 13, context.Theme.InkQuiet)
			if more != nil {
				more()
			}
			context.Text(text, small)
		})
		context.Tooltip(words)
	}
	status := &application.status
	files, changed, projects := len(application.changedOrder), application.projectsWithChanges(), len(application.workspace.Projects)
	counted("file-diff", status.files.Of(files, strconv.Itoa), status.filesWords.Of(files, changedFilesWords), nil)
	counted("folder-git-2", status.projects.Of([2]int{changed, projects}, fraction),
		status.projectsWords.Of([2]int{changed, projects}, projectsWords), nil)
	if viewed, total, words := application.progress(); total > 0 {
		counted("eye", status.progress.Of([2]int{viewed, total}, fraction), words, func() { application.buildProgress(context, viewed, total) })
	}
}

// buildProgress is a small bar of how far the mode's work has gone, green once it is done.
func (application *Window) buildProgress(context *ui.Context, viewed, total int) {
	palette := context.Theme
	colour := palette.InkQuiet
	if viewed == total {
		colour = palette.Positive
	}
	context.Paint(ui.Layout{Width: ui.Fixed(56), Height: ui.Fixed(4)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		drawing.Rectangle(area, 2, application.look.Hover.Mix(palette.Ink, 0.12), draw2d.Color{}, 0)
		if viewed > 0 {
			done := area
			done.Width = area.Width * float32(viewed) / float32(total)
			drawing.Rectangle(done, 2, colour, draw2d.Color{}, 0)
		}
	})
}

// buildBranch says where the open file's branch stands against its base, as the project is now.
func (application *Window) buildBranch(context *ui.Context, small ui.TextStyle) {
	project := application.viewer.Project
	for _, current := range application.workspace.Projects {
		if current.Path == project.Path {
			project = current
		}
	}
	if project.Base == "" || project.IsPlainFolder() {
		return // a plain folder has no branch
	}
	branch := branchState{project.Branch, project.Base, project.Ahead}
	context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
		context.IconSized("git-branch", 12, context.Theme.InkQuiet)
		context.Text(application.status.branch.Of(branch, branchWords), small)
	})
	context.Tooltip(application.status.branchWords.Of(branch, aheadWords))
}

// statusWords are the status bar's words, each kept until what it says changes.
type statusWords struct {
	loading, files, filesWords pane.Words[int]
	projects, projectsWords    pane.Words[[2]int]
	progress                   pane.Words[[2]int]
	branch, branchWords        pane.Words[branchState]
}

// branchState is where a project's branch stands: its name, its base and how far ahead it is.
type branchState struct {
	branch, base string
	ahead        int
}

// The status bar's words, each made only when what it says changes.
func readingWords(count int) string        { return fmt.Sprintf("reading %d…", count) }
func changedFilesWords(count int) string   { return format.Count(count, "file") + " changed" }
func fraction(parts [2]int) string         { return fmt.Sprintf("%d/%d", parts[0], parts[1]) }
func branchWords(state branchState) string { return state.branch + " ← " + state.base }

func projectsWords(parts [2]int) string {
	return fmt.Sprintf("%d of %d projects have changes", parts[0], parts[1])
}

func aheadWords(state branchState) string {
	return fmt.Sprintf("%s is %s ahead of %s", state.branch, format.Count(state.ahead, "commit"), state.base)
}
