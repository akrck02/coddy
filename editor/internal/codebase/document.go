package codebase

import (
	"bytes"
	"crypto/sha256"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/akrck02/coddy/editor/internal/format/highlight"
	"github.com/akrck02/coddy/editor/internal/git"
)

// largestShown is the largest file read into the viewer.
const largestShown = 16 << 20

// Document is a file opened for reading: its text on both sides of the development, coloured, and
// its diff when it changed.
type Document struct {
	Project string // the project's path
	Path    string
	Change  git.Change
	Changed bool

	Language string
	OldLines []highlight.Line // as it was at the base (empty for a new file)
	NewLines []highlight.Line // as it is now (empty for a deleted file)
	Diff     []git.DiffLine   // every line of both sides, in order, when it changed

	Binary bool
	// Error is why the file cannot be shown, with its code; nil when it can
	Error error

	// Editable tells whether the file can be edited: read from disk, as UTF-8 text. Text is then
	// the file as read, and Stamp what it was, so saving can tell whether it changed since.
	Editable bool
	Text     string
	Stamp    Stamp
}

// OpenDocument reads a file of a project: as it is now, and, if the development changed it, as it
// was at the base and how the two differ.
func OpenDocument(project *Project, path string, workingTree bool) *Document {
	document := &Document{Project: project.Path, Path: path}
	workingTree = project.Comparison(workingTree).OnDisk() // a pull request's head is not on disk
	change, changed := project.ChangeOf(path)
	document.Change, document.Changed = change, changed && project.MergeBase != ""

	newContent, newError := readCurrent(project, path, workingTree, change)
	if document.Changed && change.Status == git.Deleted {
		newContent, newError = nil, nil
	}
	if newError != nil {
		document.Error = newError
	}
	oldContent := document.readBase(project)
	if isBinary(newContent) || isBinary(oldContent) {
		document.Binary = true
		return document
	}
	if len(newContent) > largestShown || len(oldContent) > largestShown {
		document.Error = TooLarge
		return document
	}
	sample := newContent
	if len(sample) == 0 {
		sample = oldContent
	}
	document.keepForEditing(newContent, newError == nil && (workingTree || change.Untracked))
	document.Language = highlight.Language(path, asText(sample))
	document.NewLines = highlight.Lines(path, asText(newContent))
	document.OldLines = highlight.Lines(change.OldPath, asText(oldContent))
	if document.Changed {
		document.Diff = document.diffLines(project, workingTree)
	}
	return document
}

// readBase reads the file as it was at the base, when the development changed it and it was there.
func (document *Document) readBase(project *Project) []byte {
	if !document.Changed || document.Change.Status == git.Added {
		return nil
	}
	content, err := project.control().ReadAt(project.Path, project.MergeBase, document.Change.OldPath)
	if err != nil && document.Error == nil {
		document.Error = err
	}
	return content
}

// keepForEditing keeps the text read to edit it, when it was read from disk and is UTF-8.
func (document *Document) keepForEditing(content []byte, fromDisk bool) {
	deleted := document.Changed && document.Change.Status == git.Deleted
	if !fromDisk || deleted || !utf8.Valid(content) {
		return
	}
	document.Editable, document.Text = true, string(content)
	document.Stamp = Stamp{Exists: true, Hash: sha256.Sum256(content)}
}

// readCurrent reads a file as the development has it: on disk, or in the last commit (or the
// project's Head) when changes not committed yet are left out.
func readCurrent(project *Project, path string, workingTree bool, change git.Change) ([]byte, error) {
	if workingTree || change.Untracked {
		return git.ReadWorkingFile(project.Path, path)
	}
	return project.control().ReadAt(project.Path, project.Comparison(false).Last(), path)
}

// diffLines is the file's diff: from the version control for a file on both sides, made here for a file only on one.
func (document *Document) diffLines(project *Project, workingTree bool) []git.DiffLine {
	switch {
	case document.Change.Status == git.Added || document.Change.Untracked:
		return onlyOneSide(len(document.NewLines), git.Insertion)
	case document.Change.Status == git.Deleted:
		return onlyOneSide(len(document.OldLines), git.Removal)
	}
	diff, err := project.control().Diff(project.Path, project.Comparison(workingTree), document.Change)
	if err != nil {
		document.Error = err
		return nil
	}
	if diff.Binary {
		document.Binary = true
	}
	return diff.Lines
}

func onlyOneSide(count int, kind git.LineKind) []git.DiffLine {
	lines := make([]git.DiffLine, count)
	for index := range lines {
		lines[index].Kind = kind
		if kind == git.Insertion {
			lines[index].NewNumber = index + 1
		} else {
			lines[index].OldNumber = index + 1
		}
	}
	return lines
}

// isBinary guesses whether a file is not text, as git does: a zero byte early on.
func isBinary(content []byte) bool {
	start := content
	if len(start) > 8000 {
		start = start[:8000]
	}
	return bytes.IndexByte(start, 0) >= 0
}

// asText reads a file's bytes as text: UTF-8, or else Latin-1, which older Java property files and
// sources are often written in and which any byte is valid in.
func asText(content []byte) string {
	if utf8.Valid(content) {
		return string(content)
	}
	runes := make([]rune, len(content))
	for index, character := range content {
		runes[index] = rune(character)
	}
	return string(runes)
}

// FileReference names a file of the workspace: its project, by its place in the list of projects under
// review, and its path in that project.
type FileReference struct {
	Project int
	Path    string
}

// LinesOfFile are the lines of a file on disk, without their endings; none when it cannot be read.
func LinesOfFile(path string) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}
