package session

import "github.com/akrck02/code-review/internal/workspace"

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

// SetBase compares one project against base, from its next reading.
func (session *Session) SetBase(path, base string) { session.bases[path] = base }

// SetBaseForAll compares every project against base, forgetting the bases of single projects and
// the documents read against the old one.
func (session *Session) SetBaseForAll(base string) {
	session.Base = base
	clear(session.bases)
	session.documents.forgetAll()
}

// ToggleWorkingTree counts the changes not committed yet, or the commits only, and forgets the
// documents read the other way. It gives back whether they count now.
func (session *Session) ToggleWorkingTree() bool {
	session.WorkingTree = !session.WorkingTree
	session.documents.forgetAll()
	return session.WorkingTree
}

// LoadOptions are how a project is read: against its base, with the changes not committed yet or not.
func (session *Session) LoadOptions(path string) workspace.LoadOptions {
	return workspace.LoadOptions{Base: session.BaseFor(path), WorkingTree: session.WorkingTree}
}
