package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEachErrorCarriesItsCode(test *testing.T) {
	project := &Project{Path: test.TempDir()}
	os.WriteFile(filepath.Join(project.Path, "file"), []byte("on disk"), 0o644)
	_, changed := Save(project, "file", "mine", Stamp{})
	_, outside := Save(project, "../escape", "mine", Stamp{})
	_, notSaved := Save(project, "missing/folder/file", "mine", Stamp{})
	cases := []struct {
		name string
		err  error
		code error
	}{
		{"a file changed since it was read", changed, ChangedOnDisk},
		{"a path out of the project", outside, OutsideTheProject},
		{"a file in a folder that does not exist", notSaved, NotSaved},
	}
	for _, kind := range cases {
		if !errors.Is(kind.err, kind.code) {
			test.Errorf("%s gave %v, want the code %v", kind.name, kind.err, kind.code)
		}
	}
}
