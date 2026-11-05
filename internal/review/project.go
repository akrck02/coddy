// Package review is what a reviewer looks at: the projects of a development, what changed in each
// since its base, the files of each and a changed file's diff. It knows nothing about the screen.
package review

import (
	"path/filepath"
	"sort"

	"github.com/akrck02/code-review/internal/git"
)

// Project is one repository of the development.
type Project struct {
	Name string
	Path string

	Branch string
	// Base is the ref the development is compared against (origin/main, a tag, a commit), and
	// MergeBase the commit where the branch left it.
	Base      string
	MergeBase string
	Ahead     int // commits on the branch since it left the base

	Changes []git.Change // sorted by path
	Files   []string     // every file, sorted

	changeIndex map[string]int

	Loaded bool
	Error  string
}

// Comparison is what this project's changes are measured between.
func (project *Project) Comparison(workingTree bool) git.Comparison {
	return git.Comparison{MergeBase: project.MergeBase, WorkingTree: workingTree}
}

// ChangeOf is the change made to a file, if it changed.
func (project *Project) ChangeOf(path string) (git.Change, bool) {
	index, changed := project.changeIndex[path]
	if !changed {
		return git.Change{}, false
	}
	return project.Changes[index], true
}

// LoadOptions say how a project is read.
type LoadOptions struct {
	// Base, when not empty, is the ref to compare against instead of the project's default branch.
	Base string
	// WorkingTree counts the changes not committed yet.
	WorkingTree bool
}

// LoadProject reads a repository: its branch, its base, what changed and every file. It only reads.
func LoadProject(path string, options LoadOptions) *Project {
	project := &Project{Name: filepath.Base(path), Path: path, Loaded: true}
	project.Branch = git.CurrentBranch(path)
	project.Base = options.Base
	if project.Base == "" || !git.RefExists(path, project.Base) {
		if options.Base != "" {
			project.Error = "base " + options.Base + " not found; using the default branch"
		}
		project.Base = git.DefaultBase(path)
	}
	files, err := git.TrackedAndNewFiles(path)
	if err != nil {
		project.Error = err.Error()
	}
	if project.Base == "" {
		project.Error = "no base branch found (main, master, develop)"
		project.Files = sortedUnique(files)
		return project
	}
	project.MergeBase, err = git.MergeBase(path, project.Base)
	if err != nil {
		project.Error = err.Error()
		project.Files = sortedUnique(files)
		return project
	}
	project.Ahead = git.CommitsAhead(path, project.MergeBase)
	changes, err := git.Changes(path, project.Comparison(options.WorkingTree))
	if err != nil {
		project.Error = err.Error()
	}
	project.setChanges(changes)
	// a file deleted by the development is no longer in the working tree, but it is still worth
	// finding in the tree to read what was removed
	for _, change := range changes {
		files = append(files, change.Path)
	}
	project.Files = sortedUnique(files)
	return project
}

func (project *Project) setChanges(changes []git.Change) {
	sort.Slice(changes, func(first, second int) bool { return changes[first].Path < changes[second].Path })
	project.Changes = changes
	project.changeIndex = make(map[string]int, len(changes))
	for index, change := range changes {
		project.changeIndex[change.Path] = index
	}
}

func sortedUnique(paths []string) []string {
	sort.Strings(paths)
	unique := paths[:0]
	for index, path := range paths {
		if index > 0 && path == paths[index-1] {
			continue
		}
		unique = append(unique, path)
	}
	return unique
}
