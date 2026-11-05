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
