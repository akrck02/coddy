package explorer

import (
	"reflect"
	"testing"
)

func TestAFolderHoldsTheFoldersAndFilesRightInsideIt(test *testing.T) {
	paths := []string{"README.md", "src/lamp.go", "src/beam/light.go", "src/beam/power.go", "src/keeper.go", "docs/guide.md"}
	cases := []struct {
		folder string
		wanted Listing
	}{
		{"", Listing{Folders: []string{"docs", "src"}, Files: []string{"README.md"}}},
		{"src", Listing{Folders: []string{"beam"}, Files: []string{"keeper.go", "lamp.go"}}},
		{"src/beam", Listing{Files: []string{"light.go", "power.go"}}},
		{"sr", Listing{}}, // a name that only starts like a folder's is not that folder
	}
	for _, each := range cases {
		if got := Contents(paths, each.folder); !reflect.DeepEqual(got, each.wanted) {
			test.Errorf("the folder %q holds %+v, wanted %+v", each.folder, got, each.wanted)
		}
	}
}
