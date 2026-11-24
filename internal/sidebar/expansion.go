package sidebar

import (
	"strings"

	"github.com/akrck02/code-review/internal/workspace"
)

// expansionKey is a project's or a folder's place in a mode's listing, where it was opened or
// closed by hand.
type expansionKey struct {
	listing     string
	changesOnly bool
	project     int
	path        string
}

// expandedKey is where a project or a folder is opened or closed by hand in the listing on now.
func (sidebar *Sidebar) expandedKey(project int, path string) expansionKey {
	return expansionKey{sidebar.listing.Name(), sidebar.listing.ChangesOnly(), project, path}
}

// isExpanded tells whether a project or a folder is open: as the filter, the hand or the default has it.
func (sidebar *Sidebar) isExpanded(window Window, project int, path string, isProject bool) bool {
	if sidebar.filtersFiles(window, project) {
		return true // the files found: all in sight
	}
	if open, chosen := sidebar.expanded[sidebar.expandedKey(project, path)]; chosen {
		return open
	}
	if sidebar.powers.chooser != nil || sidebar.listing.ChangesOnly() {
		return true // a few files, chosen: all in sight
	}
	return isProject && len(window.Projects()) == 1
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

// SetExpanded opens or closes a project or a folder.
func (sidebar *Sidebar) SetExpanded(row Row, open bool) {
	sidebar.expanded[sidebar.expandedKey(row.Project, row.Path)] = open
	sidebar.Stale = true
}

// expandTo opens a project and its folders around a path, a file's or a folder's, and gives the
// deepest folder row the path is in or is: a row of a chain of lone folders names the whole chain.
func (sidebar *Sidebar) expandTo(window Window, project int, path string) string {
	sidebar.expanded[sidebar.expandedKey(project, "")] = true
	folder, row := sidebar.tree(window, project), ""
	for strings.HasPrefix(path, row+"/") || row == "" {
		var inside *workspace.Folder
		for _, child := range folder.Folders {
			if child.Path == path || strings.HasPrefix(path, child.Path+"/") || strings.HasPrefix(child.Path, path+"/") {
				inside = child
				break
			}
		}
		if inside == nil {
			break
		}
		sidebar.expanded[sidebar.expandedKey(project, inside.Path)] = true
		folder, row = inside, inside.Path
	}
	return row
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

// setAllOpen opens or folds every project, or, inside a project, every folder of its tree.
func (sidebar *Sidebar) setAllOpen(window Window, open bool) {
	sidebar.Stale = true
	if project := window.ProjectIndex(sidebar.Entered); sidebar.Entered != "" && project >= 0 {
		sidebar.setFoldersOpen(project, sidebar.tree(window, project).Folders, open)
		return
	}
	for index := range window.Projects() {
		sidebar.expanded[sidebar.expandedKey(index, "")] = open
	}
}

// setFoldersOpen opens or folds folders and every folder inside them.
func (sidebar *Sidebar) setFoldersOpen(project int, folders []*workspace.Folder, open bool) {
	for _, folder := range folders {
		sidebar.expanded[sidebar.expandedKey(project, folder.Path)] = open
		sidebar.setFoldersOpen(project, folder.Folders, open)
	}
}
