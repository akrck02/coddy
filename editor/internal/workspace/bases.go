package workspace

import "github.com/akrck02/coddy/editor/internal/codebase"

// BaseOf is the base chosen for one project, and whether one was.
func (workspace *Workspace) BaseOf(path string) (string, bool) {
	base, chosen := workspace.bases[path]
	return base, chosen
}

// BaseFor is what a project is compared against: its own base, or the one for all.
func (workspace *Workspace) BaseFor(path string) string {
	if base, chosen := workspace.bases[path]; chosen {
		return base
	}
	return workspace.Base
}

// SetBase compares one project against base, from its next reading, as it is on disk or in its
// last commit again.
func (workspace *Workspace) SetBase(path, base string) {
	workspace.bases[path] = base
	delete(workspace.heads, path)
}

// SetReview compares one project's head commit (a pull request's) against base, from its next
// reading, forgetting the documents read before.
func (workspace *Workspace) SetReview(path, head, base string) {
	workspace.bases[path] = base
	workspace.heads[path] = head
	workspace.documents.forgetAll()
}

// SetBaseForAll compares every project against base, forgetting the bases of single projects and
// the documents read against the old one.
func (workspace *Workspace) SetBaseForAll(base string) {
	workspace.Base = base
	clear(workspace.bases)
	clear(workspace.heads)
	workspace.documents.forgetAll()
}

// ToggleWorkingTree counts the changes not committed yet, or the commits only, and forgets the
// documents read the other way. It gives back whether they count now.
func (workspace *Workspace) ToggleWorkingTree() bool {
	workspace.WorkingTree = !workspace.WorkingTree
	workspace.documents.forgetAll()
	return workspace.WorkingTree
}

// LoadOptions are how a project is read: against its base, with the changes not committed yet or
// not, through its version control.
func (workspace *Workspace) LoadOptions(path string) codebase.LoadOptions {
	options := codebase.LoadOptions{Base: workspace.BaseFor(path), WorkingTree: workspace.WorkingTree, Head: workspace.heads[path]}
	for _, project := range workspace.Projects {
		if project.Path == path {
			options.Control = project.Control
		}
	}
	return options
}
