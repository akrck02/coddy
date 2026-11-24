package sidebar

import (
	"slices"

	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/workspace"
)

// keptFilters is how many filters typed last keep their trees. Each letter typed is a new
// filter, so keeping all of them would keep a tree per letter for as long as the window is open;
// keeping the last few still makes going back with Backspace instant.
const keptFilters = 4

// treeKey is what a project's tree depends on: the mode, whether only changes are listed, and
// the filter when it chooses the files.
type treeKey struct {
	project int
	mode    string
	changed bool
	filter  string
}

// tree is a project's tree as the sidebar lists it now, made the first time it is asked for and
// kept until the projects change (Invalidate) or, for a filter, until it is among the oldest.
func (sidebar *Sidebar) tree(window Window, project int) *workspace.Folder {
	key := treeKey{project, sidebar.listing.Name(), sidebar.listsChangesOnly(), ""}
	if sidebar.filtersFiles(window, project) {
		key.filter = sidebar.Filter
	}
	if tree, built := sidebar.trees[key]; built {
		return tree
	}
	model := window.Projects()[project]
	isChanged := func(path string) bool { _, changed := model.ChangeOf(path); return changed }
	paths := sidebar.listedPaths(model, project)
	if key.filter != "" {
		paths = sidebar.found(paths)
		sidebar.keepFilter(key.filter)
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

// listedPaths are the paths of a project the listing shows, before any filter: every file, only
// the changed ones, or those the listing chooses, less the deleted ones in a mode that hides them.
func (sidebar *Sidebar) listedPaths(model *workspace.Project, project int) []string {
	paths := model.Files
	if sidebar.listsChangesOnly() {
		paths = model.ChangedPaths()
	}
	if chooser := sidebar.powers.chooser; chooser != nil {
		if chosen, own := chooser.ListedFiles(project); own {
			paths = chosen
		}
	}
	if sidebar.hidesDeleted() {
		paths = withoutDeleted(model, paths)
	}
	return paths
}

// keepFilter remembers a filter whose trees are kept, and forgets the trees of the oldest one
// once more than keptFilters are.
func (sidebar *Sidebar) keepFilter(filter string) {
	if slices.Contains(sidebar.filters, filter) {
		return
	}
	sidebar.filters = append(sidebar.filters, filter)
	if len(sidebar.filters) <= keptFilters {
		return
	}
	oldest := sidebar.filters[0]
	sidebar.filters = slices.Delete(sidebar.filters, 0, 1)
	for key := range sidebar.trees {
		if key.filter == oldest {
			delete(sidebar.trees, key)
		}
	}
}
