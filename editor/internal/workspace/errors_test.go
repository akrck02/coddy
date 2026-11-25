package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/akrck02/coddy/editor/internal/git"
)

func TestEachErrorCarriesItsCode(test *testing.T) {
	project := &Project{Path: test.TempDir()}
	os.WriteFile(filepath.Join(project.Path, "file"), []byte("on disk"), 0o644)
	_, changed := Save(project, "file", "mine", Stamp{})
	_, outside := Save(project, "../escape", "mine", Stamp{})
	_, notSaved := Save(project, "missing/folder/file", "mine", Stamp{})
	notKept := recordUnderAFile(test)
	baseNotFound := LoadProject(context.Background(), repositoryOnBranch(test, "main"), LoadOptions{Base: "nowhere"}).Error
	noBase := LoadProject(context.Background(), repositoryOnBranch(test, "lamp"), LoadOptions{}).Error
	large := &Project{Path: test.TempDir()}
	os.WriteFile(filepath.Join(large.Path, "large"), bytes.Repeat([]byte("a"), largestShown+1), 0o644)
	tooLarge := OpenDocument(large, "large", true).Error
	os.Symlink(filepath.Join(project.Path, "gone"), filepath.Join(project.Path, "dangling")) // its error is checked below
	_, dangling := Save(project, "dangling", "mine", Stamp{})
	cases := []struct {
		name string
		err  error
		code error
	}{
		{"a file changed since it was read", changed, ChangedOnDisk},
		{"a path out of the project", outside, OutsideTheProject},
		{"a file in a folder that does not exist", notSaved, NotSaved},
		{"a record in a folder that is a file", notKept, RecordNotKept},
		{"a base the repository does not have", baseNotFound, BaseNotFound},
		{"a repository with no usual base branch", noBase, NoBase},
		{"a file too large to show", tooLarge, TooLarge},
		{"a link to nothing", dangling, DanglingLink},
	}
	for _, kind := range cases {
		if !errors.Is(kind.err, kind.code) {
			test.Errorf("%s gave %v, want the code %v", kind.name, kind.err, kind.code)
		}
	}
}

// repositoryOnBranch makes a repository of one commit, on the branch named.
func repositoryOnBranch(test *testing.T, branch string) string {
	folder := test.TempDir()
	for _, arguments := range [][]string{{"init", "-q", "-b", branch}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "start"}} {
		command := exec.Command("git", arguments...)
		command.Dir = folder
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return folder
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
