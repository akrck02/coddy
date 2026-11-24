// Package markdown reads Markdown into blocks of styled words, for showing a file formatted: the
// headings, paragraphs, lists, quotes, code, rules and tables people write, not every corner of
// CommonMark.
package markdown

import (
	"regexp"
	"strings"
)

// Span is a run of a block's words in one style.
type Span struct {
	Text                       string
	Bold, Italic, Code, Strike bool
	Link                       string // where a link goes; "" for words that are not one
}

// Inline reads a line's styles: `code`, **bold**, *italic*, ~~struck~~, [links](to), images as
// their words, and \escapes. HTML tags are left out, as GitHub hides them, but an <img> shows its
// alt words, as a Markdown image does.
func Inline(text string) []Span {
	return merged(inline(text, Span{}))
}

// inline reads text in the style of the span around it.
func inline(text string, style Span) []Span {
	var spans []Span
	plain := strings.Builder{}
	flush := func() {
		if plain.Len() > 0 {
			spans = append(spans, withText(style, plain.String()))
			plain.Reset()
		}
	}
	for index := 0; index < len(text); {
		inner, used := styled(text[index:], style)
		if used == 0 {
			if text[index] == '\\' && index+1 < len(text) && isPunctuation(text[index+1]) {
				index++
			}
			plain.WriteByte(text[index])
			index++
			continue
		}
		flush()
		spans = append(spans, inner...)
		index += used
	}
	flush()
	return spans
}

// styled reads a styled run at the start of text: its spans and how many bytes it took, 0 when
// text does not start with one.
func styled(text string, style Span) ([]Span, int) {
	switch {
	case text[0] == '`':
		return code(text, style)
	case strings.HasPrefix(text, "!["):
		return image(text, style)
	case text[0] == '[':
		return link(text, style)
	case text[0] == '<':
		if spans, used := autolink(text, style); used > 0 {
			return spans, used
		}
		return htmlTag(text, style)
	}
	for _, marker := range []string{"**", "__", "~~", "*", "_"} {
		if spans, used := emphasis(text, marker, style); used > 0 {
			return spans, used
		}
	}
	return nil, 0
}

// code is a `code` run: its text as it is, styles inside it not read.
func code(text string, style Span) ([]Span, int) {
	ticks := len(text) - len(strings.TrimLeft(text, "`"))
	fence := text[:ticks]
	end := strings.Index(text[ticks:], fence)
	if end < 0 {
		return nil, 0
	}
	inner := strings.TrimSpace(text[ticks : ticks+end])
	style.Code = true
	return []Span{withText(style, inner)}, ticks + end + ticks
}

// emphasis is a run between two markers: ** or __ bold, * or _ italic, ~~ struck.
func emphasis(text, marker string, style Span) ([]Span, int) {
	if !strings.HasPrefix(text, marker) || len(text) <= len(marker) || text[len(marker)] == ' ' {
		return nil, 0
	}
	end := closing(text[len(marker):], marker)
	if end <= 0 {
		return nil, 0
	}
	switch marker {
	case "**", "__":
		style.Bold = true
	case "~~":
		style.Strike = true
	default:
		style.Italic = true
	}
	inner := text[len(marker) : len(marker)+end]
	return inline(inner, style), len(marker)*2 + end
}

// closing is where a marker closes in text: not after a space, and for _ not inside a word.
func closing(text, marker string) int {
	for from := 0; from < len(text); {
		found := strings.Index(text[from:], marker)
		if found < 0 {
			return -1
		}
		at := from + found
		after := at + len(marker)
		for len(marker) == 2 && after < len(text) && text[after] == marker[0] {
			at, after = at+1, after+1 // ***: the bold closes last, the italic inside it first
		}
		inWord := marker[0] == '_' && after < len(text) && isWordByte(text[after])
		doubled := len(marker) == 1 && after < len(text) && text[after] == marker[0]
		if at > 0 && text[at-1] != ' ' && text[at-1] != '\\' && !inWord && !doubled {
			return at
		}
		from = after
		if doubled {
			from++
		}
	}
	return -1
}

// link is a [words](where) run: the words, styled, going where it says. The words may hold an
// image, as a badge that links somewhere does: [![Go](badge)](go.mod).
func link(text string, style Span) ([]Span, int) {
	close := closingBracket(text)
	if close < 0 || !strings.HasPrefix(text[close:], "](") {
		return nil, 0
	}
	end := strings.IndexByte(text[close+2:], ')')
	if end < 0 {
		return nil, 0
	}
	target := strings.Fields(text[close+2 : close+2+end])
	style.Link = "#"
	if len(target) > 0 {
		style.Link = target[0]
	}
	return inline(text[1:close], style), close + 2 + end + 1
}

// image is a ![words](picture) run: its words, as a link to the picture, or to where the link
// around it goes, as a badge's does.
func image(text string, style Span) ([]Span, int) {
	spans, used := link(text[1:], style)
	if used == 0 {
		return nil, 0
	}
	for index := range spans {
		if style.Link != "" {
			spans[index].Link = style.Link
		}
	}
	return spans, used + 1
}

// closingBracket is where the [ that text starts with closes, past the brackets inside it; -1
// when it does not close on its line.
func closingBracket(text string) int {
	depth := 0
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case '\n':
			return -1
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

var (
	// tagPattern is an HTML tag, opening or closing, or a comment
	tagPattern = regexp.MustCompile(`^(?:<!--.*?-->|</?([A-Za-z][A-Za-z0-9-]*)(?:\s[^<>]*)?/?>)`)
	// altPattern is an <img>'s alt words
	altPattern = regexp.MustCompile(`\salt="([^"]*)"`)
)

// htmlTag is an HTML tag, left out: nothing for most, the alt words for an <img>.
func htmlTag(text string, style Span) ([]Span, int) {
	tag := tagPattern.FindStringSubmatch(text)
	if tag == nil {
		return nil, 0
	}
	alt := altPattern.FindStringSubmatch(tag[0])
	if !strings.EqualFold(tag[1], "img") || alt == nil || alt[1] == "" {
		return []Span{}, len(tag[0])
	}
	return []Span{withText(style, alt[1])}, len(tag[0])
}

// autolink is a <https://…> run: the address, as a link to itself.
func autolink(text string, style Span) ([]Span, int) {
	end := strings.IndexByte(text, '>')
	if end < 0 {
		return nil, 0
	}
	address := text[1:end]
	if !strings.Contains(address, "://") && !strings.HasPrefix(address, "mailto:") || strings.ContainsAny(address, " <") {
		return nil, 0
	}
	style.Link = address
	return []Span{withText(style, address)}, end + 1
}

// withText is a style with words in it.
func withText(style Span, text string) Span {
	style.Text = text
	return style
}

// merged joins spans next to each other in the same style.
func merged(spans []Span) []Span {
	var joined []Span
	for _, span := range spans {
		last := len(joined) - 1
		if last >= 0 && withText(joined[last], "") == withText(span, "") {
			joined[last].Text += span.Text
			continue
		}
		joined = append(joined, span)
	}
	return joined
}

// isPunctuation tells whether a \ before a character keeps it as it is.
func isPunctuation(character byte) bool {
	return strings.IndexByte("\\`*_{}[]()#+-.!|~<>", character) >= 0
}

// isWordByte tells whether a byte is part of a word, for _ inside snake_case.
func isWordByte(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || character >= 0x80
}
