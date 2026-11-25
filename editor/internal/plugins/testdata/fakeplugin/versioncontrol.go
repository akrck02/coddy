package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/akrck02/coddy/sdk"
)

// The fake version control marks its repositories with a .fakevc folder, which holds the files
// as its only revision, "base", has them. It has no commits of its own: the last commit is the
// disk. A repository whose .fakevc holds a file "crash" ends the plugin when asked its changes.
const (
	marker = ".fakevc"
	base   = "base"
)

type fakeControl struct{}

func (fakeControl) Branch(string) (string, error)      { return "trunk", nil }
func (fakeControl) DefaultBase(string) (string, error) { return base, nil }

func (fakeControl) Files(folder string) ([]string, error) { return filesIn(folder) }

func (fakeControl) Changes(asked sdk.CompareParams) ([]sdk.Change, error) {
	if _, err := os.Stat(filepath.Join(asked.Folder, marker, "crash")); err == nil {
		os.Exit(3)
	}
	before, err := filesIn(filepath.Join(asked.Folder, marker, base))
	if err != nil {
		return nil, err
	}
	now, err := filesIn(asked.Folder)
	if err != nil {
		return nil, err
	}
	var changes []sdk.Change
	for _, path := range now {
		old, err := os.ReadFile(filepath.Join(asked.Folder, marker, base, path))
		if err != nil {
			changes = append(changes, sdk.Change{Status: "A", Path: path})
			continue
		}
		current, err := os.ReadFile(filepath.Join(asked.Folder, path))
		if err == nil && string(current) != string(old) {
			changes = append(changes, sdk.Change{Status: "M", Path: path})
		}
	}
	for _, path := range before {
		if _, err := os.Stat(filepath.Join(asked.Folder, path)); err != nil {
			changes = append(changes, sdk.Change{Status: "D", Path: path})
		}
	}
	return changes, nil
}

func (fakeControl) Read(asked sdk.ReadParams) (string, error) {
	folder := asked.Folder
	if asked.Revision == base {
		folder = filepath.Join(folder, marker, base)
	}
	content, err := os.ReadFile(filepath.Join(folder, asked.Path))
	return string(content), err
}

// Diff is every line of the base removed and every line now added: a true diff, if not a short one.
func (control fakeControl) Diff(asked sdk.DiffParams) (string, error) {
	old, err := control.Read(sdk.ReadParams{Folder: asked.Folder, Revision: base, Path: asked.OldPath})
	if err != nil {
		return "", err
	}
	current, err := control.Read(sdk.ReadParams{Folder: asked.Folder, Revision: sdk.LastCommit, Path: asked.Path})
	if err != nil {
		return "", err
	}
	oldLines, newLines := lines(old), lines(current)
	var diff strings.Builder
	fmt.Fprintf(&diff, "--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n", asked.OldPath, asked.Path, len(oldLines), len(newLines))
	for _, line := range oldLines {
		diff.WriteString("-" + line + "\n")
	}
	for _, line := range newLines {
		diff.WriteString("+" + line + "\n")
	}
	return diff.String(), nil
}

func lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// filesIn lists the files under a folder, by their paths from it, leaving out the marker.
func filesIn(folder string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == marker {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			relative, _ := filepath.Rel(folder, path) // path is under folder, so it always has one
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	return files, err
}
