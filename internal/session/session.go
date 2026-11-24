// Package session is what the reader has open: the folders, the projects found in them, the base
// each project is compared against, and the documents already read. It knows nothing of windows,
// so it can be tested alone; reading a project or a file is the caller's job, on its own goroutine,
// and its result is handed back here.
package session

import (
	"slices"
	"sort"
	"strings"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/workspace"
)

// Session is what is open. Its fields and methods are only touched from one goroutine.
type Session struct {
	// Folders are the folders opened, and Projects the repositories found in them, by name
	Folders  []string
	Projects []*workspace.Project
	// Base is what every project without a base of its own is compared against ("" for its
	// default branch); WorkingTree tells whether the changes not committed yet count
	Base        string
	WorkingTree bool
	bases       map[string]string // a base chosen for one project, by its path
	documents   documentCache
}

// New is a session with nothing open, comparing against base.
func New(base string, workingTree bool) Session {
	return Session{Base: base, WorkingTree: workingTree, bases: map[string]string{}, documents: newDocumentCache(DefaultDocumentMegabytes * bytesPerMegabyte)}
}

// AddFolders remembers folders and adds the repositories inside them not open yet, in the order
// of their names. The documents read are forgotten. Each new project is still to be read.
func (session *Session) AddFolders(folders []string) {
	known := map[string]bool{}
	for _, project := range session.Projects {
		known[project.Path] = true
	}
	for _, folder := range folders {
		if !slices.Contains(session.Folders, folder) {
			session.Folders = append(session.Folders, folder)
		}
	}
	for _, path := range git.Discover(folders) {
		if !known[path] {
			session.Projects = append(session.Projects, &workspace.Project{Name: BaseName(path), Path: path})
		}
	}
	sort.SliceStable(session.Projects, func(first, second int) bool {
		return strings.ToLower(session.Projects[first].Name) < strings.ToLower(session.Projects[second].Name)
	})
	session.documents.forgetAll()
}

// DocumentMegabytes is how much memory, in megabytes, the documents kept may take.
func (session *Session) DocumentMegabytes() int {
	return session.documents.budget / bytesPerMegabyte
}

// SetDocumentMegabytes changes how much memory, in megabytes, the documents kept may take, and
// forgets the least recently opened at once until they fit. A budget that is not one of
// DocumentMegabyteChoices (a preference written by hand, or none) is taken as the default.
func (session *Session) SetDocumentMegabytes(megabytes int) {
	if !slices.Contains(DocumentMegabyteChoices, megabytes) {
		megabytes = DefaultDocumentMegabytes
	}
	session.documents.setBudget(megabytes * bytesPerMegabyte)
}

// DocumentUse is how many documents are kept, and about how many bytes they take.
func (session *Session) DocumentUse() (documents, bytes int) {
	return session.documents.order.Len(), session.documents.used
}

// Clear forgets every folder, project and document.
func (session *Session) Clear() {
	session.Projects = nil
	session.Folders = nil
	session.documents.forgetAll()
}

// Unread puts an empty project in the place of the one at index, as it is read again, and gives
// back its path.
func (session *Session) Unread(index int) string {
	project := session.Projects[index]
	session.Projects[index] = &workspace.Project{Name: project.Name, Path: project.Path}
	return project.Path
}

// Loaded puts a project just read in the place of the one at its path, and forgets that
// project's documents, read from what it was before.
func (session *Session) Loaded(loaded *workspace.Project) {
	for position, current := range session.Projects {
		if current.Path == loaded.Path {
			session.Projects[position] = loaded
		}
	}
	session.documents.forgetWhere(func(reference workspace.FileReference) bool {
		return reference.Project < len(session.Projects) && session.Projects[reference.Project].Path == loaded.Path
	})
}

// BaseName is the last part of a path: the name of a folder or a file.
func BaseName(path string) string {
	path = strings.TrimRight(path, "/")
	return path[strings.LastIndexByte(path, '/')+1:]
}
