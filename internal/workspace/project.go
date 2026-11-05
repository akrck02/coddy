// Package workspace is what the reader opens: the projects of a development, what changed in each
// since its base, the files of each, a file's diff and its tree. It knows nothing about the screen.
package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	// Counts are the lines each changed file adds and removes, by its path now.
	Counts map[string]git.LineCounts
	// Fingerprints name each changed file's text as the development has it, so a file marked as
	// reviewed is seen again when it changes.
	Fingerprints map[string]string

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
	changes := project.readChanges(options.WorkingTree)
	// a file deleted by the development is no longer in the working tree, but it is still worth
	// finding in the tree to read what was removed
	for _, change := range changes {
		files = append(files, change.Path)
	}
	project.Files = sortedUnique(files)
	return project
}

// readChanges reads what the development changed against its merge base: the commits ahead, the
// files changed, their counts of lines and their fingerprints. It returns the changes.
func (project *Project) readChanges(workingTree bool) []git.Change {
	path := project.Path
	project.Ahead = git.CommitsAhead(path, project.MergeBase)
	changes, err := git.Changes(path, project.Comparison(workingTree))
	if err != nil {
		project.Error = err.Error()
	}
	project.setChanges(changes)
	// counts that cannot be read leave the files without their +/− numbers; the changes still show
	project.Counts, _ = git.CountLines(path, project.Comparison(workingTree))
	if project.Counts == nil {
		project.Counts = map[string]git.LineCounts{}
	}
	project.Fingerprints = fingerprints(path, project.MergeBase, changes, workingTree, project.Counts)
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
func fingerprints(repository, mergeBase string, changes []git.Change, workingTree bool, counts map[string]git.LineCounts) map[string]string {
	result := make(map[string]string, len(changes))
	var blobs map[string]string
	if !workingTree {
		// a git that cannot list the content ids leaves the marks following only the base, not each
		// file's text: still usable, and nothing better can be had from it
		blobs, _ = git.BlobsAt(repository, "HEAD")
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
