// Package sidebar is the tree pane: the projects open, their folders and files, the one selected,
// and the keys that move through them.
package sidebar

import (
	"fmt"
	"strings"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// Mode is what the sidebar lists.
type Mode int

const (
	FilesMode  Mode = iota // every file, to read the code: the default
	ReviewMode             // only what the development changed, to review it file by file
)

type RowKind int

const (
	ProjectRow RowKind = iota
	FolderRow
	FileRow
	NoteRow // a line of explanation: projects hidden, a project still loading
)

// Window is what the sidebar needs from the window it sits in.
type Window interface {
	Look() theme.Look
	Keys() pane.Keys
	// ModalOpen tells whether a dialog or the finder is over the window, taking the input.
	ModalOpen() bool
	Projects() []*workspace.Project
	// ProjectIndex is a project's place in Projects, by its path; -1 when it is not open.
	ProjectIndex(path string) int
	// Hidden tells whether a project is left out of the list: one with no changes, while only the
	// projects with changes are shown.
	Hidden(project *workspace.Project) bool
	ShowEveryProject()
	OpenFile(file workspace.FileReference)
	// OpenedFile is the file the viewer shows, if any.
	OpenedFile() (workspace.FileReference, bool)
	IsOpened(file workspace.FileReference) bool
	FocusViewer()
	FocusSidebar()
	SidebarFocused() bool
	IsViewed(file workspace.FileReference) bool
	ToggleViewed(file workspace.FileReference, moveOn bool)
	// ReviewProgress counts the changed files viewed, and all of them, in the projects shown;
	// ProjectProgress counts them in one project.
	ReviewProgress() (viewed, total int)
	ProjectProgress(project *workspace.Project) (viewed, total int)
	// ClassKind is what a Java or Kotlin file declares, once read; asking for one not read yet
	// has it read in the background.
	ClassKind(file workspace.FileReference) java.ClassKind
	// Loading tells whether projects are being read, and ReadingKinds whether class kinds are.
	Loading() bool
	ReadingKinds() bool
}

// Row is a row of the sidebar.
type Row struct {
	Kind     RowKind
	Project  int
	Path     string // a folder's or a file's path in the project
	Label    string
	Depth    int
	Expanded bool
	Status   git.Status
	Changed  int // changed files inside a project or a folder
	Quiet    bool
	Role     workspace.FolderRole
}

type Sidebar struct {
	Mode Mode
	// expanded keeps the folders and projects opened or closed by hand, by key; the others follow
	// the mode's default (everything open in the review mode, closed in the files mode).
	expanded map[string]bool
	trees    map[treeKey]*workspace.Folder
	Rows     []Row
	Stale    bool
	Selected int
	list     pane.Scroller
	Width    float32
	// entered is the project the sidebar was taken into, by its path: then only its tree is
	// listed, from its top, as if it were the only project open. Empty lists every project.
	Entered string
	// onlyChanged lists, in the files mode, only the files the development changed, still opened
	// whole with their changes marked: a lighter way in than the review mode.
	onlyChanged bool
}

type treeKey struct {
	project int
	mode    Mode
	changed bool
}

func New() Sidebar {
	return Sidebar{expanded: map[string]bool{}, trees: map[treeKey]*workspace.Folder{}, Stale: true, Width: 380, list: pane.Scroller{RowHeight: RowHeight}}
}

func (sidebar *Sidebar) Invalidate() {
	sidebar.trees = map[treeKey]*workspace.Folder{}
	sidebar.Stale = true
}

func (sidebar *Sidebar) expandedKey(project int, path string) string {
	return fmt.Sprintf("%d|%t|%d|%s", sidebar.Mode, sidebar.listsChangesOnly(), project, path)
}

func (sidebar *Sidebar) isExpanded(window Window, project int, path string, isProject bool) bool {
	if open, chosen := sidebar.expanded[sidebar.expandedKey(project, path)]; chosen {
		return open
	}
	if sidebar.listsChangesOnly() {
		return true
	}
	return isProject && len(window.Projects()) == 1
}

func (sidebar *Sidebar) tree(window Window, project int) *workspace.Folder {
	key := treeKey{project, sidebar.Mode, sidebar.listsChangesOnly()}
	if tree, built := sidebar.trees[key]; built {
		return tree
	}
	model := window.Projects()[project]
	isChanged := func(path string) bool { _, changed := model.ChangeOf(path); return changed }
	paths := model.Files
	if sidebar.listsChangesOnly() {
		paths = model.ChangedPaths()
	}
	// a Gradle or Maven project is shown as IntelliJ shows it: modules, source roots, packages
	build := workspace.BuildTree
	if java.IsProject(model.Files) {
		build = java.BuildTree
	}
	tree := build(paths, isChanged)
	sidebar.trees[key] = tree
	return tree
}

// Rebuild lays the projects out as rows, keeping the selected row on the same thing.
func (sidebar *Sidebar) Rebuild(window Window) {
	var previous Row
	hadSelection := sidebar.Selected >= 0 && sidebar.Selected < len(sidebar.Rows)
	if hadSelection {
		previous = sidebar.Rows[sidebar.Selected]
	}
	sidebar.Rows = sidebar.Rows[:0]
	hidden := 0
	if sidebar.Entered != "" && window.ProjectIndex(sidebar.Entered) < 0 {
		sidebar.Entered = "" // the project was closed
	}
	for index, project := range window.Projects() {
		if sidebar.Entered != "" {
			if project.Path == sidebar.Entered {
				sidebar.addProjectContent(window, index, project, 0)
			}
			continue
		}
		if window.Hidden(project) {
			hidden++
			continue
		}
		expanded := sidebar.isExpanded(window, index, "", true)
		sidebar.Rows = append(sidebar.Rows, Row{
			Kind: ProjectRow, Project: index, Label: project.Name, Expanded: expanded, Changed: len(project.Changes),
			Quiet: project.Loaded && len(project.Changes) == 0,
		})
		if expanded {
			sidebar.addProjectContent(window, index, project, 1)
		}
	}
	if hidden > 0 {
		sidebar.Rows = append(sidebar.Rows, Row{Kind: NoteRow, Project: -1, Quiet: true,
			Label: fmt.Sprintf("%d projects without changes hidden (h shows them)", hidden)})
	}
	sidebar.Stale = false
	sidebar.list.Count = len(sidebar.Rows)
	if hadSelection {
		for index, row := range sidebar.Rows {
			if row.Kind == previous.Kind && row.Project == previous.Project && row.Path == previous.Path {
				sidebar.Selected = index
				return
			}
		}
	}
	sidebar.Selected = min(max(sidebar.Selected, 0), max(len(sidebar.Rows)-1, 0))
}

// addProjectContent lists a project's tree at a depth, or why it cannot be listed yet.
func (sidebar *Sidebar) addProjectContent(window Window, index int, project *workspace.Project, depth int) {
	if !project.Loaded {
		sidebar.Rows = append(sidebar.Rows, Row{Kind: NoteRow, Project: index, Label: "Reading…", Depth: depth, Quiet: true})
		return
	}
	if project.Error != "" {
		sidebar.Rows = append(sidebar.Rows, Row{Kind: NoteRow, Project: index, Label: project.Error, Depth: depth, Quiet: true})
	}
	sidebar.addFolder(window, index, sidebar.tree(window, index), depth)
}

// Enter takes the sidebar into a project, so only its tree is listed.
func (sidebar *Sidebar) Enter(window Window, project int) {
	if project < 0 || project >= len(window.Projects()) {
		return
	}
	sidebar.Entered = window.Projects()[project].Path
	sidebar.Selected = 0
	sidebar.list.Offset = 0
	sidebar.Rebuild(window)
	if file, open := window.OpenedFile(); open && file.Project == project {
		sidebar.RevealFile(window, file)
	}
}

// Leave goes back to every project, with the one left selected.
func (sidebar *Sidebar) Leave(window Window) {
	left := window.ProjectIndex(sidebar.Entered)
	sidebar.Entered = ""
	sidebar.Rebuild(window)
	for index, row := range sidebar.Rows {
		if row.Kind == ProjectRow && row.Project == left {
			sidebar.Selected = index
			sidebar.list.Reveal(index)
			return
		}
	}
}

func (sidebar *Sidebar) addFolder(window Window, project int, folder *workspace.Folder, depth int) {
	model := window.Projects()[project]
	for _, child := range folder.Folders {
		expanded := sidebar.isExpanded(window, project, child.Path, false)
		sidebar.Rows = append(sidebar.Rows, Row{
			Kind: FolderRow, Project: project, Path: child.Path, Label: child.Name, Depth: depth,
			Expanded: expanded, Changed: child.Changed, Role: child.Role,
		})
		if expanded {
			sidebar.addFolder(window, project, child, depth+1)
		}
	}
	for _, path := range folder.Files {
		change, _ := model.ChangeOf(path)
		sidebar.Rows = append(sidebar.Rows, Row{
			Kind: FileRow, Project: project, Path: path, Label: workspace.BaseName(path), Depth: depth, Status: change.Status,
		})
	}
}

// SetExpanded opens or closes a project or a folder.
func (sidebar *Sidebar) SetExpanded(row Row, open bool) {
	sidebar.expanded[sidebar.expandedKey(row.Project, row.Path)] = open
	sidebar.Stale = true
}

// RevealFile opens the folders around a file and selects it.
func (sidebar *Sidebar) RevealFile(window Window, reference workspace.FileReference) {
	project := window.Projects()[reference.Project]
	if sidebar.Entered != "" && sidebar.Entered != project.Path {
		sidebar.Entered = "" // a file of another project: back to every project to show it
	}
	_, changed := project.ChangeOf(reference.Path)
	if !changed {
		sidebar.onlyChanged = false
	}
	if sidebar.Mode == ReviewMode && !changed {
		sidebar.Mode = FilesMode
	}
	sidebar.expanded[sidebar.expandedKey(reference.Project, "")] = true
	folder := sidebar.tree(window, reference.Project)
	for {
		var inside *workspace.Folder
		for _, child := range folder.Folders {
			if strings.HasPrefix(reference.Path, child.Path+"/") {
				inside = child
				break
			}
		}
		if inside == nil {
			break
		}
		sidebar.expanded[sidebar.expandedKey(reference.Project, inside.Path)] = true
		folder = inside
	}
	sidebar.Rebuild(window)
	for index, row := range sidebar.Rows {
		if row.Kind == FileRow && row.Project == reference.Project && row.Path == reference.Path {
			sidebar.Selected = index
			sidebar.list.Reveal(index)
			return
		}
	}
}

// activate does what Enter or a click does on a row: open a file, open or close a folder.
func (sidebar *Sidebar) activate(window Window, index int) {
	if index < 0 || index >= len(sidebar.Rows) {
		return
	}
	row := sidebar.Rows[index]
	switch row.Kind {
	case FileRow:
		window.OpenFile(workspace.FileReference{Project: row.Project, Path: row.Path})
	case ProjectRow, FolderRow:
		sidebar.SetExpanded(row, !row.Expanded)
	case NoteRow:
		if row.Project < 0 {
			window.ShowEveryProject()
		}
	}
}

// MoveSelection moves the selected row by step rows; a pane.PageStep is a page, a pane.EndStep the far end.
func (sidebar *Sidebar) MoveSelection(window Window, step int) {
	switch {
	case step >= pane.EndStep:
		sidebar.Selected = len(sidebar.Rows) - 1
	case step <= -pane.EndStep:
		sidebar.Selected = 0
	case step >= pane.PageStep:
		sidebar.Selected += sidebar.list.PageRows()
	case step <= -pane.PageStep:
		sidebar.Selected -= sidebar.list.PageRows()
	default:
		sidebar.Selected += step
	}
	sidebar.selectionMoved(window)
}

// OpenOrClose opens the selected folder, or goes into it, for a step forward; closes it, or goes
// to its parent, for a step back.
func (sidebar *Sidebar) OpenOrClose(window Window, step int) {
	if step > 0 {
		sidebar.expandOrEnter()
	} else {
		sidebar.collapseOrLeave()
	}
	sidebar.selectionMoved(window)
}

// OpenSelected opens the selected row: enters a project, opens a folder, or shows a file, moving
// the keyboard to it when asked.
func (sidebar *Sidebar) OpenSelected(window Window, moveToFile bool) {
	if sidebar.Rows[sidebar.Selected].Kind == ProjectRow && moveToFile {
		sidebar.Enter(window, sidebar.Rows[sidebar.Selected].Project)
		return
	}
	sidebar.activate(window, sidebar.Selected)
	if sidebar.Selected < len(sidebar.Rows) && sidebar.Rows[sidebar.Selected].Kind == FileRow && moveToFile {
		window.FocusViewer()
	}
	sidebar.selectionMoved(window)
}

// selectionMoved keeps the selection on a row, in sight, and shows the file it is on.
func (sidebar *Sidebar) selectionMoved(window Window) {
	sidebar.Selected = min(max(sidebar.Selected, 0), len(sidebar.Rows)-1)
	sidebar.list.Reveal(sidebar.Selected)
	if !sidebar.Stale {
		sidebar.previewSelected(window)
	}
}

// previewSelected opens the selected file as the selection moves, as a preview.
func (sidebar *Sidebar) previewSelected(window Window) {
	row := sidebar.Rows[sidebar.Selected]
	if row.Kind == FileRow {
		window.OpenFile(workspace.FileReference{Project: row.Project, Path: row.Path})
	}
}

func (sidebar *Sidebar) expandOrEnter() {
	row := sidebar.Rows[sidebar.Selected]
	if row.Kind != ProjectRow && row.Kind != FolderRow {
		return
	}
	if !row.Expanded {
		sidebar.SetExpanded(row, true)
		return
	}
	if sidebar.Selected+1 < len(sidebar.Rows) {
		sidebar.Selected++
	}
}

func (sidebar *Sidebar) collapseOrLeave() {
	row := sidebar.Rows[sidebar.Selected]
	if (row.Kind == ProjectRow || row.Kind == FolderRow) && row.Expanded {
		sidebar.SetExpanded(row, false)
		return
	}
	for index := sidebar.Selected - 1; index >= 0; index-- {
		if sidebar.Rows[index].Depth < row.Depth && sidebar.Rows[index].Kind != NoteRow {
			sidebar.Selected = index
			return
		}
	}
}

// RowHeight leaves air around each name: a tree of hundreds of files is read for long.
const RowHeight float32 = 28

const (
	indentWidth   float32 = 14
	treeIconSize  float32 = 15
	sidebarMargin float32 = 10
)

// Build adds the sidebar: a header naming the mode, and the rows.
func (sidebar *Sidebar) Build(context *ui.Context, window Window) {
	if sidebar.Stale {
		sidebar.Rebuild(window)
	}
	palette := context.Theme
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: palette.Surface}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(40), Padding: ui.PadXY(headerPadding, 0), Gap: headerGap, AlignY: ui.Center}, func() {
			if sidebar.Entered != "" {
				sidebar.buildEnteredHeader(context, window)
				return
			}
			title := "Files"
			if sidebar.Mode == ReviewMode {
				title = "Review"
			}
			titleStyle := ui.TextStyle{Weight: draw2d.Medium, Color: palette.InkSoft}
			context.Text(title, titleStyle)
			room := sidebar.headerRoom() - theme.TextWidth(context, title, titleStyle) - toolButtonWidth*2 - headerGap*3
			if sidebar.Mode == ReviewMode {
				viewed, total := window.ReviewProgress()
				progressStyle := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet}
				if total > 0 {
					if progress := theme.FitText(context, fmt.Sprintf("%d of %d viewed", viewed, total), progressStyle, room); progress != "" {
						context.Text(progress, progressStyle)
					}
				}
			}
			context.Spacer()
			sidebar.buildChangedFilter(context)
			if theme.ToolButton(context, "Collapse all", "chevrons-down-up", false) {
				sidebar.collapseAll(window)
			}
		})
		sidebar.list.RowHeight = RowHeight
		sidebar.list.Count = len(sidebar.Rows)
		if !window.ModalOpen() {
			onBar := sidebar.list.Follow(context, window.Keys(), palette.ScrollStep)
			x, y := context.Pointer()
			if index := sidebar.list.RowAt(x, y); index >= 0 && !onBar && window.Keys().Clicked() {
				sidebar.Selected = index
				window.FocusSidebar()
				if sidebar.onEnter(index, x) {
					sidebar.Enter(window, sidebar.Rows[index].Project)
				} else if sidebar.onCheck(index, x) {
					window.ToggleViewed(workspace.FileReference{Project: sidebar.Rows[index].Project, Path: sidebar.Rows[index].Path}, false)
				} else {
					sidebar.activate(window, index)
				}
			}
		}
		context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			sidebar.list.Place(area)
			sidebar.paint(context, window, drawing)
		})
	})
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
	count, countStyle := "", ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Mono, Color: palette.Warning}
	if sidebar.Mode == ReviewMode && len(project.Changes) > 0 {
		viewed, total := window.ProjectProgress(project)
		count, countStyle.Color = fmt.Sprintf("%d/%d", viewed, total), palette.InkQuiet
	} else if len(project.Changes) > 0 {
		count = fmt.Sprint(len(project.Changes))
	}
	// what is left for the name and the branch once the buttons and the count have their room
	room := sidebar.headerRoom() - toolButtonWidth - headerGap
	if count != "" {
		room -= theme.TextWidth(context, count, countStyle) + headerGap
	}
	if sidebar.Mode == FilesMode {
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
	sidebar.buildChangedFilter(context)
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

// listsChangesOnly tells whether the tree lists only the changed files: always in the review mode,
// and in the files mode when asked.
func (sidebar *Sidebar) listsChangesOnly() bool {
	return sidebar.Mode == ReviewMode || sidebar.onlyChanged
}

// ToggleChangedFilter lists only the changed files in the files mode, or every file again.
func (sidebar *Sidebar) ToggleChangedFilter() {
	if sidebar.Mode != FilesMode {
		return
	}
	sidebar.onlyChanged = !sidebar.onlyChanged
	sidebar.Stale = true
	sidebar.list.Offset = 0
}

// buildChangedFilter is the files mode's button that lists only the changed files.
func (sidebar *Sidebar) buildChangedFilter(context *ui.Context) {
	if sidebar.Mode != FilesMode {
		return
	}
	if theme.ToolButton(context, "Only the changed files (g)", "file-diff", sidebar.onlyChanged) {
		sidebar.ToggleChangedFilter()
	}
}

// enterButtonWidth is the room the button that enters a project takes at the right of its row,
// shown while the pointer is on the row.
const enterButtonWidth float32 = 26

// onEnter tells whether a click at x on a row fell on the button that enters its project.
func (sidebar *Sidebar) onEnter(index int, x float32) bool {
	right := sidebar.list.Area.Right() - 4 - pane.ScrollBarWidth/2 - 4
	return sidebar.Rows[index].Kind == ProjectRow && x >= right-enterButtonWidth && x <= right
}

// onCheck tells whether a click at x on a row fell on its viewed mark, in the review mode.
func (sidebar *Sidebar) onCheck(index int, x float32) bool {
	row := sidebar.Rows[index]
	if sidebar.Mode != ReviewMode || row.Kind != FileRow {
		return false
	}
	left := sidebar.rowLeft(row)
	return x >= left-4 && x <= left+treeIconSize+4
}

// rowLeft is where a row's chevron (or the room for it) starts.
func (sidebar *Sidebar) rowLeft(row Row) float32 {
	return sidebar.list.Area.X + sidebarMargin + float32(row.Depth)*indentWidth
}

func (sidebar *Sidebar) collapseAll(window Window) {
	for index := range window.Projects() {
		sidebar.expanded[sidebar.expandedKey(index, "")] = false
	}
	sidebar.Stale = true
}

func (sidebar *Sidebar) paint(context *ui.Context, window Window, drawing *draw2d.List) {
	palette := context.Theme
	look := window.Look()
	area := sidebar.list.Area
	measurer := context.Measurer()
	size := palette.TextSize
	small := palette.SmallTextSize
	metrics := measurer.MeasureText("Ag", size, draw2d.Regular, 1)
	pointerX, pointerY := context.Pointer()
	hovered := -1
	if !window.ModalOpen() {
		hovered = sidebar.list.RowAt(pointerX, pointerY)
	}
	drawing.PushClip(area)
	first, last := sidebar.list.Visible()
	for index := first; index < last; index++ {
		row := sidebar.Rows[index]
		top := sidebar.list.RowTop(index)
		rowArea := draw2d.Rectangle{X: area.X + 4, Y: top + 1, Width: area.Width - 8 - pane.ScrollBarWidth/2, Height: RowHeight - 2}
		reference := workspace.FileReference{Project: row.Project, Path: row.Path}
		isOpen := row.Kind == FileRow && window.IsOpened(reference)
		switch {
		case index == sidebar.Selected && window.SidebarFocused():
			drawing.Rectangle(rowArea, 5, look.Chosen, look.FocusEdge, 1)
		case index == sidebar.Selected, isOpen:
			// the selection of a pane without the keyboard, quieter than the one with it
			drawing.Rectangle(rowArea, 5, look.Hover.Mix(look.Chosen, 0.5), draw2d.Color{}, 0)
		case index == hovered:
			drawing.Rectangle(rowArea, 5, look.Hover, draw2d.Color{}, 0)
		}
		baseline := top + (RowHeight+metrics.Ascent-metrics.Descent)/2
		x := sidebar.rowLeft(row)
		iconTop := top + (RowHeight-treeIconSize)/2
		ink := palette.Ink
		if row.Quiet {
			ink = palette.InkQuiet
		}
		weight := draw2d.Regular
		label := row.Label
		switch row.Kind {
		case ProjectRow, FolderRow:
			chevron := "chevron-right"
			if row.Expanded {
				chevron = "chevron-down"
			}
			drawing.Icon(chevron, x, iconTop, treeIconSize, palette.InkQuiet)
			x += treeIconSize + 4
			icon, iconInk := folderIcon(row, look)
			if row.Kind == ProjectRow {
				icon, iconInk, weight = "git-branch", palette.InkQuiet, draw2d.Medium
			}
			if row.Role == workspace.ModuleFolder {
				weight = draw2d.Medium
			}
			drawing.Icon(icon, x, iconTop, treeIconSize, iconInk)
			x += treeIconSize + 7
		case FileRow:
			// in the review mode the room a folder gives its chevron holds the viewed mark
			if sidebar.Mode == ReviewMode {
				check, checkInk := "circle", palette.InkQuiet
				if window.IsViewed(reference) {
					check, checkInk, ink = "circle-check", palette.InkSoft, palette.InkQuiet
				}
				drawing.Icon(check, x, iconTop, treeIconSize, checkInk)
			}
			x += treeIconSize + 4
			// a Java or Kotlin class is named without its extension and drawn by its kind, as
			// IntelliJ draws it, once the file has been read
			if kind := window.ClassKind(reference); kind != java.UnknownKind && kind != java.NoClass {
				paintClassBadge(drawing, look, kind, x, iconTop, treeIconSize, ink == palette.InkQuiet)
				label = strings.TrimSuffix(strings.TrimSuffix(row.Label, ".java"), ".kt")
			} else {
				icon, iconInk := theme.FileIcon(row.Label, look)
				if ink == palette.InkQuiet {
					iconInk = iconInk.WithAlpha(0.5)
				}
				drawing.Icon(icon, x, iconTop, treeIconSize, iconInk)
			}
			x += treeIconSize + 7
			// in the files mode the name says how the file changed, as Zed's project panel does;
			// in the review mode every file changed, and the colour is kept for its counts
			if row.Status != git.Unchanged && sidebar.Mode == FilesMode {
				ink = theme.StatusColour(palette, row.Status)
			}
		case NoteRow:
			ink = palette.InkQuiet
		}
		right := rowArea.Right() - 8
		if row.Kind == ProjectRow && index == hovered {
			button := draw2d.Rectangle{X: rowArea.Right() - 4 - enterButtonWidth, Y: top + 3, Width: enterButtonWidth, Height: RowHeight - 6}
			if sidebar.onEnter(index, pointerX) {
				drawing.Rectangle(button, 4, look.Chosen, draw2d.Color{}, 0)
			}
			drawing.Icon("arrow-right-to-line", button.X+(enterButtonWidth-14)/2, top+(RowHeight-14)/2, 14, palette.InkSoft)
			right = button.X - 6
		}
		right = sidebar.paintRight(context, window, drawing, row, right, baseline, top)
		drawing.PushClip(draw2d.Rectangle{X: area.X, Y: top, Width: max(right-area.X-8, 0), Height: RowHeight})
		drawing.Text(label, x, baseline, size, weight, ink)
		if row.Kind == ProjectRow {
			labelWidth := measurer.MeasureText(row.Label, size, weight, 1).Width
			project := window.Projects()[row.Project]
			if project.Branch != "" {
				drawing.Text(project.Branch, x+labelWidth+8, baseline, small, draw2d.Regular, palette.InkQuiet)
			}
		}
		drawing.PopClip()
	}
	if len(sidebar.Rows) == 0 {
		message := "No projects open: press o to open a folder"
		if window.Loading() {
			message = "Reading the projects…"
		}
		drawing.Text(message, area.X+16, area.Y+24, size, draw2d.Regular, palette.InkQuiet)
	}
	sidebar.list.PaintBar(drawing, look.ScrollThumb)
	drawing.PopClip()
	if window.ReadingKinds() {
		context.RequestFrame(0) // the next frame starts reading the classes that came into sight
	}
}

// paintRight paints what a row shows on its right, from right to left, and returns where it ends:
// a file's status letter and, in the review mode, the lines it adds and removes; a project's
// changed files, or how many of them were viewed.
func (sidebar *Sidebar) paintRight(context *ui.Context, window Window, drawing *draw2d.List, row Row, right, baseline, top float32) float32 {
	palette := context.Theme
	measurer := context.Measurer()
	small := palette.SmallTextSize
	text := func(words string, colour draw2d.Color) {
		width := measurer.MeasureText(words, small, draw2d.Mono, 1).Width
		right -= width
		drawing.Text(words, right, baseline, small, draw2d.Mono, colour)
		right -= 8
	}
	switch row.Kind {
	case FileRow:
		if row.Status == git.Unchanged {
			return right
		}
		text(row.Status.Letter(), theme.StatusColour(palette, row.Status))
		if sidebar.Mode == ReviewMode {
			project := window.Projects()[row.Project]
			counts := project.Counts[row.Path]
			if counts.Binary {
				text("bin", palette.InkQuiet)
				break
			}
			viewed := window.IsViewed(workspace.FileReference{Project: row.Project, Path: row.Path})
			removedInk, addedInk := palette.Danger, palette.Positive
			if viewed {
				removedInk, addedInk = palette.InkQuiet, palette.InkQuiet
			}
			if counts.Removed > 0 {
				text(fmt.Sprintf("−%d", counts.Removed), removedInk)
			}
			if counts.Added > 0 {
				text(fmt.Sprintf("+%d", counts.Added), addedInk)
			}
		}
	case ProjectRow:
		project := window.Projects()[row.Project]
		if !project.Loaded {
			text("…", palette.InkQuiet)
			break
		}
		if row.Changed == 0 {
			break
		}
		if sidebar.Mode == FilesMode {
			text(fmt.Sprint(row.Changed), palette.Warning)
			break
		}
		viewed, total := window.ProjectProgress(project)
		colour := palette.InkQuiet
		if viewed == total {
			colour = palette.Positive
		}
		text(fmt.Sprintf("%d/%d", viewed, total), colour)
		bar := draw2d.Rectangle{X: right - 28, Y: top + RowHeight/2 - 1.5, Width: 28, Height: 3}
		drawing.Rectangle(bar, 1.5, window.Look().Hover.Mix(palette.Ink, 0.12), draw2d.Color{}, 0)
		if total > 0 && viewed > 0 {
			done := bar
			done.Width = bar.Width * float32(viewed) / float32(total)
			drawing.Rectangle(done, 1.5, colour, draw2d.Color{}, 0)
		}
		right -= 36
	case FolderRow:
		if row.Changed > 0 && sidebar.Mode == FilesMode {
			dot := draw2d.Rectangle{X: right - 6, Y: top + RowHeight/2 - 3, Width: 6, Height: 6}
			drawing.Rectangle(dot, 3, palette.Warning.WithAlpha(0.8), draw2d.Color{}, 0)
			right -= 14
		}
	}
	return right
}

// SelectedRow is the row selected, if any.
func (sidebar *Sidebar) SelectedRow() (Row, bool) {
	if sidebar.Selected < 0 || sidebar.Selected >= len(sidebar.Rows) {
		return Row{}, false
	}
	return sidebar.Rows[sidebar.Selected], true
}
