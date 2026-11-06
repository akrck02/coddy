package highlight

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestHighlightedLinesKeepTheTextAndExpandTabs(test *testing.T) {
	lines := Lines("lamp.go", "package lamp\n\nfunc On() {\n\treturn\n}\n")
	if len(lines) != 5 {
		test.Fatalf("read %d lines, want 5", len(lines))
	}
	if text := lines[3].Text(); text != "    return" {
		test.Errorf("line 4 reads %q", text)
	}
	if lines[0].Spans[0].Kind == lines[0].Spans[len(lines[0].Spans)-1].Kind {
		test.Errorf("the keyword and the name were not told apart: %+v", lines[0].Spans)
	}
	if language := Language("build.gradle.kts", ""); language != "Kotlin" {
		test.Errorf("build.gradle.kts is %s", language)
	}
}

func TestTokensTakeTheRoleTheyAreColouredBy(test *testing.T) {
	cases := map[chroma.TokenType]Role{
		chroma.Keyword: Keyword, chroma.LiteralStringDouble: String,
		chroma.LiteralNumberInteger: Number, chroma.CommentSingle: Comment,
		chroma.NameFunction: Function, chroma.NameDecorator: Attribute,
		chroma.Name: Plain,
	}
	for kind, want := range cases {
		if role := RoleOf(kind); role != want {
			test.Errorf("%v takes %s, want %s", kind, role, want)
		}
	}
}

func TestColumnsCountTabsToTheirStop(test *testing.T) {
	line := "\tab\tc"
	if column := Column(line, 4); column != 8 {
		test.Errorf("the fifth character starts at column %d, want 8", column)
	}
	for _, place := range []struct {
		at   float32
		want int
	}{{1.9, 0}, {2.1, 1}, {4.4, 1}, {5.6, 3}, {99, 5}} {
		if got := CharacterAt(line, place.at); got != place.want {
			test.Errorf("at column %v is character %d, want %d", place.at, got, place.want)
		}
	}
}
