package sidebar

import (
	"testing"

	"github.com/akrck02/code-review/internal/workspace"
)

func TestTheBadgeCountsTheFilesOfEachServer(test *testing.T) {
	project := &workspace.Project{ServerFiles: []workspace.ServerFile{
		{Path: ".classpath", Server: "jdtls"}, {Path: ".project", Server: "jdtls"}, {Path: "go.work", Server: "gopls"},
	}}
	if badge := serverBadge(project); badge != "gopls · 1  jdtls · 2" {
		test.Errorf("the badge says %q", badge)
	}
	if badge := serverBadge(&workspace.Project{}); badge != "" {
		test.Errorf("a project no server wrote in has the badge %q", badge)
	}
}

func TestAClickOnTheBadgeIsOnlyOnIt(test *testing.T) {
	sidebar := New()
	sidebar.Rows = []Row{{Kind: ProjectRow}, {Kind: FileRow}}
	sidebar.badges = map[int][2]float32{0: {200, 260}}
	if !sidebar.onBadge(0, 230) || sidebar.onBadge(0, 190) || sidebar.onBadge(1, 230) {
		test.Error("a click lands on the badge outside it, or misses it inside it")
	}
}
