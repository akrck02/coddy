package markdown

import (
	"reflect"
	"testing"
)

func TestWordsKeepTheirStyles(test *testing.T) {
	spans := Inline("A **bold** word, *one* in `code`, ~~gone~~ and [a link](https://harbour.example) here")
	want := []Span{
		{Text: "A "}, {Text: "bold", Bold: true}, {Text: " word, "}, {Text: "one", Italic: true},
		{Text: " in "}, {Text: "code", Code: true}, {Text: ", "}, {Text: "gone", Strike: true},
		{Text: " and "}, {Text: "a link", Link: "https://harbour.example"}, {Text: " here"},
	}
	if !reflect.DeepEqual(spans, want) {
		test.Errorf("the spans are %+v", spans)
	}
}

func TestMarksThatCloseNothingAreKeptAsWritten(test *testing.T) {
	for _, text := range []string{"2 * 3 = 6", "a snake_case_name", "[x] not a link", "`open"} {
		if spans := Inline(text); len(spans) != 1 || spans[0].Text != text || spans[0] != (Span{Text: text}) {
			test.Errorf("%q reads as %+v", text, spans)
		}
	}
}

func TestStylesNestAndEscapesAreKept(test *testing.T) {
	spans := Inline(`**bold *and italic***, \*not italic\*`)
	want := []Span{{Text: "bold ", Bold: true}, {Text: "and italic", Bold: true, Italic: true}, {Text: ", *not italic*"}}
	if !reflect.DeepEqual(spans, want) {
		test.Errorf("the spans are %+v", spans)
	}
}

func TestATextIsReadIntoItsBlocks(test *testing.T) {
	text := "# The harbour\n\nA paragraph\ngoing on.\n\n- one\n  - two\n1. first\n- [x] done\n\n> said\n\n```go\nfunc Lamp() {}\n```\n\n---\n"
	var kinds []Kind
	for _, block := range Parse(text) {
		kinds = append(kinds, block.Kind)
	}
	want := []Kind{Heading, Paragraph, Item, Item, Item, Item, Quote, Code, Rule}
	if !reflect.DeepEqual(kinds, want) {
		test.Fatalf("the kinds are %v, want %v", kinds, want)
	}
	blocks := Parse(text)
	if blocks[1].Spans[0].Text != "A paragraph going on." || blocks[3].Level != 1 || blocks[4].Mark != "1." || blocks[5].Mark != "☑" {
		test.Errorf("the blocks are %+v", blocks)
	}
	if blocks[7].Language != "go" || len(blocks[7].Lines) != 1 || blocks[7].Line != 12 {
		test.Errorf("the code is %+v", blocks[7])
	}
}

func TestATableKeepsItsCells(test *testing.T) {
	blocks := Parse("| Key | Does |\n|---|:--|\n| `a\\|b` | **opens** |\n| n | next |\n\nafter")
	if len(blocks) != 2 || blocks[0].Kind != Table || len(blocks[0].Rows) != 3 {
		test.Fatalf("the blocks are %+v", blocks)
	}
	if cell := blocks[0].Rows[1][0]; len(cell) != 1 || !cell[0].Code || cell[0].Text != `a\|b` {
		test.Errorf("the escaped cell is %+v", cell)
	}
}
