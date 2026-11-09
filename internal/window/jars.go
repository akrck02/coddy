package window

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/workspace"
)

// fromJar turns a location in a jar (jdtls's jdt://) into the class's source, when a project open
// holds it: in the big workspace every repository is a dependency of the others as a jar, and its source is
// what the person reads. The line is where the name is declared in the source, as the jar's lines
// need not be the source's.
func (application *Window) fromJar(location lsp.Location, name string) (lsp.Location, bool) {
	pkg, file, isJar := lsp.ClassInJar(location.URI)
	if !isJar {
		return location, false
	}
	source, found := application.sourceOf(strings.ReplaceAll(pkg, ".", "/") + "/" + file)
	if !found {
		return location, false
	}
	lines := workspace.LinesOfFile(source)
	line := declarationLine(lines, name, location.Range.Start.Line)
	column := 0
	if line < len(lines) {
		column = max(wordColumn(lines[line], name), 0)
	}
	at := lsp.Position{Line: line, Character: lsp.UTF16Column(lines[min(line, max(len(lines)-1, 0))], column)}
	return lsp.Location{URI: lsp.FileURI(source), Range: lsp.Range{Start: at, End: at}}, true
}

// sourceOf is the file on disk of a project open whose path ends with a class's path: the main
// sources first, then any other.
func (application *Window) sourceOf(classPath string) (string, bool) {
	var other string
	for _, project := range application.projects {
		for _, file := range project.Files {
			if file != classPath && !strings.HasSuffix(file, "/"+classPath) {
				continue
			}
			full := filepath.Join(project.Path, filepath.FromSlash(file))
			if strings.Contains(file, "src/main/") {
				return full, true
			}
			if other == "" {
				other = full
			}
		}
	}
	return other, other != ""
}

// declarationLine is the line, from 0, where a name is declared in a Java source: a type
// declared with it, else the jar's line when it holds the name, else the first line past the
// imports that holds it, else the jar's line.
func declarationLine(lines []string, name string, jarLine int) int {
	typeDeclared := regexp.MustCompile(`\b(class|interface|enum|record|@interface)\s+` + regexp.QuoteMeta(name) + `\b`)
	for number, line := range lines {
		if typeDeclared.MatchString(line) {
			return number
		}
	}
	if jarLine < len(lines) && wordColumn(lines[jarLine], name) >= 0 {
		return jarLine
	}
	for number, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "package ") {
			continue
		}
		if wordColumn(line, name) >= 0 {
			return number
		}
	}
	return min(jarLine, max(len(lines)-1, 0))
}

// wordColumn is the character where a name first stands as a whole word in a line; -1 if nowhere.
func wordColumn(line, name string) int {
	if name == "" {
		return -1
	}
	whole := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	at := whole.FindStringIndex(line)
	if at == nil {
		return -1
	}
	return len([]rune(line[:at[0]]))
}
