package workspace

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/akrck02/code-review/internal/git"
)

func TestEachErrorCarriesItsCode(test *testing.T) {
	project := &Project{Path: test.TempDir()}
	os.WriteFile(filepath.Join(project.Path, "file"), []byte("on disk"), 0o644)
	_, changed := Save(project, "file", "mine", Stamp{})
	_, outside := Save(project, "../escape", "mine", Stamp{})
	_, notSaved := Save(project, "missing/folder/file", "mine", Stamp{})
	notKept := recordUnderAFile(test)
	cases := []struct {
		name string
		err  error
		code error
	}{
		{"a file changed since it was read", changed, ChangedOnDisk},
		{"a path out of the project", outside, OutsideTheProject},
		{"a file in a folder that does not exist", notSaved, NotSaved},
		{"a record in a folder that is a file", notKept, RecordNotKept},
	}
	for _, kind := range cases {
		if !errors.Is(kind.err, kind.code) {
			test.Errorf("%s gave %v, want the code %v", kind.name, kind.err, kind.code)
		}
	}
}

// recordUnderAFile keeps what a server wrote in a record whose folder is a file, which fails.
func recordUnderAFile(test *testing.T) error {
	blocker := filepath.Join(test.TempDir(), "a file")
	os.WriteFile(blocker, nil, 0o644)
	project := &Project{Path: test.TempDir(), Fingerprints: map[string]string{".classpath": "base:abc"}}
	for _, arguments := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "start"}} {
		command := exec.Command("git", arguments...)
		command.Dir = project.Path
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	os.WriteFile(filepath.Join(project.Path, ".classpath"), []byte("by the server"), 0o644)
	project.setChanges([]git.Change{{Status: git.Added, Path: ".classpath", Untracked: true}})
	snapshot := Snapshot{Server: "jdtls", Files: map[string]bool{}, Changed: map[string]string{}}
	return OpenWritten(filepath.Join(blocker, "wrote.json")).SortOut(project, &snapshot, true)
}
