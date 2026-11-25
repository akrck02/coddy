package navigation

import (
	"os"
	"strings"

	"github.com/akrck02/coddy/editor/internal/highlight"
	"github.com/akrck02/coddy/editor/internal/lsp"
)

// commentReader tells which places of files are in comments, colouring each file once.
type commentReader map[string]commented

// commented is a file read, as text and as coloured lines.
type commented struct {
	text     []string
	coloured []highlight.Line
}

// references are what comes before a name in a comment that links to it, as a use does: Javadoc's
// {@link Name} and @see, KDoc's [Name].
var references = []string{"{@link ", "{@linkplain ", "@see ", "@throws ", "@exception ", "["}

// mentionOnly tells whether a place is in a comment that only mentions the name, in words: a
// server may answer where a word of a comment is declared, but a word is not a use.
func (reader commentReader) mentionOnly(place candidate) bool {
	file := reader.read(place.path)
	if place.at.Line >= len(file.text) || place.at.Line >= len(file.coloured) {
		return false
	}
	line := file.text[place.at.Line]
	characters := lsp.CharacterColumn(line, place.at.Character)
	if !file.coloured[place.at.Line].InComment(highlight.Column(line, characters)) {
		return false
	}
	before := string([]rune(line)[:characters])
	for _, reference := range references {
		if strings.HasSuffix(before, reference) {
			return false
		}
	}
	return true
}

// read is a file as text and coloured lines, read the first time it is asked for.
func (reader commentReader) read(path string) commented {
	if file, read := reader[path]; read {
		return file
	}
	content, _ := os.ReadFile(path) // a file gone has no comments: its places are asked about
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	file := commented{text: strings.Split(text, "\n"), coloured: highlight.Lines(path, text)}
	reader[path] = file
	return file
}
