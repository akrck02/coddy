package codebase

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A plain folder, with no version control, opens with every file on disk and no change, less
// the hidden files and the folders left out: the repositories inside it.
func TestAPlainFolderListsItsFilesButNotTheHiddenOnesNorTheLeftOut(test *testing.T) {
	folder := test.TempDir()
	for _, file := range []string{"notes.txt", "songs/morning.md", ".cache/skipped.txt", ".hidden", "archive/harbour/main.go"} {
		path := filepath.Join(folder, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			test.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("text"), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	project := LoadProject(context.Background(), folder, LoadOptions{Control: &FolderControl{LeftOut: []string{"archive/harbour"}}})
	if project.Error != nil {
		test.Fatalf("the folder was read with the error %v", project.Error)
	}
	if want := []string{"notes.txt", "songs/morning.md"}; !slices.Equal(project.Files, want) {
		test.Errorf("the files are %v, want %v", project.Files, want)
	}
	if len(project.Changes) != 0 {
		test.Errorf("a plain folder has the changes %v, want none", project.Changes)
	}
}
