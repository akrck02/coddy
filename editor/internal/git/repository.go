package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// IsRepository tells whether a folder is the top of a git working tree (it holds a .git folder,
// or a .git file as worktrees and submodules do).
func IsRepository(folder string) bool {
	_, err := os.Stat(filepath.Join(folder, ".git"))
	return err == nil
}

// CurrentBranch is the name of the branch checked out, or a short commit when the head is detached.
func CurrentBranch(repository string) string {
	branch, err := runText(repository, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	if branch == "HEAD" {
		short, _ := runText(repository, "rev-parse", "--short", "HEAD")
		return "detached at " + short
	}
	return branch
}

// baseCandidates are the refs a development is usually compared against, best first: the remote's
// copy is preferred to a local branch, which may not have been brought up to date.
var baseCandidates = []string{
	"origin/main", "origin/master", "main", "master", "origin/develop", "develop",
}

// DefaultBase finds the branch a development in this repository is compared against: the remote's
// default branch when it is known, or else the first of main, master and develop that exists.
func DefaultBase(repository string) string {
	if remoteHead, err := runText(repository, "symbolic-ref", "-q", "--short", "refs/remotes/origin/HEAD"); err == nil && remoteHead != "" {
		return remoteHead
	}
	for _, candidate := range baseCandidates {
		if RefExists(repository, candidate) {
			return candidate
		}
	}
	return ""
}

// RefExists tells whether a branch, tag or commit names a commit in this repository.
func RefExists(repository, ref string) bool {
	if ref == "" {
		return false
	}
	_, err := runText(repository, "rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	return err == nil
}

// MergeBase is the commit where head (LastCommit for the current branch) left the base: what the
// development is compared against, so changes made on the base since then are not counted as the
// development's.
func MergeBase(repository, head, base string) (string, error) {
	return runText(repository, "merge-base", "--end-of-options", head, base)
}

// CommitsAhead counts the commits on head (LastCommit for the current branch) since it left the
// base.
func CommitsAhead(repository, mergeBase, head string) int {
	text, err := runText(repository, "rev-list", "--count", mergeBase+".."+head)
	if err != nil {
		return 0
	}
	count, _ := strconv.Atoi(text)
	return count
}

// TrackedAndNewFiles lists every file of the working tree git knows about, and the new files it
// does not ignore, as paths relative to the repository. A tracked file deleted from the disk, and
// not yet committed so, is not on the list: it is no longer in the working tree. Git stops once
// stop is cancelled.
func TrackedAndNewFiles(stop context.Context, repository string) ([]string, error) {
	tracked, err := runStoppable(stop, repository, "ls-files", "-z", "--cached")
	if err != nil {
		return nil, err
	}
	deleted, err := runStoppable(stop, repository, "ls-files", "-z", "--deleted")
	if err != nil {
		return nil, err
	}
	untracked, err := runStoppable(stop, repository, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	gone := map[string]bool{}
	for _, path := range splitNull(deleted) {
		gone[path] = true
	}
	files := slices.DeleteFunc(splitNull(tracked), func(path string) bool { return gone[path] })
	return append(files, splitNull(untracked)...), nil
}

// ReadAtRevision reads a file as it was in a commit.
func ReadAtRevision(repository, revision, path string) ([]byte, error) {
	return run(repository, "cat-file", "blob", revision+":"+path)
}

// ReadWorkingFile reads a file as it is on disk now.
func ReadWorkingFile(repository, path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(repository, filepath.FromSlash(path)))
}

// splitNull splits git's -z output into its entries.
func splitNull(output []byte) []string {
	text := strings.TrimSuffix(string(output), "\x00")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\x00")
}
