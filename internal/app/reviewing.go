package app

import (
	"fmt"

	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// setMode moves between reading the files and reviewing the changes. The file open stays open,
// shown as the mode shows a file: whole in the files mode, as its diff in the review mode.
func (application *App) setMode(mode sidebarMode) {
	if application.sidebar.mode == mode {
		return
	}
	application.sidebar.mode = mode
	application.sidebar.stale = true
	if mode == reviewMode {
		application.editor.setMode(diffView)
	} else {
		application.editor.setMode(fileView)
	}
	if application.editor.isOpen() {
		application.sidebar.revealFile(application, application.editor.reference)
	}
}

// isViewed tells whether a changed file was marked as viewed as its text is now.
func (application *App) isViewed(reference fileReference) bool {
	if reference.project < 0 || reference.project >= len(application.projects) {
		return false
	}
	return application.reviewed.IsViewed(application.projects[reference.project], reference.path)
}

// reviewProgress counts the changed files viewed, in the projects shown.
func (application *App) reviewProgress() (viewed, total int) {
	for _, project := range application.projects {
		if application.hidden(project) {
			continue
		}
		projectViewed, projectTotal := application.reviewed.Progress(project)
		viewed += projectViewed
		total += projectTotal
	}
	return viewed, total
}

// toggleViewed marks a changed file as viewed, or not. Marking the file open as viewed moves on to
// the next file not viewed yet, as a reviewer who is done with one wants the next.
func (application *App) toggleViewed(reference fileReference, moveOn bool) {
	if reference.project < 0 || reference.project >= len(application.projects) {
		return
	}
	project := application.projects[reference.project]
	if _, changed := project.ChangeOf(reference.path); !changed {
		application.toast = "Only changed files are reviewed"
		return
	}
	viewed := !application.isViewed(reference)
	if err := application.reviewed.SetViewed(project, reference.path, viewed); err != nil {
		application.toast = "Could not keep the mark: " + err.Error()
	}
	if !viewed {
		return
	}
	if moveOn && !application.openNextUnviewed(reference) {
		application.toast = "Every changed file has been viewed"
	}
}

// openNextUnviewed opens the first changed file after this one that was not viewed yet, going
// round to the start; false when there is none.
func (application *App) openNextUnviewed(after fileReference) bool {
	order := application.changedOrder
	start := 0
	for index, reference := range order {
		if reference == after {
			start = index + 1
			break
		}
	}
	for step := range order {
		reference := order[(start+step)%len(order)]
		if !application.isViewed(reference) {
			application.openFile(reference)
			application.sidebar.revealFile(application, reference)
			return true
		}
	}
	return false
}

// markTarget is the file v marks: the one open, or the one selected in the sidebar.
func (application *App) markTarget() (fileReference, bool) {
	if application.focus == sidebarPane {
		if row, ok := application.sidebar.selectedRow(); ok && row.kind == fileRow {
			return fileReference{row.project, row.path}, true
		}
	}
	if application.editor.isOpen() {
		return application.editor.reference, true
	}
	return fileReference{}, false
}

// buildOverview is what the review mode shows before a file is open: each project the development
// changed, where its branch stands against its base, how much it changed and how much of it has
// been viewed, to choose where to start.
func (application *App) buildOverview(context *ui.Context) {
	theme := context.Theme
	look := application.look
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: look.editor}, func() {
		context.ScrollArea("overview", ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), Padding: ui.PadXY(40, 32), Gap: 6}, func() {
			viewed, total := application.reviewProgress()
			context.Text("Review", ui.TextStyle{Size: theme.HeadingSize, Weight: draw2d.Bold})
			context.Text(fmt.Sprintf("%d changed files in %d projects · %d viewed", total, application.projectsWithChanges(), viewed),
				ui.TextStyle{Color: theme.InkSoft})
			context.Caption("v marks the file open as viewed and opens the next one · n / m go through every change · ⌘P finds any file")
			context.Box(ui.Layout{Height: ui.Fixed(16)}, nil)
			for index, project := range application.projects {
				if len(project.Changes) == 0 {
					continue
				}
				application.buildOverviewRow(context, index, project)
			}
			if total == 0 && application.loading == 0 {
				context.Caption("Nothing changed against the base in the projects open.")
			}
		})
	})
}

func (application *App) buildOverviewRow(context *ui.Context, index int, project *review.Project) {
	theme := context.Theme
	look := application.look
	viewed, total := application.reviewed.Progress(project)
	added, removed := project.TotalCounts()
	normal := ui.Look{Fill: theme.Surface, Radius: 8, Border: look.hover, BorderWidth: 1}
	hovered := normal
	hovered.Fill = look.hover
	layout := ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(16, 12), Gap: 16, AlignY: ui.Center}
	if context.Clickable("project "+project.Path, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized("git-branch", 16, theme.InkQuiet)
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 2}, func() {
			context.Text(project.Name, ui.TextStyle{Weight: draw2d.Medium})
			where := project.Branch
			if project.Base != "" {
				where = fmt.Sprintf("%s ← %s · %d commits", project.Branch, project.Base, project.Ahead)
			}
			context.Text(where, ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkQuiet})
		})
		context.Text(fmt.Sprintf("%d files", len(project.Changes)), ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkSoft})
		if added > 0 {
			context.Text(fmt.Sprintf("+%d", added), ui.TextStyle{Size: theme.SmallTextSize, Weight: draw2d.Mono, Color: theme.Positive})
		}
		if removed > 0 {
			context.Text(fmt.Sprintf("−%d", removed), ui.TextStyle{Size: theme.SmallTextSize, Weight: draw2d.Mono, Color: theme.Danger})
		}
		colour := theme.InkQuiet
		if viewed == total {
			colour = theme.Positive
		}
		context.Paint(ui.Layout{Width: ui.Fixed(64), Height: ui.Fixed(4)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			drawing.Rectangle(area, 2, look.hover.Mix(theme.Ink, 0.12), draw2d.Color{}, 0)
			if viewed > 0 {
				done := area
				done.Width = area.Width * float32(viewed) / float32(max(total, 1))
				drawing.Rectangle(done, 2, colour, draw2d.Color{}, 0)
			}
		})
		context.Text(fmt.Sprintf("%d/%d", viewed, total), ui.TextStyle{Size: theme.SmallTextSize, Weight: draw2d.Mono, Color: colour})
	}) {
		application.openFirstUnviewedOf(index)
	}
}

// openFirstUnviewedOf opens a project's first changed file not viewed yet, or its first one.
func (application *App) openFirstUnviewedOf(project int) {
	var first *fileReference
	for _, reference := range application.changedOrder {
		if reference.project != project {
			continue
		}
		if first == nil {
			chosen := reference
			first = &chosen
		}
		if !application.isViewed(reference) {
			first = &reference
			break
		}
	}
	if first != nil {
		application.openFile(*first)
		application.sidebar.revealFile(application, *first)
		application.focus = editorPane
	}
}
