// Package workspace is what the reader opens: the projects of a development, what changed in each
// since its base, the files of each, a file's diff and its tree. It knows nothing about the screen.
package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"

	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/go-renderer/failure"
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
	// Head, when not empty, is the commit read instead of the files on disk or the last commit:
	// a pull request's, which is not checked out
	Head string

	Changes []git.Change // sorted by path
	Files   []string     // every file, sorted
	// Counts are the lines each changed file adds and removes, by its path now.
	Counts map[string]git.LineCounts
	// Fingerprints name each changed file's text as the development has it, so a file marked as
	// reviewed is seen again when it changes.
	Fingerprints map[string]string
	// ServerFiles are the files language servers wrote, left out of Changes (SortOut)
	ServerFiles []ServerFile

	changeIndex map[string]int

	// Control is the version control the project is read through; nil is git.
	Control VersionControl
	// Root is the path of the plain folder this project was found in, when that folder is a
	// project of its own and shows it in its tree; "" otherwise.
	Root string

	Loaded bool
	// Error is why the project was read only in part, with its code; nil when it was read whole
	Error error
}

// Comparison is what this project's changes are measured between.
func (project *Project) Comparison(workingTree bool) git.Comparison {
	return git.Comparison{MergeBase: project.MergeBase, WorkingTree: workingTree, Head: project.Head}
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
	// Control, when not nil, reads the project instead of git.
	Control VersionControl
	// Head, when not empty, is the commit compared with the base (a pull request's), instead of
	// the files on disk or the last commit. Only git reads it.
	// TODO(akrck02): a head for the projects of other version controls.
	Head string
}

// LoadProject reads a repository: its branch, its base, what changed and every file. It only reads.
// Its longest git commands stop once stop is cancelled, and what they gave is then incomplete.
func LoadProject(stop context.Context, path string, options LoadOptions) *Project {
	project := &Project{Name: filepath.Base(path), Path: path, Control: options.Control, Loaded: true}
	if options.Control != nil {
		return loadThrough(stop, project, options)
	}
	project.Branch = git.CurrentBranch(path)
	project.Head = options.Head
	project.Base = options.Base
	if project.Base == "" || !git.RefExists(path, project.Base) {
		if options.Base != "" {
			project.Error = failure.Wrap(BaseNotFound, options.Base, nil)
		}
		project.Base = git.DefaultBase(path)
	}
	files, err := git.TrackedAndNewFiles(stop, path)
	if err != nil {
		project.Error = err
	}
	if project.Base == "" {
		project.Error = NoBase
		project.Files = sortedUnique(files)
		return project
	}
	project.MergeBase, err = git.MergeBase(path, project.Comparison(false).Last(), project.Base)
	if err != nil {
		project.Error = err
		project.Files = sortedUnique(files)
		return project
	}
	changes := project.loadChanges(stop, options.WorkingTree)
	// a file deleted by the development is no longer in the working tree, but it is still worth
	// finding in the tree to read what was removed
	for _, change := range changes {
		files = append(files, change.Path)
	}
	project.Files = sortedUnique(files)
	return project
}

// loadChanges reads what the development changed against its merge base and keeps it in the
// project: the commits ahead, the files changed, their counts of lines and their fingerprints. It
// returns the changes.
func (project *Project) loadChanges(stop context.Context, workingTree bool) []git.Change {
	path := project.Path
	comparison := project.Comparison(workingTree)
	project.Ahead = git.CommitsAhead(path, project.MergeBase, comparison.Last())
	changes, err := git.Changes(stop, path, comparison)
	if err != nil {
		project.Error = err
	}
	project.setChanges(changes)
	// counts that cannot be read leave the files without their +/− numbers; the changes still show
	project.Counts, _ = git.CountLines(stop, path, comparison)
	if project.Counts == nil {
		project.Counts = map[string]git.LineCounts{}
	}
	project.Fingerprints = fingerprints(stop, path, comparison, changes, project.Counts)
	return changes
}

// ChangedPaths are the paths of the files the development changed, sorted.
func (project *Project) ChangedPaths() []string {
	paths := make([]string, len(project.Changes))
	for index, change := range project.Changes {
		paths[index] = change.Path
	}
	return paths
}

// TotalCounts adds up the lines the development adds and removes in this project.
func (project *Project) TotalCounts() (added, removed int) {
	for _, counts := range project.Counts {
		added += counts.Added
		removed += counts.Removed
	}
	return added, removed
}

// fingerprints names the text of each changed file: the content id git gives it in the last
// commit, or a hash of the file on disk when changes not committed yet count. Each starts with
// the base, so choosing another base starts the review again. New files git does not know are
// counted here too, as wholly added.
func fingerprints(stop context.Context, repository string, comparison git.Comparison, changes []git.Change, counts map[string]git.LineCounts) map[string]string {
	result := make(map[string]string, len(changes))
	mergeBase, workingTree := comparison.MergeBase, comparison.OnDisk()
	var blobs map[string]string
	if !workingTree {
		// a git that cannot list the content ids leaves the marks following only the base, not each
		// file's text: still usable, and nothing better can be had from it
		blobs, _ = git.BlobsAt(stop, repository, comparison.Last())
	}
	for _, change := range changes {
		key := mergeBase + ":"
		switch {
		case change.Status == git.Deleted:
			key += "deleted"
		case !workingTree:
			key += blobs[change.Path]
		default:
			content, err := git.ReadWorkingFile(repository, change.Path)
			if err != nil {
				key += "unreadable"
				break
			}
			sum := sha256.Sum256(content)
			key += hex.EncodeToString(sum[:12])
			if change.Untracked {
				counts[change.Path] = git.LineCounts{Added: countLines(content), Binary: isBinary(content)}
			}
		}
		result[change.Path] = key
	}
	return result
}

func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	lines := bytes.Count(content, []byte{'\n'})
	if content[len(content)-1] != '\n' {
		lines++
	}
	return lines
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
