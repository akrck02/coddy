package sidebar

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/workspace"
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

func TestThePointerOnTheMarkOfAProjectLeftOutFindsIt(test *testing.T) {
	sidebar := New()
	sidebar.Rows = []Row{{Kind: ProjectRow, Project: 3}, {Kind: FileRow}}
	sidebar.leftOut = map[int][2]float32{0: {200, 219}}
	if !sidebar.onLeftOutMark(0, 210) || sidebar.onLeftOutMark(0, 230) || sidebar.onLeftOutMark(1, 210) {
		test.Error("the pointer lands on the mark outside it, or misses it inside it")
	}
}
