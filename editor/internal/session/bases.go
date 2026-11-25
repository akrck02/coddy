package session

import "github.com/akrck02/coddy/editor/internal/workspace"

// BaseOf is the base chosen for one project, and whether one was.
func (session *Session) BaseOf(path string) (string, bool) {
	base, chosen := session.bases[path]
	return base, chosen
}

// BaseFor is what a project is compared against: its own base, or the one for all.
func (session *Session) BaseFor(path string) string {
	if base, chosen := session.bases[path]; chosen {
		return base
	}
	return session.Base
}

// SetBase compares one project against base, from its next reading, as it is on disk or in its
// last commit again.
func (session *Session) SetBase(path, base string) {
	session.bases[path] = base
	delete(session.heads, path)
}

// SetReview compares one project's head commit (a pull request's) against base, from its next
// reading, forgetting the documents read before.
func (session *Session) SetReview(path, head, base string) {
	session.bases[path] = base
	session.heads[path] = head
	session.documents.forgetAll()
}

// SetBaseForAll compares every project against base, forgetting the bases of single projects and
// the documents read against the old one.
func (session *Session) SetBaseForAll(base string) {
	session.Base = base
	clear(session.bases)
	clear(session.heads)
	session.documents.forgetAll()
}

// ToggleWorkingTree counts the changes not committed yet, or the commits only, and forgets the
// documents read the other way. It gives back whether they count now.
func (session *Session) ToggleWorkingTree() bool {
	session.WorkingTree = !session.WorkingTree
	session.documents.forgetAll()
	return session.WorkingTree
}

// LoadOptions are how a project is read: against its base, with the changes not committed yet or
// not, through its version control.
func (session *Session) LoadOptions(path string) workspace.LoadOptions {
	options := workspace.LoadOptions{Base: session.BaseFor(path), WorkingTree: session.WorkingTree, Head: session.heads[path]}
	for _, project := range session.Projects {
		if project.Path == path {
			options.Control = project.Control
		}
	}
	return options
}
