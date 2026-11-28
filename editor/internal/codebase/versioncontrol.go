package codebase

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/git"
)

// VersionControl is what a project's version control must give for Coddy to list its changes and
// show a changed file's two sides. Git is the built-in one; a plugin can give another. What only
// git can do (counting lines, fingerprints, commits ahead, choosing a base by name) stays git's.
type VersionControl interface {
	// Branch is the name of what is checked out.
	Branch(folder string) (string, error)
	// DefaultBase is the revision to compare against when the person chose none, "" for none.
	DefaultBase(folder string) (string, error)
	// Files are every file it knows and every new one it does not ignore.
	Files(stop context.Context, folder string) ([]string, error)
	// Changes are the files changed between the comparison's base and the disk or last commit.
	Changes(stop context.Context, folder string, comparison git.Comparison) ([]git.Change, error)
	// ReadAt is a file's content at a revision; LastCommit is the last commit.
	ReadAt(folder, revision, path string) ([]byte, error)
	// Diff is a changed file's lines, numbered on both sides.
	Diff(folder string, comparison git.Comparison, change git.Change) (git.FileDiff, error)
}

// LastCommit is the revision of the last commit, whatever the version control calls it.
const LastCommit = "HEAD"

// Git is the built-in version control.
var Git VersionControl = gitControl{}

type gitControl struct{}

func (gitControl) Branch(folder string) (string, error) { return git.CurrentBranch(folder), nil }

func (gitControl) DefaultBase(folder string) (string, error) { return git.DefaultBase(folder), nil }

func (gitControl) Files(stop context.Context, folder string) ([]string, error) {
	return git.TrackedAndNewFiles(stop, folder)
}

func (gitControl) Changes(stop context.Context, folder string, comparison git.Comparison) ([]git.Change, error) {
	return git.Changes(stop, folder, comparison)
}

func (gitControl) ReadAt(folder, revision, path string) ([]byte, error) {
	return git.ReadAtRevision(folder, revision, path)
}

func (gitControl) Diff(folder string, comparison git.Comparison, change git.Change) (git.FileDiff, error) {
	return git.Diff(folder, comparison, change)
}

// control is the project's version control: git unless a plugin gives it.
func (project *Project) control() VersionControl {
	if project.Control == nil {
		return Git
	}
	return project.Control
}

// loadThrough reads a project through a version control other than git: only what the interface
// gives, so its files have no counts of lines and no fingerprints.
// TODO(akrck02): counts and fingerprints for other version controls, computed from both sides.
func loadThrough(stop context.Context, project *Project, options LoadOptions) *Project {
	control := options.Control
	folder := project.Path
	var err error
	if project.Branch, err = control.Branch(folder); err != nil {
		project.Error = err
		return project
	}
	project.Base = options.Base
	if project.Base == "" {
		if project.Base, err = control.DefaultBase(folder); err != nil {
			project.Error = err
			return project
		}
	}
	project.MergeBase = project.Base
	project.Counts = map[string]git.LineCounts{}
	project.Fingerprints = map[string]string{}
	files, err := control.Files(stop, folder)
	if err != nil {
		project.Error = err
		return project
	}
	if project.Base == "" {
		project.Error = NoBase
		project.Files = sortedUnique(files)
		return project
	}
	changes, err := control.Changes(stop, folder, project.Comparison(options.WorkingTree))
	if err != nil {
		project.Error = err
	}
	project.setChanges(changes)
	for _, change := range changes {
		files = append(files, change.Path)
	}
	project.Files = sortedUnique(files)
	return project
}
