package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// projectWith is a project in a temporary folder holding one file.
func projectWith(test *testing.T, name, content string, permissions os.FileMode) *Project {
	folder := test.TempDir()
	if err := os.WriteFile(filepath.Join(folder, name), []byte(content), permissions); err != nil {
		test.Fatal(err)
	}
	return &Project{Name: "example", Path: folder}
}

func TestSavingWritesTheWholeFileAtOnce(test *testing.T) {
	project := projectWith(test, "main.go", "package main\n", 0o600)
	read, err := StampOf(project, "main.go")
	if err != nil {
		test.Fatal(err)
	}
	saved, err := Save(project, "main.go", "package main\n\nfunc main() {}\n", read)
	if err != nil {
		test.Fatal(err)
	}
	full := filepath.Join(project.Path, "main.go")
	content, _ := os.ReadFile(full)
	if string(content) != "package main\n\nfunc main() {}\n" {
		test.Errorf("the file holds %q", content)
	}
	if info, _ := os.Stat(full); info.Mode().Perm() != 0o600 {
		test.Errorf("the file lost its permissions: %v", info.Mode().Perm())
	}
	if entries, _ := os.ReadDir(project.Path); len(entries) != 1 {
		test.Errorf("the folder holds %d files, want no temporary one left", len(entries))
	}
	if now, _ := StampOf(project, "main.go"); now != saved {
		test.Error("the stamp returned is not the one of the file written")
	}
}

func TestSavingRefusesAFileChangedOnDisk(test *testing.T) {
	project := projectWith(test, "notes.txt", "one", 0o644)
	read, _ := StampOf(project, "notes.txt")
	os.WriteFile(filepath.Join(project.Path, "notes.txt"), []byte("someone else"), 0o644)
	_, err := Save(project, "notes.txt", "mine", read)
	if !errors.Is(err, ChangedOnDisk) {
		test.Fatalf("saving over a changed file gave %v", err)
	}
	if content, _ := os.ReadFile(filepath.Join(project.Path, "notes.txt")); string(content) != "someone else" {
		test.Errorf("the refused save wrote %q", content)
	}
}

func TestSavingANewFileCreatesIt(test *testing.T) {
	project := &Project{Path: test.TempDir()}
	if _, err := Save(project, "new.txt", "hello", Stamp{}); err != nil {
		test.Fatal(err)
	}
	if content, _ := os.ReadFile(filepath.Join(project.Path, "new.txt")); string(content) != "hello" {
		test.Errorf("the new file holds %q", content)
	}
}
