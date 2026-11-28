package sidebar

import (
	"path/filepath"

	"github.com/akrck02/coddy/editor/internal/codebase"
)

// findNesting works out which plain folders hold the projects found in them in their trees: each
// one listed, unless the listing shows changes only, chooses its files or is filtered. Such a list
// is about changes and names, not places, so the projects are then listed at the top, each on
// its own.
func (sidebar *Sidebar) findNesting(window Window) {
	clear(sidebar.nesting)
	if sidebar.powers.chooser != nil || sidebar.listsChangesOnly() || sidebar.Filter != "" {
		return
	}
	for _, project := range window.Projects() {
		if project.IsPlainFolder() && !window.Hidden(project) {
			if sidebar.nesting == nil {
				sidebar.nesting = map[string]bool{}
			}
			sidebar.nesting[project.Path] = true
		}
	}
}

// nestedAt is the place in the window of the project found in a plain folder at a path inside
// it, or -1 when none is open there.
func (sidebar *Sidebar) nestedAt(window Window, root *codebase.Project, place string) int {
	path := filepath.Join(root.Path, filepath.FromSlash(place))
	for index, project := range window.Projects() {
		if project.Root == root.Path && project.Path == path {
			return index
		}
	}
	return -1
}

// placesIn are the paths, inside a plain folder that nests them, of the projects found in it.
func (sidebar *Sidebar) placesIn(window Window, root *codebase.Project) []string {
	if !sidebar.nesting[root.Path] {
		return nil
	}
	var places []string
	for _, project := range window.Projects() {
		if project.Root != root.Path {
			continue
		}
		if relative, err := filepath.Rel(root.Path, project.Path); err == nil {
			places = append(places, filepath.ToSlash(relative))
		}
	}
	return places
}
