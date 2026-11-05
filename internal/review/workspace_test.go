package review

import (
	"os"
	"testing"
)

// TestReadingARealRepository reads a repository named by REVIEW_REPOSITORY, to try the reading
// against real developments; without it the test is skipped.
func TestReadingARealRepository(test *testing.T) {
	path := os.Getenv("REVIEW_REPOSITORY")
	if path == "" {
		test.Skip("set REVIEW_REPOSITORY to a repository to read")
	}
	project := LoadProject(path, LoadOptions{WorkingTree: true})
	test.Logf("%s on %s against %s (%s), %d commits ahead, %d changed of %d files, error %q",
		project.Name, project.Branch, project.Base, project.MergeBase, project.Ahead,
		len(project.Changes), len(project.Files), project.Error)
	for index, change := range project.Changes {
		if index >= 6 {
			break
		}
		document := OpenDocument(project, change.Path, true)
		added, removed := Counts(document.Diff)
		test.Logf("  %s %s (%s) +%d -%d, %d rows folded, error %q", change.Status.Letter(), change.Path,
			document.Language, added, removed, len(DiffRows(document.Diff, 3, nil)), document.Error)
	}
}
