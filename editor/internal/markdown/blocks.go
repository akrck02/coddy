package markdown

import (
	"regexp"
	"strings"
)

// Kind is what a block is.
type Kind int

const (
	Paragraph Kind = iota
	Heading
	Item  // a list's item
	Quote // a paragraph of a quote
	Code  // a fenced block of code, its lines as they are
	Rule
	Table
)

// Block is a part of a Markdown text, as it is shown: a paragraph, a heading, a list's item…
type Block struct {
	Kind  Kind
	Level int    // a heading's level, from 1; how deep an item is, from 0
	Mark  string // an item's bullet or number: "•", "1.", "☐" or "☑"
	Spans []Span // the words of a paragraph, heading, item or quote
	Lines []string
	// Language is what a block of code is written in, as its fence says
	Language string
	Rows     [][][]Span // a table's cells, its head first
	// Line is where the block starts in the text, from 0
	Line int
}

var (
	headingLine = regexp.MustCompile(`^ {0,3}(#{1,6})(?:\s+(.*?))?\s*#*\s*$`)
	itemLine    = regexp.MustCompile(`^(\s*)([-*+]|\d{1,9}[.)])\s+(.*)$`)
	ruleLine    = regexp.MustCompile(`^ {0,3}(?:(?:-\s*){3,}|(?:\*\s*){3,}|(?:_\s*){3,})$`)
	fenceLine   = regexp.MustCompile("^ {0,3}(```+|~~~+)\\s*([^`\\s]*)")
	tableRule   = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
	taskMark    = regexp.MustCompile(`^\[([ xX])\]\s+`)
	// htmlLine starts a block of HTML: a tag or a comment at the start of a line
	htmlLine = regexp.MustCompile(`^ {0,3}<(?:[A-Za-z]|/[A-Za-z]|!--)`)
)

// Parse reads a Markdown text into its blocks.
func Parse(text string) []Block {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	reader := &blocks{lines: lines}
	for reader.at < len(lines) {
		reader.next()
	}
	return reader.found
}

// blocks reads a text's lines one block at a time.
type blocks struct {
	lines []string
	at    int
	found []Block
}

// next reads the block that starts at the line it is on.
func (reader *blocks) next() {
	line := reader.lines[reader.at]
	start := reader.at
	switch {
	case strings.TrimSpace(line) == "":
		reader.at++
	case fenceLine.MatchString(line):
		reader.code(line)
	case headingLine.MatchString(line):
		level := headingLine.FindStringSubmatch(line)
		reader.add(Block{Kind: Heading, Level: len(level[1]), Spans: Inline(level[2]), Line: start})
		reader.at++
	case ruleLine.MatchString(line):
		reader.add(Block{Kind: Rule, Line: start})
		reader.at++
	case strings.HasPrefix(strings.TrimSpace(line), ">"):
		reader.quote()
	case htmlLine.MatchString(line):
		reader.html()
	case itemLine.MatchString(line):
		reader.item(line)
	case strings.Contains(line, "|") && reader.at+1 < len(reader.lines) && tableRule.MatchString(reader.lines[reader.at+1]):
		reader.table()
	default:
		reader.paragraph()
	}
}

// add keeps a block read.
func (reader *blocks) add(block Block) { reader.found = append(reader.found, block) }

// code reads a fenced block of code, up to its closing fence or the text's end.
func (reader *blocks) code(line string) {
	fence := fenceLine.FindStringSubmatch(line)
	block := Block{Kind: Code, Language: fence[2], Line: reader.at}
	marker := strings.TrimSpace(fence[1])
	for reader.at++; reader.at < len(reader.lines); reader.at++ {
		if strings.HasPrefix(strings.TrimSpace(reader.lines[reader.at]), marker) {
			reader.at++
			break
		}
		block.Lines = append(block.Lines, strings.ReplaceAll(reader.lines[reader.at], "\t", "    "))
	}
	reader.add(block)
}

// quote reads the lines of a quote, each paragraph of it a block.
func (reader *blocks) quote() {
	var words []string
	start := reader.at
	flush := func() {
		if len(words) > 0 {
			reader.add(Block{Kind: Quote, Spans: Inline(strings.Join(words, " ")), Line: start})
			words = nil
		}
	}
	for ; reader.at < len(reader.lines); reader.at++ {
		line := strings.TrimSpace(reader.lines[reader.at])
		if !strings.HasPrefix(line, ">") {
			break
		}
		inner := strings.TrimSpace(strings.TrimLeft(line, ">"))
		if inner == "" {
			flush()
			start = reader.at + 1
			continue
		}
		words = append(words, inner)
	}
	flush()
}

// item reads a list's item and the lines that go on with it.
func (reader *blocks) item(line string) {
	parts := itemLine.FindStringSubmatch(line)
	indent := len(strings.ReplaceAll(parts[1], "\t", "    "))
	block := Block{Kind: Item, Level: indent / 2, Mark: "•", Line: reader.at}
	if parts[2][0] >= '0' && parts[2][0] <= '9' {
		block.Mark = parts[2][:len(parts[2])-1] + "."
	}
	words := []string{parts[3]}
	if task := taskMark.FindStringSubmatch(parts[3]); task != nil {
		block.Mark, words[0] = "☐", parts[3][len(task[0]):]
		if task[1] != " " {
			block.Mark = "☑"
		}
	}
	for reader.at++; reader.at < len(reader.lines) && reader.goesOn(reader.lines[reader.at]); reader.at++ {
		words = append(words, strings.TrimSpace(reader.lines[reader.at]))
	}
	block.Spans = Inline(strings.Join(words, " "))
	reader.add(block)
}

// paragraph reads lines up to a blank one or the start of another block.
func (reader *blocks) paragraph() {
	start := reader.at
	words := []string{strings.TrimSpace(reader.lines[reader.at])}
	for reader.at++; reader.at < len(reader.lines) && reader.goesOn(reader.lines[reader.at]); reader.at++ {
		words = append(words, strings.TrimSpace(reader.lines[reader.at]))
	}
	reader.add(Block{Kind: Paragraph, Spans: Inline(strings.Join(words, " ")), Line: start})
}

// html reads a block of HTML, up to a blank line. Its tags are left out, as GitHub hides them;
// each of its lines with words left (a table's cell, an image's alt words) is a paragraph, since
// HTML lays out its own lines.
func (reader *blocks) html() {
	for ; reader.at < len(reader.lines) && strings.TrimSpace(reader.lines[reader.at]) != ""; reader.at++ {
		spans := Inline(strings.TrimSpace(reader.lines[reader.at]))
		if hasWords(spans) {
			reader.add(Block{Kind: Paragraph, Spans: spans, Line: reader.at})
		}
	}
}

// hasWords tells whether spans hold more than spaces.
func hasWords(spans []Span) bool {
	for _, span := range spans {
		if strings.TrimSpace(span.Text) != "" {
			return true
		}
	}
	return false
}

// goesOn tells whether a line carries on the paragraph or item before it.
func (reader *blocks) goesOn(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" && !headingLine.MatchString(line) && !fenceLine.MatchString(line) &&
		!ruleLine.MatchString(line) && !itemLine.MatchString(line) && !strings.HasPrefix(trimmed, ">")
}

// table reads a table: its head, the rule under it, and the rows up to a line with no |.
func (reader *blocks) table() {
	block := Block{Kind: Table, Line: reader.at}
	block.Rows = append(block.Rows, cells(reader.lines[reader.at]))
	for reader.at += 2; reader.at < len(reader.lines); reader.at++ {
		line := reader.lines[reader.at]
		if strings.TrimSpace(line) == "" || !strings.Contains(line, "|") {
			break
		}
		block.Rows = append(block.Rows, cells(line))
	}
	reader.add(block)
}

// cells are a table's line cut at its |, each cell's words styled.
func cells(line string) [][]Span {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	var row [][]Span
	for _, cell := range splitCells(line) {
		row = append(row, Inline(strings.TrimSpace(cell)))
	}
	return row
}

// splitCells cuts at the | that are not escaped nor inside `code`.
func splitCells(line string) []string {
	var found []string
	start, inCode := 0, false
	for index := 0; index < len(line); index++ {
		switch {
		case line[index] == '\\':
			index++
		case line[index] == '`':
			inCode = !inCode
		case line[index] == '|' && !inCode:
			found = append(found, line[start:index])
			start = index + 1
		}
	}
	return append(found, line[start:])
}
