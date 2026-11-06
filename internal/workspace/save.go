package workspace

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/akrck02/go-renderer/failure"
)

// Stamp is what a file was when it was read, to notice when someone changes it on disk afterwards.
type Stamp struct {
	Exists bool
	Hash   [sha256.Size]byte
}

// StampOf reads what a file of a project is now. A file that does not exist has an empty stamp.
func StampOf(project *Project, path string) (Stamp, error) {
	full, err := pathIn(project, path)
	if err != nil {
		return Stamp{}, err
	}
	content, err := os.ReadFile(full)
	if errors.Is(err, fs.ErrNotExist) {
		return Stamp{}, nil
	}
	if err != nil {
		return Stamp{}, failure.Wrap(NotSaved, path, err)
	}
	return Stamp{Exists: true, Hash: sha256.Sum256(content)}, nil
}

// Save writes a file of a project: the only place the reader writes inside a repository. It writes
// the whole text to a file beside it and renames that over it, so the file is never left half
// written, and keeps its permissions. It refuses when the file changed on disk since it was read,
// as read says, and returns the stamp of what it wrote.
func Save(project *Project, path, content string, read Stamp) (Stamp, error) {
	full, err := pathIn(project, path)
	if err != nil {
		return Stamp{}, err
	}
	now, err := StampOf(project, path)
	if err != nil {
		return Stamp{}, err
	}
	if now != read {
		return Stamp{}, failure.Wrap(ChangedOnDisk, path, nil)
	}
	if err := writeWhole(full, []byte(content)); err != nil {
		return Stamp{}, failure.Wrap(NotSaved, path, err)
	}
	return Stamp{Exists: true, Hash: sha256.Sum256([]byte(content))}, nil
}

// writeWhole writes a file through a temporary one in the same folder, renamed over it at the end.
func writeWhole(full string, content []byte) error {
	permissions := fs.FileMode(0o644)
	if info, err := os.Stat(full); err == nil {
		permissions = info.Mode().Perm()
	}
	temporary, err := os.CreateTemp(filepath.Dir(full), "."+filepath.Base(full)+".saving-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name()) // gone already once renamed; this cleans up a failure
	_, err = temporary.Write(content)
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(temporary.Name(), permissions)
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary.Name(), full)
}

// pathIn is the full path of a file of a project, refusing a path that leads out of it. A link is
// followed, so saving writes the file it points at, never over the link, and only inside the project.
func pathIn(project *Project, path string) (string, error) {
	full := filepath.Join(project.Path, filepath.FromSlash(path))
	if filepath.IsAbs(path) || !inside(project.Path, full) {
		return "", failure.Wrap(OutsideTheProject, path, nil)
	}
	target, err := filepath.EvalSymlinks(full)
	if err != nil || target == full {
		return full, nil // a new file, or not a link
	}
	root, err := filepath.EvalSymlinks(project.Path)
	if err != nil || !inside(root, target) {
		return "", failure.Wrap(OutsideTheProject, path, err)
	}
	return target, nil
}

// inside tells whether a path lies inside a folder.
func inside(folder, path string) bool {
	relative, err := filepath.Rel(folder, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
