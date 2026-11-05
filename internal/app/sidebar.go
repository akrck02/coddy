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
	changesMode sidebarMode = iota // only what the development changed
	filesMode                      // every file
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
	// the mode's default (everything open in the changes mode, closed in the files mode).
	expanded map[string]bool
	trees    map[treeKey]*review.Folder
	rows     []treeRow
	stale    bool
	selected int
	list     scroller
	width    float32
}

type treeKey struct {
	project int
	mode    sidebarMode
}

func newSidebar() sidebarState {
	return sidebarState{expanded: map[string]bool{}, trees: map[treeKey]*review.Folder{}, stale: true, width: 320, list: scroller{rowHeight: sidebarRowHeight}}
}

func (sidebar *sidebarState) invalidate() {
	sidebar.trees = map[treeKey]*review.Folder{}
	sidebar.stale = true
}

func (sidebar *sidebarState) expandedKey(project int, path string) string {
	return fmt.Sprintf("%d|%d|%s", sidebar.mode, project, path)
}

func (sidebar *sidebarState) isExpanded(application *App, project int, path string, isProject bool) bool {
	if open, chosen := sidebar.expanded[sidebar.expandedKey(project, path)]; chosen {
		return open
	}
	if sidebar.mode == changesMode {
		return true
	}
	return isProject && len(application.projects) == 1
}

func (sidebar *sidebarState) tree(application *App, project int) *review.Folder {
	key := treeKey{project, sidebar.mode}
	if tree, built := sidebar.trees[key]; built {
		return tree
	}
	model := application.projects[project]
	isChanged := func(path string) bool { _, changed := model.ChangeOf(path); return changed }
	var tree *review.Folder
	if sidebar.mode == changesMode {
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
	for index, project := range application.projects {
		if application.hidden(project) {
			hidden++
			continue
		}
		expanded := sidebar.isExpanded(application, index, "", true)
		sidebar.rows = append(sidebar.rows, treeRow{
			kind: projectRow, project: index, label: project.Name, expanded: expanded, changed: len(project.Changes),
			quiet: project.Loaded && len(project.Changes) == 0,
		})
		if !expanded {
			continue
		}
		if !project.Loaded {
			sidebar.rows = append(sidebar.rows, treeRow{kind: noteRow, project: index, label: "Reading…", depth: 1, quiet: true})
			continue
		}
		if project.Error != "" {
			sidebar.rows = append(sidebar.rows, treeRow{kind: noteRow, project: index, label: project.Error, depth: 1, quiet: true})
		}
		sidebar.addFolder(application, index, sidebar.tree(application, index), 1)
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
	_, changed := project.ChangeOf(reference.path)
	if sidebar.mode == changesMode && !changed {
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

const sidebarRowHeight float32 = 24

// build adds the sidebar: a header with the mode, and the rows.
func (sidebar *sidebarState) build(context *ui.Context, application *App) {
	if sidebar.stale {
		sidebar.rebuild(application)
	}
	theme := context.Theme
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: theme.Surface}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(8, 6), Gap: 6, AlignY: ui.Center}, func() {
			mode := int(sidebar.mode)
			if context.Tabs("sidebar mode", &mode, "Changes", "Files") {
				sidebar.mode = sidebarMode(mode)
				sidebar.stale = true
			}
			context.Spacer()
			if context.FlatIconButton("chevrons-down-up", "") {
				sidebar.collapseAll(application)
			}
			context.Tooltip("Collapse all")
		})
		sidebar.list.rowHeight = sidebarRowHeight
		sidebar.list.count = len(sidebar.rows)
		if !application.modalOpen() {
			onBar := sidebar.list.follow(context, application.keys, theme.ScrollStep)
			x, y := context.Pointer()
			if index := sidebar.list.rowAt(x, y); index >= 0 && !onBar && application.keys.clicked() {
				sidebar.selected = index
				application.focus = sidebarPane
				sidebar.activate(application, index)
			}
		}
		context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			sidebar.list.place(area)
			sidebar.paint(context, application, drawing)
		})
	})
}

func (sidebar *sidebarState) collapseAll(application *App) {
	for index := range application.projects {
		sidebar.expanded[sidebar.expandedKey(index, "")] = false
	}
	sidebar.stale = true
}

func (sidebar *sidebarState) paint(context *ui.Context, application *App, drawing *draw2d.List) {
	theme := context.Theme
	area := sidebar.list.area
	measurer := context.Measurer()
	size := theme.TextSize
	metrics := measurer.MeasureText("Ag", size, draw2d.Regular, 1)
	drawing.PushClip(area)
	first, last := sidebar.list.visible()
	for index := first; index < last; index++ {
		row := sidebar.rows[index]
		top := sidebar.list.rowTop(index)
		rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: sidebarRowHeight}
		reference := fileReference{row.project, row.path}
		switch {
		case index == sidebar.selected && application.focus == sidebarPane:
			drawing.Rectangle(rowArea, 0, theme.PrimaryTint, theme.Primary.WithAlpha(0.6), 1)
		case index == sidebar.selected:
			drawing.Rectangle(rowArea, 0, theme.Sunken, draw2d.Color{}, 0)
		case row.kind == fileRow && application.editor.isOpen() && application.editor.reference == reference:
			drawing.Rectangle(rowArea, 0, theme.Sunken.WithAlpha(0.6), draw2d.Color{}, 0)
		}
		baseline := top + (sidebarRowHeight+metrics.Ascent-metrics.Descent)/2
		x := area.X + 8 + float32(row.depth)*14
		iconSize := float32(14)
		iconTop := top + (sidebarRowHeight-iconSize)/2
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
			drawing.Icon(chevron, x, iconTop, iconSize, theme.InkQuiet)
			x += iconSize + 4
			icon := "folder"
			if row.kind == projectRow {
				icon, weight = "git-branch", draw2d.Medium
			} else if row.expanded {
				icon = "folder-open"
			}
			drawing.Icon(icon, x, iconTop, iconSize, theme.InkSoft)
			x += iconSize + 6
		case fileRow:
			x += iconSize + 4
			drawing.Icon("file", x, iconTop, iconSize, theme.InkSoft)
			x += iconSize + 6
			if row.status != git.Unchanged {
				ink = statusColour(theme, row.status)
			}
		case noteRow:
			ink = theme.InkQuiet
		}
		badge, badgeColour := sidebar.badge(application, row, theme)
		badgeWidth := float32(0)
		if badge != "" {
			badgeWidth = measurer.MeasureText(badge, theme.SmallTextSize, draw2d.Mono, 1).Width + 12
			drawing.Text(badge, rowArea.Right()-badgeWidth-scrollBarWidth+4, baseline, theme.SmallTextSize, draw2d.Mono, badgeColour)
		}
		drawing.PushClip(draw2d.Rectangle{X: area.X, Y: top, Width: max(area.Width-badgeWidth-scrollBarWidth-4, 0), Height: sidebarRowHeight})
		drawing.Text(row.label, x, baseline, size, weight, ink)
		if row.kind == projectRow {
			labelWidth := measurer.MeasureText(row.label, size, weight, 1).Width
			project := application.projects[row.project]
			if project.Branch != "" {
				drawing.Text(project.Branch, x+labelWidth+8, baseline, theme.SmallTextSize, draw2d.Regular, theme.InkQuiet)
			}
		}
		drawing.PopClip()
	}
	if len(sidebar.rows) == 0 {
		message := "No projects open: press o to open a folder"
		if application.loading > 0 {
			message = "Reading the projects…"
		}
		drawing.Text(message, area.X+12, area.Y+24, size, draw2d.Regular, theme.InkQuiet)
	}
	sidebar.list.paintBar(drawing, theme.InkQuiet.WithAlpha(0.4))
	drawing.PopClip()
}

// badge is what a row shows on its right: a file's status, a folder's or project's count of
// changed files.
func (sidebar *sidebarState) badge(application *App, row treeRow, theme ui.Theme) (string, draw2d.Color) {
	switch row.kind {
	case fileRow:
		return row.status.Letter(), statusColour(theme, row.status)
	case projectRow:
		project := application.projects[row.project]
		if !project.Loaded {
			return "…", theme.InkQuiet
		}
		if row.changed > 0 {
			return fmt.Sprint(row.changed), theme.Warning
		}
	case folderRow:
		if row.changed > 0 && sidebar.mode == filesMode {
			return "•", theme.Warning
		}
	}
	return "", draw2d.Color{}
}

// statusColour is a status's colour: green for new, amber for changed, red for deleted.
func statusColour(theme ui.Theme, status git.Status) draw2d.Color {
	switch status {
	case git.Added:
		return theme.Positive
	case git.Deleted:
		return theme.Danger
	case git.Renamed, git.Copied:
		return theme.Primary
	case git.Unchanged:
		return theme.Ink
	}
	return theme.Warning
}
