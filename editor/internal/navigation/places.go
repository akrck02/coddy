package navigation

import (
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/lsp"
)

// ReferenceOf is the file of a project open at a path on disk: in the project nearest to it, as
// projects may hold others.
func ReferenceOf(projects []*codebase.Project, path string) (codebase.FileReference, bool) {
	best, bestLength := -1, 0
	for index, project := range projects {
		relative, err := filepath.Rel(project.Path, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if len(project.Path) > bestLength {
			best, bestLength = index, len(project.Path)
		}
	}
	if best < 0 {
		return codebase.FileReference{}, false
	}
	relative, _ := filepath.Rel(projects[best].Path, path) // it was worked out above without an error
	return codebase.FileReference{Project: best, Path: filepath.ToSlash(relative)}, true
}

// Declares tells whether a location is the declaration at a place of a file: the name asked about
// is the one declared there.
func Declares(location lsp.Location, path string, at lsp.Position) bool {
	start, end := location.Range.Start, location.Range.End
	return lsp.PathOf(location.URI) == path && start.Line == at.Line && end.Line == at.Line &&
		start.Character <= at.Character && at.Character <= end.Character
}

// OutsideName names a declaration outside the projects open: its file, or the class in a jar.
func OutsideName(uri, path string) string {
	if packageName, class, isJar := lsp.ClassInJar(uri); isJar {
		return packageName + "." + class + ", in a jar whose project is not open"
	}
	if path == "" {
		return uri
	}
	return path
}

// isNamePart tells whether a character may be part of a name: letters, digits and _.
func isNamePart(character rune) bool {
	return character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character)
}

// WordAt is the name a character of a line is in: letters, digits and _.
func WordAt(line string, column int) string {
	characters := []rune(line)
	start, end := min(max(column, 0), len(characters)), min(max(column, 0), len(characters))
	for start > 0 && isNamePart(characters[start-1]) {
		start--
	}
	for end < len(characters) && isNamePart(characters[end]) {
		end++
	}
	return string(characters[start:end])
}

// wordColumns are where a name stands as a whole word in a line, in bytes: git grep gives the
// first only.
func wordColumns(line, name string) []int {
	var columns []int
	for from := 0; from <= len(line)-len(name); {
		at := strings.Index(line[from:], name)
		if at < 0 {
			break
		}
		at += from
		before := at > 0 && isNameBefore(line, at)
		after := at+len(name) < len(line) && isNameAt(line, at+len(name))
		if !before && !after {
			columns = append(columns, at)
		}
		from = at + len(name)
	}
	return columns
}

// isNameAt tells whether the character at a byte of a line is part of a name.
func isNameAt(line string, at int) bool {
	character, _ := utf8.DecodeRuneInString(line[at:])
	return isNamePart(character)
}

// isNameBefore tells whether the character before a byte of a line is part of a name.
func isNameBefore(line string, at int) bool {
	character, _ := utf8.DecodeLastRuneInString(line[:at])
	return isNamePart(character)
}

// placeKey names a place by its file, line and character.
func placeKey(location lsp.Location) string {
	start := location.Range.Start
	return lsp.PathOf(location.URI) + ":" + strconv.Itoa(start.Line) + ":" + strconv.Itoa(start.Character)
}
