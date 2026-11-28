package sidebar

import (
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/pane"
	ui "github.com/akrck02/go-ui"
)

// RevealFile opens the folders around a file and selects it.
func (sidebar *Sidebar) RevealFile(window Window, reference codebase.FileReference) {
	project := window.Projects()[reference.Project]
	if sidebar.Entered != "" && sidebar.Entered != project.Path {
		sidebar.Entered = "" // a file of another project: back to every project to show it
	}
	_, changed := project.ChangeOf(reference.Path)
	if !changed && sidebar.onlyChanged {
		window.ShowEveryProject()
	}
	if sidebar.listing.ChangesOnly() && !changed {
		window.ListEveryFile()
	}
	sidebar.expandTo(window, reference.Project, reference.Path)
	sidebar.Rebuild(window)
	sidebar.selectRow(FileRow, reference.Project, reference.Path)
}

// RevealFolder opens a project's folders down to one and selects it: the row of a chain of lone
// folders it is in, if it is one of the chain's, or the project's row for "".
func (sidebar *Sidebar) RevealFolder(window Window, project int, folder string) {
	if project < 0 || project >= len(window.Projects()) {
		return
	}
	if sidebar.Entered != "" && sidebar.Entered != window.Projects()[project].Path {
		sidebar.Entered = ""
	}
	row := sidebar.expandTo(window, project, folder)
	kind := FolderRow
	if row == "" {
		kind = ProjectRow
	}
	sidebar.Rebuild(window)
	if sidebar.selectRow(kind, project, row) {
		return
	}
	if sidebar.onlyChanged { // a folder with no change, hidden by the filters
		window.ShowEveryProject()
	}
	if sidebar.listing.ChangesOnly() {
		window.ListEveryFile()
	}
	sidebar.Rebuild(window)
	sidebar.selectRow(kind, project, row)
}

// selectRow selects the row of a kind, project and path, a project's whatever its path, and
// brings it into sight; it tells whether there was one.
func (sidebar *Sidebar) selectRow(kind RowKind, project int, path string) bool {
	for index, row := range sidebar.Rows {
		if row.Kind == kind && row.Project == project && (kind == ProjectRow || row.Path == path) {
			sidebar.Selected = index
			sidebar.list.Reveal(index)
			return true
		}
	}
	return false
}

// activate does what Enter or a click does on a row: open a file, open or close a folder and show
// what it holds.
func (sidebar *Sidebar) activate(window Window, index int) {
	if index < 0 || index >= len(sidebar.Rows) {
		return
	}
	row := sidebar.Rows[index]
	switch row.Kind {
	case FileRow:
		window.OpenFile(codebase.FileReference{Project: row.Project, Path: row.Path})
	case ProjectRow, FolderRow:
		sidebar.SetExpanded(row, !row.Expanded)
		window.ExploreFolder(row.Project, folderPath(row))
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

// previewSelected shows the selected file or folder as the selection moves, as a preview.
func (sidebar *Sidebar) previewSelected(window Window) {
	row := sidebar.Rows[sidebar.Selected]
	switch row.Kind {
	case FileRow:
		window.OpenFile(codebase.FileReference{Project: row.Project, Path: row.Path})
	case ProjectRow, FolderRow:
		window.ExploreFolder(row.Project, folderPath(row))
	}
}

// folderPath is the path of a project's or a folder's row in its project, "" for a project.
func folderPath(row Row) string {
	if row.Kind == ProjectRow {
		return ""
	}
	return row.Path
}

// followPointer scrolls the rows with the wheel and the bar, and answers a click on a row: on its
// enter button, on its check, or anywhere else on it.
func (sidebar *Sidebar) followPointer(context *ui.Context, window Window) {
	onBar := sidebar.list.Follow(context, window.Keys(), context.Theme.ScrollStep)
	x, y := context.Pointer()
	index := sidebar.list.RowAt(x, y)
	if index < 0 || onBar || !window.Keys().Clicked() {
		return
	}
	sidebar.Selected = index
	window.FocusSidebar()
	row := sidebar.Rows[index]
	switch {
	case sidebar.onBadge(index, x):
		if shower, shows := window.(ServerFiles); shows {
			shower.ShowServerFiles(row.Project)
		}
	case sidebar.onLeftOutMark(index, x):
		if outer, leavesOut := window.(LeftOuter); leavesOut {
			outer.ShowServers()
		}
	case sidebar.onEnter(index, x):
		sidebar.Enter(window, row.Project)
	case sidebar.onCheck(index, x):
		if presser := sidebar.powers.presser; presser != nil {
			presser.Press(codebase.FileReference{Project: row.Project, Path: row.Path}, false)
		}
	default:
		sidebar.activate(window, index)
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

// onCheck tells whether a click at x on a row fell on its mark, when the mode marks files.
func (sidebar *Sidebar) onCheck(index int, x float32) bool {
	row := sidebar.Rows[index]
	if row.Kind != FileRow || sidebar.listing.Decorate(row).Mark == NoMark {
		return false
	}
	left := sidebar.rowLeft(row)
	return x >= left-4 && x <= left+treeIconSize+4
}

// rowLeft is where a row's chevron (or the room for it) starts.
func (sidebar *Sidebar) rowLeft(row Row) float32 {
	return sidebar.list.Area.X + sidebarMargin + float32(row.Depth)*indentWidth
}

// SelectedRow is the row selected, if any.
func (sidebar *Sidebar) SelectedRow() (Row, bool) {
	if sidebar.Selected < 0 || sidebar.Selected >= len(sidebar.Rows) {
		return Row{}, false
	}
	return sidebar.Rows[sidebar.Selected], true
}

// SelectFirstFile selects the first file listed, if any, and tells whether there was one.
func (sidebar *Sidebar) SelectFirstFile() bool {
	for index, row := range sidebar.Rows {
		if row.Kind == FileRow {
			sidebar.Selected = index
			sidebar.list.Reveal(index)
			return true
		}
	}
	return false
}
