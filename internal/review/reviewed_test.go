package review

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/code-review/internal/workspace"
)

// The marks are kept as they were before the reader was split into modes: by repository, then by
// file, the fingerprint the file was viewed at.
func TestMarksSavedBeforeTheSplitAreRead(test *testing.T) {
	path := filepath.Join(test.TempDir(), "reviewed.json")
	saved := `{"/work/lighthouse":{"src/lamp.go":"base1:blob1","notes.txt":"base1:blob2"}}`
	if err := os.WriteFile(path, []byte(saved), 0o600); err != nil {
		test.Fatal(err)
	}
	project := &workspace.Project{Path: "/work/lighthouse", Fingerprints: map[string]string{
		"src/lamp.go": "base1:blob1", // as it was viewed
		"notes.txt":   "base1:blob3", // changed since
	}}
	reviewed := OpenReviewed(path)
	if !reviewed.IsViewed(project, "src/lamp.go") {
		test.Error("a file viewed before the split is not viewed after it")
	}
	if reviewed.IsViewed(project, "notes.txt") {
		test.Error("a file that changed after it was viewed is still viewed")
	}
}
