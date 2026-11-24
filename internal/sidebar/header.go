package sidebar

import (
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// buildHeader names the mode, says how far it has gone if it can, and offers its buttons.
func (sidebar *Sidebar) buildHeader(context *ui.Context, window Window) {
	palette := context.Theme
	title := sidebar.listing.Name()
	titleStyle := ui.TextStyle{Weight: draw2d.Medium, Color: palette.InkSoft}
	context.Text(title, titleStyle)
	room := sidebar.headerRoom() - theme.TextWidth(context, title, titleStyle) - toolButtonWidth*2 - headerGap*3
	if header := sidebar.header; header != "" {
		progressStyle := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet}
		if progress := theme.FitText(context, header, progressStyle, room); progress != "" {
			context.Text(progress, progressStyle)
		}
	}
	context.Spacer()
	sidebar.buildChangedFilter(context, window)
	sidebar.buildFoldButton(context, window)
}

// buildFoldButton folds everything listed while something is open, and opens it all once nothing
// is: the projects, or inside a project its folders.
func (sidebar *Sidebar) buildFoldButton(context *ui.Context, window Window) {
	if sidebar.anyOpen {
		if theme.ToolButton(context, "Collapse all", "chevrons-down-up", false) {
			sidebar.setAllOpen(window, false)
		}
		return
	}
	if theme.ToolButton(context, "Expand all", "chevrons-up-down", false) {
		sidebar.setAllOpen(window, true)
	}
}

// buildEnteredHeader names the project the sidebar is in, with the way back to every project.
func (sidebar *Sidebar) buildEnteredHeader(context *ui.Context, window Window) {
	palette := context.Theme
	index := window.ProjectIndex(sidebar.Entered)
	if index < 0 {
		return
	}
	project := window.Projects()[index]
	if theme.ToolButton(context, "Every project (Esc)", "arrow-left", false) {
		sidebar.Leave(window)
		return
	}
	count, countStyle := sidebar.enteredCount(context, index, project)
	// what is left for the name and the branch once the buttons and the count have their room
	room := sidebar.headerRoom() - 2*(toolButtonWidth+headerGap) // going back, and folding
	if count != "" {
		room -= theme.TextWidth(context, count, countStyle) + headerGap
	}
	if !sidebar.listing.ChangesOnly() {
		room -= toolButtonWidth + headerGap
	}
	nameStyle := ui.TextStyle{Weight: draw2d.Medium}
	name := theme.FitText(context, project.Name, nameStyle, room)
	context.Text(name, nameStyle)
	room -= theme.TextWidth(context, name, nameStyle) + headerGap
	branchStyle := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet}
	if branch := theme.FitText(context, project.Branch, branchStyle, room); len([]rune(branch)) > 3 {
		context.Text(branch, branchStyle)
	}
	context.Spacer()
	if count != "" {
		context.Text(count, countStyle)
	}
	sidebar.buildChangedFilter(context, window)
	sidebar.buildFoldButton(context, window)
}

// enteredCount is what the mode counts in the project entered, and the style it is written in.
func (sidebar *Sidebar) enteredCount(context *ui.Context, index int, project *workspace.Project) (string, ui.TextStyle) {
	palette := context.Theme
	count, countStyle := "", ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Mono, Color: palette.Warning}
	if len(project.Changes) == 0 {
		return count, countStyle
	}
	decoration := sidebar.decorate(Row{Kind: ProjectRow, Project: index, Changed: len(project.Changes)})
	if len(decoration.Words) > 0 {
		count, countStyle.Color = decoration.Words[0].Text, decoration.Words[0].Ink.colour(palette)
	}
	if decoration.Progress != nil {
		countStyle.Color = palette.InkQuiet // the header keeps the bar's colour for the rows
	}
	return count, countStyle
}

// The sidebar's header: its padding, the gap between its parts and an icon button's width.
const (
	headerPadding   float32 = 14
	headerGap       float32 = 8
	toolButtonWidth float32 = 30
)

// headerRoom is the width inside the sidebar's header, less a little for the pane's resize edge.
func (sidebar *Sidebar) headerRoom() float32 {
	return sidebar.Width - 2*headerPadding - 4
}
