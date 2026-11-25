package servers

import (
	"testing"
	"time"

	"github.com/akrck02/coddy/editor/internal/lsp"
)

func TestAServerIsSummedUpInAFewWords(test *testing.T) {
	started := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	projects := []lsp.ProjectProgress{{Path: "/a", State: "read"}, {Path: "/b", State: "reading"}, {Path: "/c", State: "reading"}}
	cases := []struct {
		progress lsp.Progress
		headline string
		fraction float32
	}{
		{lsp.Progress{Phase: "importing", Percent: -1, Done: 7, Total: 28, Started: started}, "Importing · 7 of 28 projects", 0.25},
		{lsp.Progress{Phase: "indexing", Percent: 71, Started: started, Projects: projects}, "Indexing · 71 % · 1 of 3 projects", 0.71},
		{lsp.Progress{Phase: "ready"}, "Ready", 1},
	}
	for _, each := range cases {
		view := ViewOf(each.progress)
		if view.Headline != each.headline || view.Fraction != each.fraction {
			test.Errorf("%+v is shown as %q at %v, want %q at %v", each.progress, view.Headline, view.Fraction, each.headline, each.fraction)
		}
	}
}
