package review

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/coddy/editor/internal/codebase"
)

func TestMarksThatCannotBeWrittenSayWhy(test *testing.T) {
	blocker := filepath.Join(test.TempDir(), "a-file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		test.Fatal(err)
	}
	reviewed := OpenReviewed(filepath.Join(blocker, "reviewed.json"))
	project := &codebase.Project{Path: "lighthouse", Fingerprints: map[string]string{"lamp.go": "one"}}
	err := reviewed.SetViewed(project, "lamp.go", true)
	if !errors.Is(err, MarksNotKept) {
		test.Errorf("keeping the marks under a file gave %v, want the code %v", err, MarksNotKept)
	}
	if !reviewed.IsViewed(project, "lamp.go") {
		test.Error("the mark was lost while the reader runs")
	}
}
