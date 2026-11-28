// Package workspace is what the reader has open: the folders, the projects found in them, the base
// each project is compared against, and the documents already read. It knows nothing of windows,
// so it can be tested alone; reading a project or a file is the caller's job, on its own goroutine,
// and its result is handed back here.
package workspace

import (
	"slices"
	"sort"
	"strings"

	"github.com/akrck02/coddy/editor/internal/codebase"
)

// Workspace is what is open. Its fields and methods are only touched from one goroutine.
type Workspace struct {
	// Folders are the folders opened, and Projects the repositories found in them, by name
	Folders  []string
	Projects []*codebase.Project
	// Base is what every project without a base of its own is compared against ("" for its
	// default branch); WorkingTree tells whether the changes not committed yet count
	Base        string
	WorkingTree bool
	// Marked are the version controls plugins give, each known by the marker a repository of its
	// holds (".hg"); a folder with a .git is read by git all the same, unless GitOff
	Marked []Marked
	// GitOff reads no folder as a git repository: the person turned the git plugin off
	GitOff    bool
	bases     map[string]string // a base chosen for one project, by its path
	heads     map[string]string // a pull request's head commit read in one project, by its path
	documents documentCache
}

// Marked is a version control and the file or folder that marks its repositories.
type Marked struct {
	Marker  string
	Control codebase.VersionControl
}

// New is a workspace with nothing open, comparing against base.
func New(base string, workingTree bool) Workspace {
	return Workspace{Base: base, WorkingTree: workingTree, bases: map[string]string{}, heads: map[string]string{}, documents: newDocumentCache(DefaultDocumentMegabytes * bytesPerMegabyte)}
}

// AddFolders remembers folders and adds the repositories inside them not open yet, in the order
// of their names. The documents read are forgotten. Each new project is still to be read.
func (workspace *Workspace) AddFolders(folders []string) {
	known := map[string]bool{}
	for _, project := range workspace.Projects {
		known[project.Path] = true
	}
	for _, folder := range folders {
		if !slices.Contains(workspace.Folders, folder) {
			workspace.Folders = append(workspace.Folders, folder)
		}
	}
	for _, found := range workspace.discover(folders) {
		if !known[found.path] {
			workspace.Projects = append(workspace.Projects, &codebase.Project{Name: BaseName(found.path), Path: found.path, Control: found.control, Root: found.root})
		}
	}
	sort.SliceStable(workspace.Projects, func(first, second int) bool {
		return strings.ToLower(workspace.Projects[first].Name) < strings.ToLower(workspace.Projects[second].Name)
	})
	workspace.documents.forgetAll()
}

// DocumentMegabytes is how much memory, in megabytes, the documents kept may take.
func (workspace *Workspace) DocumentMegabytes() int {
	return workspace.documents.budget / bytesPerMegabyte
}

// SetDocumentMegabytes changes how much memory, in megabytes, the documents kept may take, and
// forgets the least recently opened at once until they fit. A budget is kept between
// DocumentMegabytesLeast and DocumentMegabytesMost, in whole steps; none (0, no preference) is
// the default.
func (workspace *Workspace) SetDocumentMegabytes(megabytes int) {
	workspace.documents.setBudget(budgetOf(megabytes) * bytesPerMegabyte)
}

// budgetOf is a budget asked, in megabytes, as one a person may choose.
func budgetOf(megabytes int) int {
	if megabytes <= 0 {
		return DefaultDocumentMegabytes
	}
	stepped := (megabytes + DocumentMegabytesStep/2) / DocumentMegabytesStep * DocumentMegabytesStep
	return min(max(stepped, DocumentMegabytesLeast), DocumentMegabytesMost)
}

// DocumentUse is how many documents are kept, and about how many bytes they take.
func (workspace *Workspace) DocumentUse() (documents, bytes int) {
	return workspace.documents.order.Len(), workspace.documents.used
}

// Clear forgets every folder, project and document.
func (workspace *Workspace) Clear() {
	workspace.Projects = nil
	workspace.Folders = nil
	workspace.documents.forgetAll()
}

// Unread puts an empty project in the place of the one at index, as it is read again, and gives
// back its path.
func (workspace *Workspace) Unread(index int) string {
	project := workspace.Projects[index]
	workspace.Projects[index] = &codebase.Project{Name: project.Name, Path: project.Path, Control: project.Control, Root: project.Root}
	return project.Path
}

// Loaded puts a project just read in the place of the one at its path, and forgets that
// project's documents, read from what it was before.
func (workspace *Workspace) Loaded(loaded *codebase.Project) {
	for position, current := range workspace.Projects {
		if current.Path == loaded.Path {
			loaded.Root = current.Root // where it was found is the workspace's, not the reading's
			workspace.Projects[position] = loaded
		}
	}
	workspace.documents.forgetWhere(func(reference codebase.FileReference) bool {
		return reference.Project < len(workspace.Projects) && workspace.Projects[reference.Project].Path == loaded.Path
	})
}

// BaseName is the last part of a path: the name of a folder or a file.
func BaseName(path string) string {
	path = strings.TrimRight(path, "/")
	return path[strings.LastIndexByte(path, '/')+1:]
}
