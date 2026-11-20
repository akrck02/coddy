package viewer

import (
	"slices"
	"testing"
)

// width is a text's width at 7 pixels a letter.
func width(text string) float32 { return float32(len([]rune(text))) * 7 }

// labels are the words of some crumbs.
func labels(crumbs []crumb) []string {
	var words []string
	for _, each := range crumbs {
		words = append(words, each.label)
	}
	return words
}

func TestThePathIsABreadcrumbOfItsFoldersNearestTheFileFirst(test *testing.T) {
	folder := "common/src/main/java/app/kaytek/android/common/data/repository"
	all := crumbs(folder, width, 2000)
	if !slices.Equal(labels(all), []string{"common", "src", "main", "java", "app", "kaytek", "android", "common", "data", "repository"}) {
		test.Errorf("with room for all: %v", labels(all))
	}
	if all[2].path != "common/src/main" || all[9].path != folder {
		test.Errorf("the crumbs show the folders %q and %q", all[2].path, all[9].path)
	}
	// room for "…", "data" and "repository", and three crumbs' padding, chevrons and gaps
	room := width("…") + width("data") + width("repository") + 3*crumbWidth
	some := crumbs(folder, width, room)
	if !slices.Equal(labels(some), []string{"…", "data", "repository"}) {
		test.Errorf("with room for the last two: %v", labels(some))
	}
	if some[0].path != "common/src/main/java/app/kaytek/android/common" || some[1].path != "common/src/main/java/app/kaytek/android/common/data" {
		test.Errorf("the gathered crumb shows %q, the next %q", some[0].path, some[1].path)
	}
	if none := crumbs(folder, width, 10); !slices.Equal(labels(none), []string{"…"}) || none[0].path != folder {
		test.Errorf("with no room: %v", none)
	}
	if top := crumbs("", width, 500); len(top) != 0 {
		test.Errorf("a file at the top of its project has crumbs: %v", top)
	}
}
