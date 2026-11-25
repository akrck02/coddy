package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	hash := sha256.Sum256([]byte(content))
	remember(filepath.Join(project.Path, filepath.FromSlash(path)), hash)
	return Stamp{Exists: true, Hash: hash}, nil
}

// saved are the files the reader saved since it started, by path on disk, with their text's hash
// as fingerprints write it: what the person wrote, never a server's.
var saved = struct {
	sync.Mutex
	texts map[string]string
}{texts: map[string]string{}}

func remember(full string, hash [sha256.Size]byte) {
	saved.Lock()
	defer saved.Unlock()
	saved.texts[full] = hex.EncodeToString(hash[:12])
}

// SavedHere tells whether the reader saved a file, as its text is now (a fingerprint's text).
func SavedHere(full, text string) bool {
	saved.Lock()
	defer saved.Unlock()
	return saved.texts[full] == text
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
// followed, so saving writes the file it points at, never over the link, and only inside the
// project; a link to nothing is refused, as there is no file to write but the link itself.
func pathIn(project *Project, path string) (string, error) {
	full := filepath.Join(project.Path, filepath.FromSlash(path))
	if filepath.IsAbs(path) || !inside(project.Path, full) {
		return "", failure.Wrap(OutsideTheProject, path, nil)
	}
	if isDanglingLink(full) {
		return "", failure.Wrap(DanglingLink, path, nil)
	}
	target, err := resolved(full)
	if err != nil {
		return "", failure.Wrap(OutsideTheProject, path, err)
	}
	if target == full {
		return full, nil // no link on the way
	}
	root, err := filepath.EvalSymlinks(project.Path)
	if err != nil || !inside(root, target) {
		return "", failure.Wrap(OutsideTheProject, path, err)
	}
	return target, nil
}

// resolved is a path with its links followed: the file's own, or, for a file not written yet, the
// links of the folders it will be written in.
func resolved(full string) (string, error) {
	target, err := filepath.EvalSymlinks(full)
	if err == nil {
		return target, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	folder, err := resolved(filepath.Dir(full)) // ends at the file system's root, which exists
	if err != nil {
		return "", err
	}
	return filepath.Join(folder, filepath.Base(full)), nil
}

// isDanglingLink tells whether a path is a link that leads to nothing.
func isDanglingLink(full string) bool {
	info, err := os.Lstat(full)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		return false
	}
	_, err = os.Stat(full)
	return errors.Is(err, fs.ErrNotExist)
}

// inside tells whether a path lies inside a folder.
func inside(folder, path string) bool {
	relative, err := filepath.Rel(folder, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
