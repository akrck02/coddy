package workspace

import "testing"

// rootLayout fits a project with a file named "root", and lays it out as one folder named so.
type rootLayout struct{}

func (rootLayout) Fits(paths []string) bool {
	for _, path := range paths {
		if path == "root" {
			return true
		}
	}
	return false
}

func (rootLayout) Tree([]string, func(string) bool) *Folder { return &Folder{Name: "root"} }

func TestAProjectNoLayoutFitsIsLaidOutAsItsFoldersAre(test *testing.T) {
	paths := []string{"lamp/light.go", "README.md"}
	layout := LayoutOf([]Layout{rootLayout{}}, paths)
	if layout != PlainLayout {
		test.Fatalf("a project without a root file took %T, want the plain layout", layout)
	}
	tree := layout.Tree(paths, nil)
	if len(tree.Folders) != 1 || tree.Folders[0].Name != "lamp" || len(tree.Files) != 1 {
		test.Errorf("the plain layout gave %d folders and %d files, want the folder lamp and one file", len(tree.Folders), len(tree.Files))
	}
}

func TestAProjectTakesTheFirstLayoutThatFitsIt(test *testing.T) {
	layout := LayoutOf([]Layout{rootLayout{}}, []string{"lamp/light.go", "root"})
	if tree := layout.Tree(nil, nil); tree.Name != "root" {
		test.Errorf("a project with a root file was laid out as %q, want the root layout's tree", tree.Name)
	}
}
