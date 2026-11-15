// Package markdown reads Markdown into blocks of styled words, for showing a file formatted: the
// headings, paragraphs, lists, quotes, code, rules and tables people write, not every corner of
// CommonMark.
package markdown

import "strings"

// Span is a run of a block's words in one style.
type Span struct {
	Text                       string
	Bold, Italic, Code, Strike bool
	Link                       string // where a link goes; "" for words that are not one
}

// Inline reads a line's styles: `code`, **bold**, *italic*, ~~struck~~, [links](to), images as
// their words, and \escapes.
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
		spans, used := link(text[1:], style)
		return spans, used + min(used, 1)
	case text[0] == '[':
		return link(text, style)
	case text[0] == '<':
		return autolink(text, style)
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

// link is a [words](where) run: the words, styled, going where it says.
func link(text string, style Span) ([]Span, int) {
	close := strings.Index(text, "](")
	if close < 0 || strings.ContainsAny(text[1:close], "]\n") {
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
