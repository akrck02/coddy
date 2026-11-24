package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/go-renderer/failure"
)

// Snapshot is what a project held when a language server started in it: every file git saw, and
// the text of each changed one. A file that changes after it, and that the reader did not save,
// was written by the server.
type Snapshot struct {
	Server  string
	Files   map[string]bool
	Changed map[string]string // the changed files, by the hash of their text
}

// TakeSnapshot notes what a project holds before a server starts in it.
func TakeSnapshot(project *Project, server string) Snapshot {
	snapshot := Snapshot{Server: server, Files: map[string]bool{}, Changed: map[string]string{}}
	for _, path := range project.Files {
		snapshot.Files[path] = true
	}
	for path, fingerprint := range project.Fingerprints {
		snapshot.Changed[path] = textOf(fingerprint)
	}
	return snapshot
}

// textOf is the part of a fingerprint that names the file's text, without the base.
func textOf(fingerprint string) string {
	return fingerprint[strings.LastIndexByte(fingerprint, ':')+1:]
}

// ServerFile is a file a language server wrote in a project.
type ServerFile struct {
	Path   string
	Server string
	Text   string // the hash of its text when it was found
	// Existed tells that the file was there before the server started and changed while it ran:
	// it may be the person's, changed outside the reader.
	Existed bool `json:",omitempty"`
}

// Written keeps the files language servers wrote, by project, in a file of the reader's own, so a
// file a server wrote stays the server's after the reader starts again.
type Written struct {
	path  string
	mutex sync.Mutex
	files map[string]map[string]ServerFile // project → file → what wrote it
}

// OpenWritten reads what servers wrote from a file; a missing or damaged file starts empty. An
// empty path keeps nothing on disk (tests).
func OpenWritten(path string) *Written {
	written := &Written{path: path, files: map[string]map[string]ServerFile{}}
	if path == "" {
		return written
	}
	if content, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(content, &written.files) // a broken file starts empty: what servers wrote is a hint, kept again on close
	}
	if written.files == nil {
		written.files = map[string]map[string]ServerFile{}
	}
	return written
}

// DefaultWrittenPath is where what servers wrote is kept: in the user's configuration folder.
func DefaultWrittenPath() string {
	folder, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(folder, "code-review", "servers-wrote.json")
}

// SortOut takes what servers wrote out of a project just read: out of its changes, its counts and
// its fingerprints, into ServerFiles. Only a file that differs from the last commit on disk, and
// that the reader did not save, can be a server's: one recorded as the server's while it keeps the
// text the server left (or while a server runs in the project), or one that changed since the
// snapshot (nil when no server started in the project). Anything else stays the person's: a
// commit, a pull or another base never hides a change. A project read without the changes not
// committed yet holds no server's file.
func (written *Written) SortOut(project *Project, snapshot *Snapshot, workingTree bool) error {
	project.ServerFiles = nil
	written.mutex.Lock()
	defer written.mutex.Unlock()
	known := written.files[project.Path]
	if !workingTree || (snapshot == nil && len(known) == 0) {
		return nil
	}
	onDisk := notCommitted(project.Path)
	found := map[string]ServerFile{}
	for _, change := range project.Changes {
		fingerprint, hashed := project.Fingerprints[change.Path]
		if !hashed || change.Status == git.Deleted || !onDisk[change.Path] {
			continue // a server adds and rewrites files; a file gone, or committed, is the person's
		}
		if file, isServers := whoWrote(project, change.Path, textOf(fingerprint), known, snapshot); isServers {
			found[change.Path] = file
		}
	}
	project.leave(found)
	if sameFiles(known, found) {
		return nil
	}
	if len(found) == 0 {
		delete(written.files, project.Path)
	} else {
		written.files[project.Path] = found
	}
	return written.keep()
}

// notCommitted are the files of a repository that differ from its last commit on disk, or that git
// does not know yet; none when git cannot tell, so nothing is taken for a server's.
func notCommitted(repository string) map[string]bool {
	changes, err := git.Changes(context.Background(), repository, git.Comparison{MergeBase: "HEAD", WorkingTree: true})
	paths := map[string]bool{}
	if err != nil {
		return paths
	}
	for _, change := range changes {
		paths[change.Path] = true
	}
	return paths
}

// whoWrote tells whether a changed file is a server's, and how.
func whoWrote(project *Project, path, text string, known map[string]ServerFile, snapshot *Snapshot) (ServerFile, bool) {
	if SavedHere(filepath.Join(project.Path, filepath.FromSlash(path)), text) {
		return ServerFile{}, false // the person saved it in the reader
	}
	if file, recorded := known[path]; recorded && (file.Text == text || snapshot != nil) {
		file.Text = text // a server running rewrites its own files
		return file, true
	}
	if snapshot == nil || snapshot.Changed[path] == text {
		return ServerFile{}, false
	}
	_, wasChanged := snapshot.Changed[path]
	existed := wasChanged || snapshot.Files[path]
	return ServerFile{Path: path, Server: snapshot.Server, Text: text, Existed: existed}, true
}

// sameFiles tells whether two records hold the same files, as the same texts by the same servers.
func sameFiles(one, other map[string]ServerFile) bool {
	if len(one) != len(other) {
		return false
	}
	for path, file := range one {
		if other[path] != file {
			return false
		}
	}
	return true
}

// keep writes the record to its file.
func (written *Written) keep() error {
	if written.path == "" {
		return nil
	}
	content, err := json.MarshalIndent(written.files, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(written.path), 0o755)
	}
	if err == nil {
		err = os.WriteFile(written.path, content, 0o644)
	}
	if err != nil {
		return failure.Wrap(RecordNotKept, written.path, err)
	}
	return nil
}

// leave moves the files a server wrote out of the project's changes into ServerFiles.
func (project *Project) leave(found map[string]ServerFile) {
	if len(found) == 0 {
		return
	}
	kept := make([]git.Change, 0, len(project.Changes))
	for _, change := range project.Changes {
		if file, isServers := found[change.Path]; isServers {
			project.ServerFiles = append(project.ServerFiles, file)
			delete(project.Counts, change.Path)
			delete(project.Fingerprints, change.Path)
			continue
		}
		kept = append(kept, change)
	}
	project.setChanges(kept)
	sort.Slice(project.ServerFiles, func(first, second int) bool {
		return project.ServerFiles[first].Path < project.ServerFiles[second].Path
	})
}
