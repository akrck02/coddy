package review

import (
	"fmt"

	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// Mode is the review mode: only the files the development changed, opened on their changes,
// each marked as viewed once read, with how much is left on every project.
type Mode struct {
	window   mode.Window
	reviewed *Reviewed
}

// New makes the review mode, keeping the marks in a file at path; an empty path keeps nothing on
// disk.
func New(path string) *Mode { return &Mode{reviewed: OpenReviewed(path)} }

// Name is the sidebar's title in this mode.
func (review *Mode) Name() string { return "Review" }

// Icon is the mode's icon on the rail.
func (review *Mode) Icon() string { return "git-pull-request" }

// Description is the rail's tooltip.
func (review *Mode) Description() string {
	return "Review (2): only the changed files, as diffs, marked as viewed one by one"
}

// Start gives the mode its window.
func (review *Mode) Start(window mode.Window) { review.window = window }

// ChangesOnly is always true: only what changed is reviewed.
func (review *Mode) ChangesOnly() bool { return true }

// ShowsChanges is always true: a changed file opens on its diff.
func (review *Mode) ShowsChanges() bool { return true }

// Header says how many changed files were viewed.
func (review *Mode) Header() string {
	if viewed, total := review.Progress(); total > 0 {
		return fmt.Sprintf("%d of %d viewed", viewed, total)
	}
	return ""
}

// Progress counts the changed files viewed, and all of them, in the projects shown.
func (review *Mode) Progress() (viewed, total int) {
	for _, project := range review.window.Projects() {
		if review.window.Hidden(project) {
			continue
		}
		projectViewed, projectTotal := review.reviewed.Progress(project)
		viewed += projectViewed
		total += projectTotal
	}
	return viewed, total
}

// IsViewed tells whether a changed file was marked as viewed as its text is now.
func (review *Mode) IsViewed(file workspace.FileReference) bool {
	projects := review.window.Projects()
	if file.Project < 0 || file.Project >= len(projects) {
		return false
	}
	return review.reviewed.IsViewed(projects[file.Project], file.Path)
}

// Decorate marks each file as viewed or not, with the lines it adds and removes, and shows how
// much of each project was viewed.
func (review *Mode) Decorate(row sidebar.Row) sidebar.Decoration {
	switch row.Kind {
	case sidebar.FileRow:
		return review.decorateFile(row)
	case sidebar.ProjectRow:
		viewed, total := review.reviewed.Progress(review.window.Projects()[row.Project])
		ink := sidebar.QuietInk
		if viewed == total {
			ink = sidebar.PositiveInk
		}
		return sidebar.Decoration{
			Words:    []sidebar.Word{{Text: fmt.Sprintf("%d/%d", viewed, total), Ink: ink}},
			Progress: &sidebar.Progress{Done: viewed, Total: total, Ink: ink},
		}
	}
	return sidebar.Decoration{}
}

func (review *Mode) decorateFile(row sidebar.Row) sidebar.Decoration {
	file := workspace.FileReference{Project: row.Project, Path: row.Path}
	decoration := sidebar.Decoration{Mark: sidebar.OpenMark}
	viewed := review.IsViewed(file)
	if viewed {
		decoration.Mark = sidebar.DoneMark
	}
	counts := review.window.Projects()[row.Project].Counts[row.Path]
	if counts.Binary {
		decoration.Words = []sidebar.Word{{Text: "bin", Ink: sidebar.QuietInk}}
		return decoration
	}
	removed, added := sidebar.DangerInk, sidebar.PositiveInk
	if viewed {
		removed, added = sidebar.QuietInk, sidebar.QuietInk
	}
	if counts.Removed > 0 {
		decoration.Words = append(decoration.Words, sidebar.Word{Text: fmt.Sprintf("−%d", counts.Removed), Ink: removed})
	}
	if counts.Added > 0 {
		decoration.Words = append(decoration.Words, sidebar.Word{Text: fmt.Sprintf("+%d", counts.Added), Ink: added})
	}
	return decoration
}

// FileButton is the Viewed button of the viewer's header.
func (review *Mode) FileButton(file workspace.FileReference) (viewer.Button, bool) {
	viewed := review.IsViewed(file)
	icon := "circle"
	if viewed {
		icon = "circle-check"
	}
	return viewer.Button{Icon: icon, Tip: "Viewed (v)", On: viewed}, true
}

// Press marks a changed file as viewed, or not. Marking a file as viewed with moveOn opens the
// next file not viewed yet, as a reviewer who is done with one wants the next.
func (review *Mode) Press(file workspace.FileReference, moveOn bool) {
	projects := review.window.Projects()
	if file.Project < 0 || file.Project >= len(projects) {
		return
	}
	project := projects[file.Project]
	if _, changed := project.ChangeOf(file.Path); !changed {
		review.window.Toast("Only changed files are reviewed")
		return
	}
	viewed := !review.IsViewed(file)
	if err := review.reviewed.SetViewed(project, file.Path, viewed); err != nil {
		review.window.Toast("Could not keep the mark: " + err.Error())
	}
	if viewed && moveOn && !review.openNextUnviewed(file) {
		review.window.Toast("Every changed file has been viewed")
	}
}

// openNextUnviewed opens the first changed file after this one that was not viewed yet, going
// round to the start; false when there is none.
func (review *Mode) openNextUnviewed(after workspace.FileReference) bool {
	order := review.window.ChangedFiles()
	start := 0
	for index, file := range order {
		if file == after {
			start = index + 1
			break
		}
	}
	for step := range order {
		file := order[(start+step)%len(order)]
		if review.window.Shows(file) && !review.IsViewed(file) {
			review.window.Reveal(file)
			return true
		}
	}
	return false
}

// Commands are the review mode's keys.
func (review *Mode) Commands() []mode.Command {
	return []mode.Command{{
		Description: "Mark the file as viewed and open the next one not viewed",
		Keys:        []pane.Press{pane.Key(input.KeyV, "v")},
		Do: func(int) {
			if file, inViewer, found := review.window.Target(); found {
				review.Press(file, inViewer)
			}
		},
	}}
}

// BuildInsteadOfAFile shows the overview while no file is open, when anything changed.
func (review *Mode) BuildInsteadOfAFile(context *ui.Context) bool {
	if len(review.window.ChangedFiles()) == 0 {
		return false
	}
	review.buildOverview(context)
	return true
}

func (review *Mode) projectsWithChanges() int {
	count := 0
	for _, project := range review.window.Projects() {
		if len(project.Changes) > 0 {
			count++
		}
	}
	return count
}

// buildOverview is what the review mode shows before a file is open: each project the development
// changed, where its branch stands against its base, how much it changed and how much of it has
// been viewed, to choose where to start.
func (review *Mode) buildOverview(context *ui.Context) {
	palette := context.Theme
	look := review.window.Look()
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: look.Editor}, func() {
		context.ScrollArea("overview", ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), Padding: ui.PadXY(40, 32), Gap: 6}, func() {
			viewed, total := review.Progress()
			context.Text("Review", ui.TextStyle{Size: palette.HeadingSize, Weight: draw2d.Bold})
			context.Text(fmt.Sprintf("%d changed files in %d projects · %d viewed", total, review.projectsWithChanges(), viewed),
				ui.TextStyle{Color: palette.InkSoft})
			context.Caption("v marks the file open as viewed and opens the next one · n / m go through every change · ⌘P finds any file")
			context.Box(ui.Layout{Height: ui.Fixed(16)}, nil)
			for index, project := range review.window.Projects() {
				if len(project.Changes) == 0 {
					continue
				}
				review.buildOverviewRow(context, index, project)
			}
			if total == 0 && !review.window.Loading() {
				context.Caption("Nothing changed against the base in the projects open.")
			}
		})
	})
}

func (review *Mode) buildOverviewRow(context *ui.Context, index int, project *workspace.Project) {
	palette := context.Theme
	look := review.window.Look()
	viewed, total := review.reviewed.Progress(project)
	normal := ui.Look{Fill: palette.Surface, Radius: 8, Border: look.Hover, BorderWidth: 1}
	hovered := normal
	hovered.Fill = look.Hover
	layout := ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(16, 12), Gap: 16, AlignY: ui.Center}
	if context.Clickable("project "+project.Path, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized("git-branch", 16, palette.InkQuiet)
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 2}, func() {
			context.Text(project.Name, ui.TextStyle{Weight: draw2d.Medium})
			where := project.Branch
			if project.Base != "" {
				where = fmt.Sprintf("%s ← %s · %d commits", project.Branch, project.Base, project.Ahead)
			}
			context.Text(where, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
		})
		buildCounts(context, project)
		review.buildProjectProgress(context, viewed, total)
	}) {
		review.openFirstUnviewedOf(index)
	}
}

// buildCounts says how many files a project changed, and the lines added and removed.
func buildCounts(context *ui.Context, project *workspace.Project) {
	palette := context.Theme
	added, removed := project.TotalCounts()
	context.Text(fmt.Sprintf("%d files", len(project.Changes)), ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft})
	if added > 0 {
		context.Text(fmt.Sprintf("+%d", added), ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Mono, Color: palette.Positive})
	}
	if removed > 0 {
		context.Text(fmt.Sprintf("−%d", removed), ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Mono, Color: palette.Danger})
	}
}

// buildProjectProgress is a bar of the files viewed in a project and their count, green once every
// one is.
func (review *Mode) buildProjectProgress(context *ui.Context, viewed, total int) {
	palette := context.Theme
	look := review.window.Look()
	colour := palette.InkQuiet
	if viewed == total {
		colour = palette.Positive
	}
	context.Paint(ui.Layout{Width: ui.Fixed(64), Height: ui.Fixed(4)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		drawing.Rectangle(area, 2, look.Hover.Mix(palette.Ink, 0.12), draw2d.Color{}, 0)
		if viewed > 0 {
			done := area
			done.Width = area.Width * float32(viewed) / float32(max(total, 1))
			drawing.Rectangle(done, 2, colour, draw2d.Color{}, 0)
		}
	})
	context.Text(fmt.Sprintf("%d/%d", viewed, total), ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Mono, Color: colour})
}

// openFirstUnviewedOf opens a project's first changed file not viewed yet, or its first one.
func (review *Mode) openFirstUnviewedOf(project int) {
	var first *workspace.FileReference
	for _, file := range review.window.ChangedFiles() {
		if file.Project != project {
			continue
		}
		if first == nil {
			chosen := file
			first = &chosen
		}
		if !review.IsViewed(file) {
			first = &file
			break
		}
	}
	if first != nil {
		review.window.Reveal(*first)
		review.window.FocusViewer()
	}
}
