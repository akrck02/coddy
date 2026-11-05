package review

import (
	"bytes"
	"unicode/utf8"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
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
	Error  string
}

// OpenDocument reads a file of a project: as it is now, and, if the development changed it, as it
// was at the base and how the two differ.
func OpenDocument(project *Project, path string, workingTree bool) *Document {
	document := &Document{Project: project.Path, Path: path}
	change, changed := project.ChangeOf(path)
	document.Change, document.Changed = change, changed && project.MergeBase != ""

	newContent, newError := readCurrent(project, path, workingTree, change)
	if document.Changed && change.Status == git.Deleted {
		newContent, newError = nil, nil
	}
	if newError != nil {
		document.Error = newError.Error()
	}
	var oldContent []byte
	if document.Changed && change.Status != git.Added {
		var err error
		oldContent, err = git.ReadAtRevision(project.Path, project.MergeBase, change.OldPath)
		if err != nil && document.Error == "" {
			document.Error = err.Error()
		}
	}
	if isBinary(newContent) || isBinary(oldContent) {
		document.Binary = true
		return document
	}
	if len(newContent) > largestShown || len(oldContent) > largestShown {
		document.Error = "the file is too large to show"
		return document
	}
	sample := newContent
	if len(sample) == 0 {
		sample = oldContent
	}
	document.Language = highlight.Language(path, asText(sample))
	document.NewLines = highlight.Lines(path, asText(newContent))
	document.OldLines = highlight.Lines(change.OldPath, asText(oldContent))
	if document.Changed {
		document.Diff = document.diffLines(project, workingTree)
	}
	return document
}

// readCurrent reads a file as the development has it: on disk, or in the last commit when changes
// not committed yet are left out.
func readCurrent(project *Project, path string, workingTree bool, change git.Change) ([]byte, error) {
	if workingTree || change.Untracked {
		return git.ReadWorkingFile(project.Path, path)
	}
	return git.ReadAtRevision(project.Path, "HEAD", path)
}

// diffLines is the file's diff: from git for a file on both sides, made here for a file only on one.
func (document *Document) diffLines(project *Project, workingTree bool) []git.DiffLine {
	switch {
	case document.Change.Status == git.Added || document.Change.Untracked:
		return onlyOneSide(len(document.NewLines), git.Insertion)
	case document.Change.Status == git.Deleted:
		return onlyOneSide(len(document.OldLines), git.Removal)
	}
	diff, err := git.Diff(project.Path, project.Comparison(workingTree), document.Change)
	if err != nil {
		document.Error = err.Error()
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
