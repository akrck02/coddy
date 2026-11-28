// Package review is the review mode: only the changed files, opened on their diffs, each marked as
// viewed once read, the marks kept in a file of the reader's own.
package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/go-renderer/failure"
)

// Reviewed remembers the changed files a reviewer has marked as viewed, as GitHub's "Viewed" box
// does: by the file's text, so a file that changes again after it was viewed is shown as not
// viewed. It is kept in a file of the reader's own, never in the repositories.
type Reviewed struct {
	path   string
	mutex  sync.Mutex
	marked map[string]map[string]string // repository → file → the fingerprint it was viewed at
}

// OpenReviewed reads what was viewed from a file; a missing or unreadable file starts empty. An
// empty path keeps nothing on disk (tests).
func OpenReviewed(path string) *Reviewed {
	reviewed := &Reviewed{path: path, marked: map[string]map[string]string{}}
	if path == "" {
		return reviewed
	}
	if content, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(content, &reviewed.marked) // a damaged file starts empty, as a missing one does
	}
	if reviewed.marked == nil {
		reviewed.marked = map[string]map[string]string{}
	}
	return reviewed
}

// DefaultReviewedPath is where the viewed files are kept: in the user's configuration folder.
func DefaultReviewedPath() string {
	folder, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(folder, "coddy", "reviewed.json")
}

// IsViewed tells whether a changed file was viewed as its text is now.
func (reviewed *Reviewed) IsViewed(project *codebase.Project, path string) bool {
	fingerprint, known := project.Fingerprints[path]
	if !known {
		return false
	}
	reviewed.mutex.Lock()
	defer reviewed.mutex.Unlock()
	return reviewed.marked[project.Path][path] == fingerprint
}

// SetViewed marks a changed file as viewed, or not, and keeps it.
func (reviewed *Reviewed) SetViewed(project *codebase.Project, path string, viewed bool) error {
	fingerprint, known := project.Fingerprints[path]
	if !known {
		return nil
	}
	reviewed.mutex.Lock()
	files := reviewed.marked[project.Path]
	if files == nil {
		files = map[string]string{}
		reviewed.marked[project.Path] = files
	}
	if viewed {
		files[path] = fingerprint
	} else {
		delete(files, path)
	}
	content, err := json.MarshalIndent(reviewed.marked, "", "  ")
	reviewed.mutex.Unlock()
	if err != nil {
		return failure.Wrap(MarksNotKept, reviewed.path, err)
	}
	if reviewed.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(reviewed.path), 0o755); err != nil {
		return failure.Wrap(MarksNotKept, reviewed.path, err)
	}
	if err := os.WriteFile(reviewed.path, content, 0o644); err != nil {
		return failure.Wrap(MarksNotKept, reviewed.path, err)
	}
	return nil
}

// Progress counts a project's changed files viewed so far.
func (reviewed *Reviewed) Progress(project *codebase.Project) (viewed, total int) {
	for _, change := range project.Changes {
		if reviewed.IsViewed(project, change.Path) {
			viewed++
		}
	}
	return viewed, len(project.Changes)
}
