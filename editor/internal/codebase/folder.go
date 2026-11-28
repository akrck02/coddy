package codebase

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/akrck02/coddy/editor/internal/git"
)

// FolderControl reads a folder no version control knows: every file on disk, and no change. It
// lets a person open any folder, as other editors do. LeftOut are folders inside it, by their
// paths with slashes, whose files it does not list: the repositories it holds, each a project of
// its own.
type FolderControl struct {
	LeftOut []string
}

// folderFileLimit is how many files a plain folder lists at most: opening a home folder by
// mistake should not walk every file on the disk.
const folderFileLimit = 50000

// folderBase stands for the base of a plain folder, which has none: with a base, the project is
// read without the no-base error, and with no version control it shows no change anyway.
const folderBase = "disk"

func (*FolderControl) Branch(string) (string, error) { return "", nil }

func (*FolderControl) DefaultBase(string) (string, error) { return folderBase, nil }

// Files are the files under the folder, by their paths with slashes, leaving out hidden files and
// folders and those left out; at most folderFileLimit of them.
func (control *FolderControl) Files(stop context.Context, folder string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // a folder that cannot be read is left out, as a hidden one is
		}
		if stop.Err() != nil || len(files) >= folderFileLimit {
			return filepath.SkipAll
		}
		if path == folder {
			return nil
		}
		relative, _ := filepath.Rel(folder, path) // path is under folder, so it always has one
		relative = filepath.ToSlash(relative)
		if strings.HasPrefix(entry.Name(), ".") || slices.Contains(control.LeftOut, relative) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() {
			files = append(files, relative)
		}
		return nil
	})
	return files, err
}

func (*FolderControl) Changes(context.Context, string, git.Comparison) ([]git.Change, error) {
	return nil, nil
}

// ReadAt is the file on disk: a plain folder keeps no other revision.
func (*FolderControl) ReadAt(folder, _, path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(folder, filepath.FromSlash(path)))
}

func (*FolderControl) Diff(string, git.Comparison, git.Change) (git.FileDiff, error) {
	return git.FileDiff{}, nil
}

// IsPlainFolder tells whether the project is a plain folder, read with no version control.
func (project *Project) IsPlainFolder() bool {
	_, plain := project.Control.(*FolderControl)
	return plain
}
