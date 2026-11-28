package viewer

import (
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/format/markdown"
	"github.com/akrck02/coddy/editor/internal/ui/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// letterMeasurer measures 7 pixels a letter, and code, wider, 9.
type letterMeasurer struct{}

func (letterMeasurer) MeasureText(text string, _ float32, weight draw2d.Weight, _ float32) draw2d.TextMetrics {
	letter := float32(7)
	if weight == draw2d.Mono {
		letter = 9
	}
	return draw2d.TextMetrics{Width: float32(len([]rune(text))) * letter, Ascent: 10, Descent: 3}
}

func (letterMeasurer) HasIcon(string) bool { return false }

// tableText lays a Markdown table out in a width and gives back the words it drew.
func tableText(source string, width float32) []shape {
	context := ui.NewWithMeasurer(letterMeasurer{})
	look := theme.Find(theme.BuiltIn(), theme.DefaultDark, true).Look
	layout := &formatter{context: context, palette: context.Theme, look: look, width: width, codeSize: 13, charWidth: 9}
	for _, block := range markdown.Parse(source) {
		layout.block(block)
	}
	var words []shape
	for _, each := range layout.shapes {
		if strings.TrimSpace(each.text) != "" {
			words = append(words, each)
		}
	}
	return words
}

// A narrow table keeps each word inside its own column: the first column is not shrunk below its
// longest word, and a word longer than its column is cut instead of reaching into the next one.
func TestNoWordOfATableIsDrawnOverAnother(test *testing.T) {
	table := "| File | Purpose |\n|---|---|\n" +
		"| `AGENTS.md` | The general rules: communication, names, functions, errors, structure, tests |\n" +
		"| `templates/CLAUDE.md` | Template for each project's own file |\n" +
		"| averyveryveryverylongwordwithoutanyspace | Copies the rules |\n"
	for _, width := range []float32{600, 300, 160} {
		words := tableText(table, width)
		for first, one := range words {
			oneRight := one.area.X + letterMeasurer{}.MeasureText(strings.TrimRight(one.text, " "), 0, one.weight, 0).Width
			if oneRight > width+1 {
				test.Errorf("at %v pixels, %q reaches %v, past the table", width, one.text, oneRight)
			}
			for _, other := range words[first+1:] {
				if one.area.Y != other.area.Y {
					continue
				}
				otherRight := other.area.X + letterMeasurer{}.MeasureText(strings.TrimRight(other.text, " "), 0, other.weight, 0).Width
				if one.area.X < otherRight && other.area.X < oneRight {
					test.Errorf("at %v pixels, %q and %q are drawn over each other", width, one.text, other.text)
				}
			}
		}
	}
}

// A long piece of code in one column must not squeeze the others: its least is its longest word,
// since it goes on at its spaces. A short id beside it stays whole.
func TestLongCodeInACellLeavesTheOtherColumnsWhole(test *testing.T) {
	table := "| Id | Check |\n|---|---|\n| R1 | test: `a file takes the icon of its name, then of its longest extension` |\n"
	for _, word := range tableText(table, 620) { // the code fits the page, not its column
		if word.text == "I" || word.text == "R" {
			test.Errorf("%q was cut out of a short word: the id column was squeezed", word.text)
		}
	}
}

// Code cut at its spaces reads as one run: one shade a line, not a chip a word.
func TestCodeCutAtItsSpacesHasOneShadeALine(test *testing.T) {
	context := ui.NewWithMeasurer(letterMeasurer{})
	look := theme.Find(theme.BuiltIn(), theme.DefaultDark, true).Look
	layout := &formatter{context: context, palette: context.Theme, look: look, width: 200, codeSize: 13, charWidth: 9}
	for _, block := range markdown.Parse("`one two three four five six seven eight`") {
		layout.block(block)
	}
	shades, lines := 0, map[float32]bool{}
	for _, each := range layout.shapes {
		if each.text == "" && each.colour == look.Hover {
			shades++
		}
		if strings.TrimSpace(each.text) != "" {
			lines[each.area.Y] = true
		}
	}
	if shades != len(lines) {
		test.Errorf("the code on %d lines has %d shades, want one a line", len(lines), shades)
	}
}
