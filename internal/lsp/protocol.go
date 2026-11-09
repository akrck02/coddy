// Package lsp talks to language servers (gopls, jdtls) over the Language Server Protocol: JSON-RPC
// 2.0 messages on a server's standard input and output. It knows nothing of the window.
package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
)

// Position is a place in a text document, as the protocol counts it: a line and a character,
// both from zero, the character in UTF-16 code units.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range runs from a start to an end position.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range in a document, named by its URI.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier names a document by its URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams are a place in a document: what definition, references and hover ask about.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// FileURI is the file:// URI of a path on disk.
func FileURI(path string) string {
	address := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	return address.String()
}

// PathOf is the path on disk of a file:// URI, "" for any other URI (a jar's, say).
func PathOf(uri string) string {
	address, err := url.Parse(uri)
	if err != nil || address.Scheme != "file" {
		return ""
	}
	return filepath.FromSlash(address.Path)
}

// ClassInJar reads a location in a jar: its package and the class, the outer one of an inner
// class, ("org.lighthouse.lamp", "Lamp"). jdtls gives jdt://contents/<jar>/<package>/Lamp.java,
// kotlin-lsp jar:///<jar>!/org/lighthouse/lamp/Lamp.class.
func ClassInJar(uri string) (pkg, class string, found bool) {
	var file string
	if rest, isJdt := strings.CutPrefix(uri, "jdt://contents/"); isJdt {
		rest, _, _ = strings.Cut(rest, "?")
		parts := strings.Split(rest, "/")
		if len(parts) < 3 {
			return "", "", false
		}
		pkg, file = parts[len(parts)-2], parts[len(parts)-1]
	} else if _, inside, isJar := strings.Cut(uri, "!/"); isJar && strings.HasPrefix(uri, "jar:") {
		slash := strings.LastIndex(inside, "/")
		if slash < 0 {
			return "", "", false
		}
		pkg, file = strings.ReplaceAll(inside[:slash], "/", "."), inside[slash+1:]
	}
	class, _, _ = strings.Cut(strings.TrimSuffix(strings.TrimSuffix(file, ".class"), ".java"), "$")
	if pkg == "" || class == "" {
		return "", "", false
	}
	return pkg, class, true
}

// UTF16Column is a column counted in characters as the protocol counts it, in UTF-16 units.
func UTF16Column(line string, characters int) int {
	units := 0
	for _, character := range line {
		if characters == 0 {
			break
		}
		characters--
		units++
		if character >= 0x10000 {
			units++ // outside the basic plane: two units
		}
	}
	return units
}

// CharacterColumn is a column in UTF-16 units turned back into characters.
func CharacterColumn(line string, units int) int {
	characters := 0
	for _, character := range line {
		if units <= 0 {
			break
		}
		units--
		if character >= 0x10000 {
			units--
		}
		characters++
	}
	return characters
}

// languageIDs are the protocol's names of the languages the reader asks servers about, by extension.
var languageIDs = map[string]string{".go": "go", ".java": "java", ".kt": "kotlin", ".kts": "kotlin"}

// LanguageOf is the protocol's name of a file's language, "" when no server is known for it.
func LanguageOf(path string) string {
	return languageIDs[strings.ToLower(filepath.Ext(path))]
}
