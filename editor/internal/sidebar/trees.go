package sidebar

import (
	"slices"

	"github.com/akrck02/coddy/editor/internal/java"
	"github.com/akrck02/coddy/editor/internal/workspace"
)

// keptFilters is how many filters typed last keep their trees. Each letter typed is a new
// filter, so keeping all of them would keep a tree per letter for as long as the window is open;
// keeping the last few still makes going back with Backspace instant.
const keptFilters = 4

// projectLayouts are the kinds of project laid out otherwise than as their folders are on disk,
// tried in order; a project none fits is laid out plain.
var projectLayouts = []workspace.Layout{java.Layout}

// treeKey is what a project's tree depends on: the mode, whether only changes are listed, and
// the filter when it chooses the files.
type treeKey struct {
	project int
	mode    string
	changed bool
	filter  string
	nested  bool // the places of the projects found in a plain folder are in its tree
}

// tree is a project's tree as the sidebar lists it now, made the first time it is asked for and
// kept until the projects change (Invalidate) or, for a filter, until it is among the oldest.
func (sidebar *Sidebar) tree(window Window, project int) *workspace.Folder {
	model := window.Projects()[project]
	key := treeKey{project, sidebar.listing.Name(), sidebar.listsChangesOnly(), "", sidebar.nesting[model.Path]}
	if sidebar.filtersFiles(window, project) {
		key.filter = sidebar.Filter
	}
	if tree, built := sidebar.trees[key]; built {
		return tree
	}
	isChanged := func(path string) bool { _, changed := model.ChangeOf(path); return changed }
	paths := sidebar.listedPaths(model, project)
	if key.filter != "" {
		paths = sidebar.found(paths)
		sidebar.keepFilter(key.filter)
	}
	var tree *workspace.Folder
	if key.nested {
		tree = workspace.BuildTreeAround(paths, sidebar.placesIn(window, model), isChanged)
	} else {
		tree = workspace.LayoutOf(projectLayouts, model.Files).Tree(paths, isChanged)
	}
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
