package sidebar

import (
	"fmt"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/workspace"
)

// RowKind is what a row of the sidebar stands for.
type RowKind int

const (
	ProjectRow RowKind = iota
	FolderRow
	FileRow
	NoteRow // a line of explanation: projects hidden, a project still loading
)

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

// Rebuild lays the projects out as rows, keeping the selected row on the same thing.
func (sidebar *Sidebar) Rebuild(window Window) {
	var previous Row
	hadSelection := sidebar.Selected >= 0 && sidebar.Selected < len(sidebar.Rows)
	if hadSelection {
		previous = sidebar.Rows[sidebar.Selected]
	}
	sidebar.Rows = sidebar.Rows[:0]
	if sidebar.Entered != "" && window.ProjectIndex(sidebar.Entered) < 0 {
		sidebar.Entered = "" // the project was closed
	}
	if sidebar.Entered != "" {
		sidebar.addEnteredProject(window)
	} else {
		sidebar.addEveryProject(window)
	}
	sidebar.Stale = false
	sidebar.list.Count = len(sidebar.Rows)
	sidebar.decorateRows()
	sidebar.badgeRows(window)
	sidebar.anyOpen = anyExpanded(sidebar.Rows)
	sidebar.header = ""
	if headed := sidebar.powers.headed; headed != nil {
		sidebar.header = headed.Header()
	}
	sidebar.rebuilds++
	if hadSelection {
		for index, row := range sidebar.Rows {
			if row.Kind == previous.Kind && row.Project == previous.Project && row.Path == previous.Path {
				sidebar.Selected = index
				return
			}
		}
	}
	sidebar.Selected = pane.Within(sidebar.Selected, len(sidebar.Rows))
}

// addEnteredProject lists the content of the project entered, with no row for the project itself.
func (sidebar *Sidebar) addEnteredProject(window Window) {
	for index, project := range window.Projects() {
		if project.Path == sidebar.Entered {
			sidebar.addProjectContent(window, index, project, 0)
		}
	}
}

// addEveryProject lists each project not hidden, with its content when expanded, and says how many
// are hidden.
func (sidebar *Sidebar) addEveryProject(window Window) {
	hidden := 0
	for _, index := range sidebar.projectOrder(window) {
		project := window.Projects()[index]
		if listed, chosen := sidebar.chosenCount(index); chosen {
			if listed == 0 {
				continue // the listing chooses no file of it: not hidden, only not part of it
			}
		} else if window.Hidden(project) {
			hidden++
			continue
		}
		if !sidebar.listedByFilter(window, index) {
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
}

// projectOrder is the order the projects are listed in: the listing's, when it chooses its files.
func (sidebar *Sidebar) projectOrder(window Window) []int {
	if chooser := sidebar.powers.chooser; chooser != nil {
		if order := chooser.ProjectOrder(); order != nil {
			return order
		}
	}
	order := make([]int, len(window.Projects()))
	for index := range order {
		order[index] = index
	}
	return order
}

// chosenCount is how many files of a project the listing chooses, when it chooses them.
func (sidebar *Sidebar) chosenCount(project int) (int, bool) {
	chooser := sidebar.powers.chooser
	if chooser == nil {
		return 0, false
	}
	paths, own := chooser.ListedFiles(project)
	return len(paths), own
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

func (sidebar *Sidebar) hidesDeleted() bool {
	hider := sidebar.powers.hider
	return hider != nil && hider.HidesDeleted()
}

// LeavesOut tells whether a changed file is left out of the list whatever the filters: a file the
// development deleted, in a mode that hides them.
func (sidebar *Sidebar) LeavesOut(project *workspace.Project, path string) bool {
	if !sidebar.hidesDeleted() {
		return false
	}
	change, changed := project.ChangeOf(path)
	return changed && change.Status == git.Deleted
}

// withoutDeleted are the paths of the files the development did not delete, in a new slice.
func withoutDeleted(project *workspace.Project, paths []string) []string {
	kept := make([]string, 0, len(paths))
	for _, path := range paths {
		if change, changed := project.ChangeOf(path); changed && change.Status == git.Deleted {
			continue
		}
		kept = append(kept, path)
	}
	return kept
}

// badgeRows works out each project row's badge of the files servers wrote, as the rows are made.
func (sidebar *Sidebar) badgeRows(window Window) {
	sidebar.serverBadges = sidebar.serverBadges[:0]
	for _, row := range sidebar.Rows {
		badge := ""
		if row.Kind == ProjectRow {
			badge = serverBadge(window.Projects()[row.Project])
		}
		sidebar.serverBadges = append(sidebar.serverBadges, badge)
	}
}

// anyExpanded tells whether a project or a folder of rows is open.
func anyExpanded(rows []Row) bool {
	for _, row := range rows {
		if (row.Kind == ProjectRow || row.Kind == FolderRow) && row.Expanded {
			return true
		}
	}
	return false
}

// decorateRows works out what the mode adds to each row, as the rows are made, or as git's status
// is hidden or shown.
func (sidebar *Sidebar) decorateRows() {
	sidebar.decorations = sidebar.decorations[:0]
	for _, row := range sidebar.Rows {
		sidebar.decorations = append(sidebar.decorations, sidebar.decorate(row))
	}
	sidebar.decoratedWithoutStatus = !sidebar.ShowsStatus()
}
