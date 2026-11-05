package app

import (
	"fmt"
	"strings"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// sidebarMode is what the sidebar lists.
type sidebarMode int

const (
	filesMode  sidebarMode = iota // every file, to read the code: the default
	reviewMode                    // only what the development changed, to review it file by file
)

type rowKind int

const (
	projectRow rowKind = iota
	folderRow
	fileRow
	noteRow // a line of explanation: projects hidden, a project still loading
)

// treeRow is a row of the sidebar.
type treeRow struct {
	kind     rowKind
	project  int
	path     string // a folder's or a file's path in the project
	label    string
	depth    int
	expanded bool
	status   git.Status
	changed  int // changed files inside a project or a folder
	quiet    bool
}

type sidebarState struct {
	mode sidebarMode
	// expanded keeps the folders and projects opened or closed by hand, by key; the others follow
	// the mode's default (everything open in the review mode, closed in the files mode).
	expanded map[string]bool
	trees    map[treeKey]*review.Folder
	rows     []treeRow
	stale    bool
	selected int
	list     scroller
	width    float32
	// entered is the project the sidebar was taken into, by its path: then only its tree is
	// listed, from its top, as if it were the only project open. Empty lists every project.
	entered string
	// onlyChanged lists, in the files mode, only the files the development changed, still opened
	// whole with their changes marked: a lighter way in than the review mode.
	onlyChanged bool
}

type treeKey struct {
	project int
	mode    sidebarMode
	changed bool
}

func newSidebar() sidebarState {
	return sidebarState{expanded: map[string]bool{}, trees: map[treeKey]*review.Folder{}, stale: true, width: 380, list: scroller{rowHeight: sidebarRowHeight}}
}

func (sidebar *sidebarState) invalidate() {
	sidebar.trees = map[treeKey]*review.Folder{}
	sidebar.stale = true
}

func (sidebar *sidebarState) expandedKey(project int, path string) string {
	return fmt.Sprintf("%d|%t|%d|%s", sidebar.mode, sidebar.listsChangesOnly(), project, path)
}

func (sidebar *sidebarState) isExpanded(application *App, project int, path string, isProject bool) bool {
	if open, chosen := sidebar.expanded[sidebar.expandedKey(project, path)]; chosen {
		return open
	}
	if sidebar.listsChangesOnly() {
		return true
	}
	return isProject && len(application.projects) == 1
}

func (sidebar *sidebarState) tree(application *App, project int) *review.Folder {
	key := treeKey{project, sidebar.mode, sidebar.listsChangesOnly()}
	if tree, built := sidebar.trees[key]; built {
		return tree
	}
	model := application.projects[project]
	isChanged := func(path string) bool { _, changed := model.ChangeOf(path); return changed }
	var tree *review.Folder
	if sidebar.listsChangesOnly() {
		tree = review.BuildTree(changedPaths(model), isChanged)
	} else {
		tree = review.BuildTree(model.Files, isChanged)
	}
	sidebar.trees[key] = tree
	return tree
}

// rebuild lays the projects out as rows, keeping the selected row on the same thing.
func (sidebar *sidebarState) rebuild(application *App) {
	var previous treeRow
	hadSelection := sidebar.selected >= 0 && sidebar.selected < len(sidebar.rows)
	if hadSelection {
		previous = sidebar.rows[sidebar.selected]
	}
	sidebar.rows = sidebar.rows[:0]
	hidden := 0
	if sidebar.entered != "" && application.projectIndex(sidebar.entered) < 0 {
		sidebar.entered = "" // the project was closed
	}
	for index, project := range application.projects {
		if sidebar.entered != "" {
			if project.Path == sidebar.entered {
				sidebar.addProjectContent(application, index, project, 0)
			}
			continue
		}
		if application.hidden(project) {
			hidden++
			continue
		}
		expanded := sidebar.isExpanded(application, index, "", true)
		sidebar.rows = append(sidebar.rows, treeRow{
			kind: projectRow, project: index, label: project.Name, expanded: expanded, changed: len(project.Changes),
			quiet: project.Loaded && len(project.Changes) == 0,
		})
		if expanded {
			sidebar.addProjectContent(application, index, project, 1)
		}
	}
	if hidden > 0 {
		sidebar.rows = append(sidebar.rows, treeRow{kind: noteRow, project: -1, quiet: true,
			label: fmt.Sprintf("%d projects without changes hidden (h shows them)", hidden)})
	}
	sidebar.stale = false
	sidebar.list.count = len(sidebar.rows)
	if hadSelection {
		for index, row := range sidebar.rows {
			if row.kind == previous.kind && row.project == previous.project && row.path == previous.path {
				sidebar.selected = index
				return
			}
		}
	}
	sidebar.selected = min(max(sidebar.selected, 0), max(len(sidebar.rows)-1, 0))
}

// addProjectContent lists a project's tree at a depth, or why it cannot be listed yet.
func (sidebar *sidebarState) addProjectContent(application *App, index int, project *review.Project, depth int) {
	if !project.Loaded {
		sidebar.rows = append(sidebar.rows, treeRow{kind: noteRow, project: index, label: "Reading…", depth: depth, quiet: true})
		return
	}
	if project.Error != "" {
		sidebar.rows = append(sidebar.rows, treeRow{kind: noteRow, project: index, label: project.Error, depth: depth, quiet: true})
	}
	sidebar.addFolder(application, index, sidebar.tree(application, index), depth)
}

// enter takes the sidebar into a project, so only its tree is listed.
func (sidebar *sidebarState) enter(application *App, project int) {
	if project < 0 || project >= len(application.projects) {
		return
	}
	sidebar.entered = application.projects[project].Path
	sidebar.selected = 0
	sidebar.list.offset = 0
	sidebar.rebuild(application)
	if application.editor.isOpen() && application.editor.reference.project == project {
		sidebar.revealFile(application, application.editor.reference)
	}
}

// leave goes back to every project, with the one left selected.
func (sidebar *sidebarState) leave(application *App) {
	left := application.projectIndex(sidebar.entered)
	sidebar.entered = ""
	sidebar.rebuild(application)
	for index, row := range sidebar.rows {
		if row.kind == projectRow && row.project == left {
			sidebar.selected = index
			sidebar.list.reveal(index)
			return
		}
	}
}

func (sidebar *sidebarState) addFolder(application *App, project int, folder *review.Folder, depth int) {
	model := application.projects[project]
	for _, child := range folder.Folders {
		expanded := sidebar.isExpanded(application, project, child.Path, false)
		sidebar.rows = append(sidebar.rows, treeRow{
			kind: folderRow, project: project, path: child.Path, label: child.Name, depth: depth,
			expanded: expanded, changed: child.Changed,
		})
		if expanded {
			sidebar.addFolder(application, project, child, depth+1)
		}
	}
	for _, path := range folder.Files {
		change, _ := model.ChangeOf(path)
		sidebar.rows = append(sidebar.rows, treeRow{
			kind: fileRow, project: project, path: path, label: review.BaseName(path), depth: depth, status: change.Status,
		})
	}
}

// setExpanded opens or closes a project or a folder.
func (sidebar *sidebarState) setExpanded(row treeRow, open bool) {
	sidebar.expanded[sidebar.expandedKey(row.project, row.path)] = open
	sidebar.stale = true
}

// revealFile opens the folders around a file and selects it.
func (sidebar *sidebarState) revealFile(application *App, reference fileReference) {
	project := application.projects[reference.project]
	if sidebar.entered != "" && sidebar.entered != project.Path {
		sidebar.entered = "" // a file of another project: back to every project to show it
	}
	_, changed := project.ChangeOf(reference.path)
	if !changed {
		sidebar.onlyChanged = false
	}
	if sidebar.mode == reviewMode && !changed {
		sidebar.mode = filesMode
	}
	sidebar.expanded[sidebar.expandedKey(reference.project, "")] = true
	folder := sidebar.tree(application, reference.project)
	for {
		var inside *review.Folder
		for _, child := range folder.Folders {
			if strings.HasPrefix(reference.path, child.Path+"/") {
				inside = child
				break
			}
		}
		if inside == nil {
			break
		}
		sidebar.expanded[sidebar.expandedKey(reference.project, inside.Path)] = true
		folder = inside
	}
	sidebar.rebuild(application)
	for index, row := range sidebar.rows {
		if row.kind == fileRow && row.project == reference.project && row.path == reference.path {
			sidebar.selected = index
			sidebar.list.reveal(index)
			return
		}
	}
}

// activate does what Enter or a click does on a row: open a file, open or close a folder.
func (sidebar *sidebarState) activate(application *App, index int) {
	if index < 0 || index >= len(sidebar.rows) {
		return
	}
	row := sidebar.rows[index]
	switch row.kind {
	case fileRow:
		application.openFile(fileReference{row.project, row.path})
	case projectRow, folderRow:
		sidebar.setExpanded(row, !row.expanded)
	case noteRow:
		if row.project < 0 {
			application.setOnlyChanged(false)
		}
	}
}

// handleKeys moves through the rows while the sidebar has the keyboard.
func (sidebar *sidebarState) handleKeys(application *App, keys keys) {
	count := len(sidebar.rows)
	if count == 0 {
		return
	}
	moved := true
	switch {
	case keys.typed(input.KeyDown):
		sidebar.selected++
	case keys.typed(input.KeyUp):
		sidebar.selected--
	case keys.typed(input.KeyPageDown):
		sidebar.selected += sidebar.list.pageRows()
	case keys.typed(input.KeyPageUp):
		sidebar.selected -= sidebar.list.pageRows()
	case keys.pressed(input.KeyHome):
		sidebar.selected = 0
	case keys.pressed(input.KeyEnd):
		sidebar.selected = count - 1
	case keys.typed(input.KeyRight):
		sidebar.expandOrEnter()
	case keys.typed(input.KeyLeft):
		sidebar.collapseOrLeave()
	case keys.pressed(input.KeyEnter) && sidebar.rows[sidebar.selected].kind == projectRow:
		sidebar.enter(application, sidebar.rows[sidebar.selected].project)
		return
	case keys.pressed(input.KeyEnter) || keys.pressed(input.KeySpace):
		sidebar.activate(application, sidebar.selected)
		if sidebar.selected < count && sidebar.rows[sidebar.selected].kind == fileRow && keys.pressed(input.KeyEnter) {
			application.focus = editorPane
		}
	default:
		moved = false
	}
	if moved {
		sidebar.selected = min(max(sidebar.selected, 0), count-1)
		sidebar.list.reveal(sidebar.selected)
		if !sidebar.stale {
			sidebar.previewSelected(application)
		}
	}
}

// previewSelected opens the selected file as the selection moves, as a preview.
func (sidebar *sidebarState) previewSelected(application *App) {
	row := sidebar.rows[sidebar.selected]
	if row.kind == fileRow {
		application.openFile(fileReference{row.project, row.path})
	}
}

func (sidebar *sidebarState) expandOrEnter() {
	row := sidebar.rows[sidebar.selected]
	if row.kind != projectRow && row.kind != folderRow {
		return
	}
	if !row.expanded {
		sidebar.setExpanded(row, true)
		return
	}
	if sidebar.selected+1 < len(sidebar.rows) {
		sidebar.selected++
	}
}

func (sidebar *sidebarState) collapseOrLeave() {
	row := sidebar.rows[sidebar.selected]
	if (row.kind == projectRow || row.kind == folderRow) && row.expanded {
		sidebar.setExpanded(row, false)
		return
	}
	for index := sidebar.selected - 1; index >= 0; index-- {
		if sidebar.rows[index].depth < row.depth && sidebar.rows[index].kind != noteRow {
			sidebar.selected = index
			return
		}
	}
}

// sidebarRowHeight leaves air around each name: a tree of hundreds of files is read for long.
const sidebarRowHeight float32 = 28

const (
	indentWidth   float32 = 14
	treeIconSize  float32 = 15
	sidebarMargin float32 = 10
)

// build adds the sidebar: a header naming the mode, and the rows.
func (sidebar *sidebarState) build(context *ui.Context, application *App) {
	if sidebar.stale {
		sidebar.rebuild(application)
	}
	theme := context.Theme
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: theme.Surface}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(40), Padding: ui.PadXY(headerPadding, 0), Gap: headerGap, AlignY: ui.Center}, func() {
			if sidebar.entered != "" {
				sidebar.buildEnteredHeader(context, application)
				return
			}
			title := "Files"
			if sidebar.mode == reviewMode {
				title = "Review"
			}
			titleStyle := ui.TextStyle{Weight: draw2d.Medium, Color: theme.InkSoft}
			context.Text(title, titleStyle)
			room := sidebar.headerRoom() - textWidth(context, title, titleStyle) - toolButtonWidth*2 - headerGap*3
			if sidebar.mode == reviewMode {
				viewed, total := application.reviewProgress()
				progressStyle := ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkQuiet}
				if total > 0 {
					if progress := fitText(context, fmt.Sprintf("%d of %d viewed", viewed, total), progressStyle, room); progress != "" {
						context.Text(progress, progressStyle)
					}
				}
			}
			context.Spacer()
			sidebar.buildChangedFilter(context)
			if toolButton(context, "Collapse all", "chevrons-down-up", false) {
				sidebar.collapseAll(application)
			}
		})
		sidebar.list.rowHeight = sidebarRowHeight
		sidebar.list.count = len(sidebar.rows)
		if !application.modalOpen() {
			onBar := sidebar.list.follow(context, application.keys, theme.ScrollStep)
			x, y := context.Pointer()
			if index := sidebar.list.rowAt(x, y); index >= 0 && !onBar && application.keys.clicked() {
				sidebar.selected = index
				application.focus = sidebarPane
				if sidebar.onEnter(index, x) {
					sidebar.enter(application, sidebar.rows[index].project)
				} else if sidebar.onCheck(index, x) {
					application.toggleViewed(fileReference{sidebar.rows[index].project, sidebar.rows[index].path}, false)
				} else {
					sidebar.activate(application, index)
				}
			}
		}
		context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			sidebar.list.place(area)
			sidebar.paint(context, application, drawing)
		})
	})
}

// buildEnteredHeader names the project the sidebar is in, with the way back to every project.
func (sidebar *sidebarState) buildEnteredHeader(context *ui.Context, application *App) {
	theme := context.Theme
	index := application.projectIndex(sidebar.entered)
	if index < 0 {
		return
	}
	project := application.projects[index]
	if toolButton(context, "Every project (Esc)", "arrow-left", false) {
		sidebar.leave(application)
		return
	}
	count, countStyle := "", ui.TextStyle{Size: theme.SmallTextSize, Weight: draw2d.Mono, Color: theme.Warning}
	if sidebar.mode == reviewMode && len(project.Changes) > 0 {
		viewed, total := application.reviewed.Progress(project)
		count, countStyle.Color = fmt.Sprintf("%d/%d", viewed, total), theme.InkQuiet
	} else if len(project.Changes) > 0 {
		count = fmt.Sprint(len(project.Changes))
	}
	// what is left for the name and the branch once the buttons and the count have their room
	room := sidebar.headerRoom() - toolButtonWidth - headerGap
	if count != "" {
		room -= textWidth(context, count, countStyle) + headerGap
	}
	if sidebar.mode == filesMode {
		room -= toolButtonWidth + headerGap
	}
	nameStyle := ui.TextStyle{Weight: draw2d.Medium}
	name := fitText(context, project.Name, nameStyle, room)
	context.Text(name, nameStyle)
	room -= textWidth(context, name, nameStyle) + headerGap
	branchStyle := ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkQuiet}
	if branch := fitText(context, project.Branch, branchStyle, room); len([]rune(branch)) > 3 {
		context.Text(branch, branchStyle)
	}
	context.Spacer()
	if count != "" {
		context.Text(count, countStyle)
	}
	sidebar.buildChangedFilter(context)
}

// The sidebar's header: its padding, the gap between its parts and an icon button's width.
const (
	headerPadding   float32 = 14
	headerGap       float32 = 8
	toolButtonWidth float32 = 30
)

// headerRoom is the width inside the sidebar's header, less a little for the pane's resize edge.
func (sidebar *sidebarState) headerRoom() float32 {
	return sidebar.width - 2*headerPadding - 4
}

// listsChangesOnly tells whether the tree lists only the changed files: always in the review mode,
// and in the files mode when asked.
func (sidebar *sidebarState) listsChangesOnly() bool {
	return sidebar.mode == reviewMode || sidebar.onlyChanged
}

// toggleChangedFilter lists only the changed files in the files mode, or every file again.
func (sidebar *sidebarState) toggleChangedFilter() {
	if sidebar.mode != filesMode {
		return
	}
	sidebar.onlyChanged = !sidebar.onlyChanged
	sidebar.stale = true
	sidebar.list.offset = 0
}

// buildChangedFilter is the files mode's button that lists only the changed files.
func (sidebar *sidebarState) buildChangedFilter(context *ui.Context) {
	if sidebar.mode != filesMode {
		return
	}
	if toolButton(context, "Only the changed files (g)", "file-diff", sidebar.onlyChanged) {
		sidebar.toggleChangedFilter()
	}
}

// enterButtonWidth is the room the button that enters a project takes at the right of its row,
// shown while the pointer is on the row.
const enterButtonWidth float32 = 26

// onEnter tells whether a click at x on a row fell on the button that enters its project.
func (sidebar *sidebarState) onEnter(index int, x float32) bool {
	right := sidebar.list.area.Right() - 4 - scrollBarWidth/2 - 4
	return sidebar.rows[index].kind == projectRow && x >= right-enterButtonWidth && x <= right
}

// onCheck tells whether a click at x on a row fell on its viewed mark, in the review mode.
func (sidebar *sidebarState) onCheck(index int, x float32) bool {
	row := sidebar.rows[index]
	if sidebar.mode != reviewMode || row.kind != fileRow {
		return false
	}
	left := sidebar.rowLeft(row)
	return x >= left-4 && x <= left+treeIconSize+4
}

// rowLeft is where a row's chevron (or the room for it) starts.
func (sidebar *sidebarState) rowLeft(row treeRow) float32 {
	return sidebar.list.area.X + sidebarMargin + float32(row.depth)*indentWidth
}

func (sidebar *sidebarState) collapseAll(application *App) {
	for index := range application.projects {
		sidebar.expanded[sidebar.expandedKey(index, "")] = false
	}
	sidebar.stale = true
}

func (sidebar *sidebarState) paint(context *ui.Context, application *App, drawing *draw2d.List) {
	theme := context.Theme
	look := application.look
	area := sidebar.list.area
	measurer := context.Measurer()
	size := theme.TextSize
	small := theme.SmallTextSize
	metrics := measurer.MeasureText("Ag", size, draw2d.Regular, 1)
	pointerX, pointerY := context.Pointer()
	hovered := -1
	if !application.modalOpen() {
		hovered = sidebar.list.rowAt(pointerX, pointerY)
	}
	drawing.PushClip(area)
	first, last := sidebar.list.visible()
	for index := first; index < last; index++ {
		row := sidebar.rows[index]
		top := sidebar.list.rowTop(index)
		rowArea := draw2d.Rectangle{X: area.X + 4, Y: top + 1, Width: area.Width - 8 - scrollBarWidth/2, Height: sidebarRowHeight - 2}
		reference := fileReference{row.project, row.path}
		isOpen := row.kind == fileRow && application.editor.isOpen() && application.editor.reference == reference
		switch {
		case index == sidebar.selected && application.focus == sidebarPane:
			drawing.Rectangle(rowArea, 5, look.chosen, look.focusEdge, 1)
		case index == sidebar.selected, isOpen:
			drawing.Rectangle(rowArea, 5, look.chosen.WithAlpha(0.7), draw2d.Color{}, 0)
		case index == hovered:
			drawing.Rectangle(rowArea, 5, look.hover, draw2d.Color{}, 0)
		}
		baseline := top + (sidebarRowHeight+metrics.Ascent-metrics.Descent)/2
		x := sidebar.rowLeft(row)
		iconTop := top + (sidebarRowHeight-treeIconSize)/2
		ink := theme.Ink
		if row.quiet {
			ink = theme.InkQuiet
		}
		weight := draw2d.Regular
		switch row.kind {
		case projectRow, folderRow:
			chevron := "chevron-right"
			if row.expanded {
				chevron = "chevron-down"
			}
			drawing.Icon(chevron, x, iconTop, treeIconSize, theme.InkQuiet)
			x += treeIconSize + 4
			icon := "folder"
			if row.kind == projectRow {
				icon, weight = "git-branch", draw2d.Medium
			} else if row.expanded {
				icon = "folder-open"
			}
			drawing.Icon(icon, x, iconTop, treeIconSize, theme.InkQuiet)
			x += treeIconSize + 7
		case fileRow:
			// in the review mode the room a folder gives its chevron holds the viewed mark
			if sidebar.mode == reviewMode {
				check, checkInk := "circle", theme.InkQuiet
				if application.isViewed(reference) {
					check, checkInk, ink = "circle-check", theme.InkSoft, theme.InkQuiet
				}
				drawing.Icon(check, x, iconTop, treeIconSize, checkInk)
			}
			x += treeIconSize + 4
			icon, iconInk := fileIcon(row.label, look)
			if ink == theme.InkQuiet {
				iconInk = iconInk.WithAlpha(0.5)
			}
			drawing.Icon(icon, x, iconTop, treeIconSize, iconInk)
			x += treeIconSize + 7
			// in the files mode the name says how the file changed, as Zed's project panel does;
			// in the review mode every file changed, and the colour is kept for its counts
			if row.status != git.Unchanged && sidebar.mode == filesMode {
				ink = statusColour(theme, row.status)
			}
		case noteRow:
			ink = theme.InkQuiet
		}
		right := rowArea.Right() - 8
		if row.kind == projectRow && index == hovered {
			button := draw2d.Rectangle{X: rowArea.Right() - 4 - enterButtonWidth, Y: top + 3, Width: enterButtonWidth, Height: sidebarRowHeight - 6}
			if sidebar.onEnter(index, pointerX) {
				drawing.Rectangle(button, 4, look.chosen, draw2d.Color{}, 0)
			}
			drawing.Icon("arrow-right-to-line", button.X+(enterButtonWidth-14)/2, top+(sidebarRowHeight-14)/2, 14, theme.InkSoft)
			right = button.X - 6
		}
		right = sidebar.paintRight(context, application, drawing, row, right, baseline, top)
		drawing.PushClip(draw2d.Rectangle{X: area.X, Y: top, Width: max(right-area.X-8, 0), Height: sidebarRowHeight})
		drawing.Text(row.label, x, baseline, size, weight, ink)
		if row.kind == projectRow {
			labelWidth := measurer.MeasureText(row.label, size, weight, 1).Width
			project := application.projects[row.project]
			if project.Branch != "" {
				drawing.Text(project.Branch, x+labelWidth+8, baseline, small, draw2d.Regular, theme.InkQuiet)
			}
		}
		drawing.PopClip()
	}
	if len(sidebar.rows) == 0 {
		message := "No projects open: press o to open a folder"
		if application.loading > 0 {
			message = "Reading the projects…"
		}
		drawing.Text(message, area.X+16, area.Y+24, size, draw2d.Regular, theme.InkQuiet)
	}
	sidebar.list.paintBar(drawing, look.scrollThumb)
	drawing.PopClip()
}

// paintRight paints what a row shows on its right, from right to left, and returns where it ends:
// a file's status letter and, in the review mode, the lines it adds and removes; a project's
// changed files, or how many of them were viewed.
func (sidebar *sidebarState) paintRight(context *ui.Context, application *App, drawing *draw2d.List, row treeRow, right, baseline, top float32) float32 {
	theme := context.Theme
	measurer := context.Measurer()
	small := theme.SmallTextSize
	text := func(words string, colour draw2d.Color) {
		width := measurer.MeasureText(words, small, draw2d.Mono, 1).Width
		right -= width
		drawing.Text(words, right, baseline, small, draw2d.Mono, colour)
		right -= 8
	}
	switch row.kind {
	case fileRow:
		if row.status == git.Unchanged {
			return right
		}
		text(row.status.Letter(), statusColour(theme, row.status))
		if sidebar.mode == reviewMode {
			project := application.projects[row.project]
			counts := project.Counts[row.path]
			if counts.Binary {
				text("bin", theme.InkQuiet)
				break
			}
			viewed := application.isViewed(fileReference{row.project, row.path})
			removedInk, addedInk := theme.Danger, theme.Positive
			if viewed {
				removedInk, addedInk = theme.InkQuiet, theme.InkQuiet
			}
			if counts.Removed > 0 {
				text(fmt.Sprintf("−%d", counts.Removed), removedInk)
			}
			if counts.Added > 0 {
				text(fmt.Sprintf("+%d", counts.Added), addedInk)
			}
		}
	case projectRow:
		project := application.projects[row.project]
		if !project.Loaded {
			text("…", theme.InkQuiet)
			break
		}
		if row.changed == 0 {
			break
		}
		if sidebar.mode == filesMode {
			text(fmt.Sprint(row.changed), theme.Warning)
			break
		}
		viewed, total := application.reviewed.Progress(project)
		colour := theme.InkQuiet
		if viewed == total {
			colour = theme.Positive
		}
		text(fmt.Sprintf("%d/%d", viewed, total), colour)
		bar := draw2d.Rectangle{X: right - 28, Y: top + sidebarRowHeight/2 - 1.5, Width: 28, Height: 3}
		drawing.Rectangle(bar, 1.5, application.look.hover.Mix(theme.Ink, 0.12), draw2d.Color{}, 0)
		if total > 0 && viewed > 0 {
			done := bar
			done.Width = bar.Width * float32(viewed) / float32(total)
			drawing.Rectangle(done, 1.5, colour, draw2d.Color{}, 0)
		}
		right -= 36
	case folderRow:
		if row.changed > 0 && sidebar.mode == filesMode {
			dot := draw2d.Rectangle{X: right - 6, Y: top + sidebarRowHeight/2 - 3, Width: 6, Height: 6}
			drawing.Rectangle(dot, 3, theme.Warning.WithAlpha(0.8), draw2d.Color{}, 0)
			right -= 14
		}
	}
	return right
}

// statusColour is a status's colour: green for new, amber for changed, red for deleted.
func statusColour(theme ui.Theme, status git.Status) draw2d.Color {
	switch status {
	case git.Added:
		return theme.Positive
	case git.Deleted:
		return theme.Danger
	case git.Unchanged:
		return theme.Ink
	}
	return theme.Warning
}
