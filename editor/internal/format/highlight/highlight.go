// Package highlight splits source code into coloured pieces, line by line, with chroma: the same
// lexers papiro-indexer uses for its code blocks, but kept as tokens to paint instead of HTML.
package highlight

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/akrck02/coddy/editor/internal/language"
)

// TabWidth is how many columns a tab takes.
const TabWidth = 4

// largestHighlighted is the largest file that is coloured; larger files are shown plain, as
// colouring them would take longer than reading them is worth.
const largestHighlighted = 2 << 20

// Span is a piece of a line in one colour: a keyword, a string, a run of spaces.
type Span struct {
	Column int    // where it starts, counting tabs as TabWidth columns
	Text   string // with tabs turned into spaces
	Kind   chroma.TokenType
}

// Line is a line of code as its coloured pieces.
type Line struct {
	Spans   []Span
	Columns int // how wide the line is
}

// Text is the line's text, with tabs turned into spaces.
func (line Line) Text() string {
	var builder strings.Builder
	for _, span := range line.Spans {
		builder.WriteString(span.Text)
	}
	return builder.String()
}

// Language names the language a file is coloured as, from its name and, failing that, its start.
func Language(fileName string, content string) string {
	lexer := lexerFor(fileName, content)
	if lexer == nil {
		return "Plain text"
	}
	return lexer.Config().Name
}

func lexerFor(fileName string, content string) chroma.Lexer {
	base := filepath.Base(fileName)
	if lexer := lexers.Match(base); lexer != nil {
		return lexer
	}
	if known, found := language.Of(base); found && known.Lexer != "" {
		if lexer := lexers.Get(known.Lexer); lexer != nil {
			return lexer
		}
	}
	start := content
	if len(start) > 4096 {
		start = start[:4096]
	}
	return lexers.Analyse(start)
}

// Lines colours a file and splits it into lines.
func Lines(fileName string, content string) []Line {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lexer := lexerFor(fileName, content)
	if lexer == nil || len(content) > largestHighlighted {
		return plainLines(content)
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, content)
	if err != nil {
		return plainLines(content)
	}
	builder := lineBuilder{}
	for _, token := range iterator.Tokens() {
		builder.add(token.Value, token.Type)
	}
	return builder.finish(content)
}

// plainLines splits a file into lines of a single uncoloured span each.
func plainLines(content string) []Line {
	builder := lineBuilder{}
	builder.add(content, chroma.Text)
	return builder.finish(content)
}

// lineBuilder cuts tokens at new lines and turns tabs into spaces.
type lineBuilder struct {
	lines   []Line
	current Line
}

func (builder *lineBuilder) add(text string, kind chroma.TokenType) {
	for {
		newLine := strings.IndexByte(text, '\n')
		if newLine < 0 {
			builder.addPiece(text, kind)
			return
		}
		builder.addPiece(text[:newLine], kind)
		builder.lines = append(builder.lines, builder.current)
		builder.current = Line{}
		text = text[newLine+1:]
	}
}

func (builder *lineBuilder) addPiece(text string, kind chroma.TokenType) {
	if text == "" {
		return
	}
	start := builder.current.Columns
	expanded, columns := expandTabs(text, start)
	builder.current.Spans = append(builder.current.Spans, Span{Column: start, Text: expanded, Kind: kind})
	builder.current.Columns = columns
}

// finish ends the last line. A file that ends with a new line has no empty line after it.
func (builder *lineBuilder) finish(content string) []Line {
	if !strings.HasSuffix(content, "\n") || content == "" {
		builder.lines = append(builder.lines, builder.current)
	}
	if content == "" {
		return nil
	}
	return builder.lines
}

// expandTabs turns tabs into spaces up to the next tab stop and drops characters that cannot be
// shown (a carriage return), returning the text and the column after it.
func expandTabs(text string, column int) (string, int) {
	if !strings.ContainsAny(text, "\t\r\v\f") {
		return text, column + utf8.RuneCountInString(text)
	}
	var builder strings.Builder
	for _, character := range text {
		switch character {
		case '\t':
			spaces := TabWidth - column%TabWidth
			builder.WriteString(strings.Repeat(" ", spaces))
			column += spaces
		case '\r', '\v', '\f':
		default:
			builder.WriteRune(character)
			column++
		}
	}
	return builder.String(), column
}

// Uncoloured is a line of text in one colour, with its tabs turned into spaces: a line just
// edited, before it is coloured again.
func Uncoloured(text string) Line {
	expanded, columns := expandTabs(text, 0)
	if expanded == "" {
		return Line{}
	}
	return Line{Spans: []Span{{Text: expanded, Kind: chroma.Text}}, Columns: columns}
}

// Column is where a character of a line starts on screen, counting tabs to their stop.
func Column(line string, characters int) int {
	column := 0
	for _, character := range line {
		if characters == 0 {
			break
		}
		characters--
		if character == '\t' {
			column += TabWidth - column%TabWidth
		} else {
			column++
		}
	}
	return column
}

// CharacterAt is the character of a line nearest a place on screen, in columns: the inverse of
// Column, rounding to the nearer edge of a character.
func CharacterAt(line string, place float32) int {
	column, characters := 0, 0
	for _, character := range line {
		width := 1
		if character == '\t' {
			width = TabWidth - column%TabWidth
		}
		if place < float32(column)+float32(width)/2 {
			return characters
		}
		column += width
		characters++
	}
	return characters
}

// InComment tells whether the character at a column of a coloured line is in a comment.
func (line Line) InComment(column int) bool {
	for _, span := range line.Spans {
		if column >= span.Column && column < span.Column+len([]rune(span.Text)) {
			role := RoleOf(span.Kind)
			return role == Comment || role == DocComment
		}
	}
	return false
}
