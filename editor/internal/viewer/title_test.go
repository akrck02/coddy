package viewer

import (
	"slices"
	"testing"
)

// width is a text's width at 7 pixels a letter.
func width(text string) float32 { return float32(len([]rune(text))) * 7 }

func TestThePathIsABreadcrumbOfItsFoldersNearestTheFileFirst(test *testing.T) {
	folder := "common/src/main/java/org/harbour/lamp/common/data/repository"
	all := crumbs(folder, width, 2000)
	if !slices.Equal(all, []string{"common", "src", "main", "java", "org", "harbour", "lamp", "common", "data", "repository"}) {
		test.Errorf("with room for all: %v", all)
	}
	// room for "…", "data" and "repository", and three crumbs' padding, chevrons and gaps
	room := width("…") + width("data") + width("repository") + 3*crumbWidth
	some := crumbs(folder, width, room)
	if !slices.Equal(some, []string{"…", "data", "repository"}) {
		test.Errorf("with room for the last two: %v", some)
	}
	if none := crumbs(folder, width, 10); !slices.Equal(none, []string{"…"}) {
		test.Errorf("with no room: %v", none)
	}
	if top := crumbs("", width, 500); len(top) != 0 {
		test.Errorf("a file at the top of its project has crumbs: %v", top)
	}
}
