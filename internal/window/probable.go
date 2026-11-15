package window

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akrck02/code-review/internal/lsp"
)

// A place a server has not confirmed yet, its server still reading the projects for minutes, or
// none reading its file, is a probable use when its file names the declaration for sure: it
// imports it by its whole name, or all of its package, or is in its package. It is shown as
// probable at once, and becomes a use, or goes, once its server answers.

// packageLine is a Java or Kotlin file's package.
var packageLine = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)`)

// packageOf is the package a file's text declares, "" for none.
func packageOf(text string) string {
	if found := packageLine.FindStringSubmatch(text); found != nil {
		return found[1]
	}
	return ""
}

// namesDeclaration tells whether a file's text names a declaration of a package for sure: it
// imports it, or its whole package, or is in that package.
func namesDeclaration(text, declaredIn, name string) bool {
	if declaredIn == "" {
		return false
	}
	if packageOf(text) == declaredIn {
		return true
	}
	imported := regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?` + regexp.QuoteMeta(declaredIn) +
		`\.(?:` + regexp.QuoteMeta(name) + `\b|\*)`)
	return imported.MatchString(text)
}

// markProbable keeps, of the places to check, those whose file names the declaration for sure,
// reading each file once.
func markProbable(search *usesSearch) {
	search.probable = map[string]lsp.Location{}
	declared, err := os.ReadFile(lsp.PathOf(search.declaration.URI))
	if err != nil {
		return
	}
	declaredIn := packageOf(string(declared))
	names := map[string]bool{}
	for _, place := range search.pending {
		if _, read := names[place.path]; !read {
			text, _ := os.ReadFile(place.path)
			names[place.path] = isJavaOrKotlin(place.path) && namesDeclaration(string(text), declaredIn, search.name)
		}
		if names[place.path] {
			location := placeLocation(place, search.name)
			search.probable[placeKey(location)] = location
		}
	}
}

// isJavaOrKotlin tells whether a file is a Java or Kotlin source, whose imports are read.
func isJavaOrKotlin(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".java" || extension == ".kt" || extension == ".kts"
}

// placeLocation is a place of a file, as long as the name.
func placeLocation(place candidate, name string) lsp.Location {
	end := place.at
	end.Character += len([]rune(name))
	return lsp.Location{URI: lsp.FileURI(place.path), Range: lsp.Range{Start: place.at, End: end}}
}
