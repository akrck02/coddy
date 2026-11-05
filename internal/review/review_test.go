package review

import (
	"testing"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/alecthomas/chroma/v2"
)

func context(count int) []git.DiffLine {
	lines := make([]git.DiffLine, count)
	for index := range lines {
		lines[index] = git.DiffLine{Kind: git.Context, OldNumber: index + 1, NewNumber: index + 1}
	}
	return lines
}

func TestUnchangedLinesFarFromAChangeAreFolded(test *testing.T) {
	diff := context(20)
	diff[10].Kind = git.Insertion
	rows := DiffRows(diff, 3, nil)
	// 7 folded, 3 before, the change, 3 after, 6 folded
	if len(rows) != 9 || !rows[0].IsFold() || rows[0].Folded != 7 || !rows[8].IsFold() || rows[8].Folded != 6 {
		test.Fatalf("rows: %+v", rows)
	}
	opened := DiffRows(diff, 3, map[int]bool{0: true})
	if len(opened) != 15 {
		test.Errorf("opening the first fold shows %d rows, want 15", len(opened))
	}
	if all := DiffRows(diff, -1, nil); len(all) != 20 {
		test.Errorf("the full view shows %d rows, want 20", len(all))
	}
	if starts := HunkStarts(rows); len(starts) != 1 || starts[0] != 4 {
		test.Errorf("hunks start at %v, want [4]", starts)
	}
}

func TestAChainOfLoneFoldersIsOneFolder(test *testing.T) {
	tree := BuildTree([]string{"src/main/java/com/lamp/Lamp.java", "src/main/java/com/lamp/Beam.java", "README.md"},
		func(path string) bool { return path == "README.md" || path == "src/main/java/com/lamp/Beam.java" })
	if tree.Changed != 2 || len(tree.Folders) != 1 || len(tree.Files) != 1 {
		test.Fatalf("root: %+v", tree)
	}
	folder := tree.Folders[0]
	if folder.Name != "src/main/java/com/lamp" || folder.Changed != 1 || len(folder.Files) != 2 {
		test.Errorf("folder: %+v", folder)
	}
}

func TestTheFinderPrefersTheFileName(test *testing.T) {
	candidates := []Candidate{
		{Label: "api/src/order/service/Helper.kt"},
		{Label: "api/src/OrderService.kt"},
		{Label: "api/src/Other.kt", Changed: true},
	}
	matches := FuzzyFind(candidates, "ordsvc", 10)
	if len(matches) != 2 || matches[0].Candidate.Label != "api/src/OrderService.kt" {
		test.Fatalf("matches: %+v", matches)
	}
	if empty := FuzzyFind(candidates, "", 10); len(empty) != 1 || !empty[0].Candidate.Changed {
		test.Errorf("an empty query lists %+v, want the changed file", empty)
	}
}

func TestHighlightedLinesKeepTheTextAndExpandTabs(test *testing.T) {
	lines := highlight.Lines("lamp.go", "package lamp\n\nfunc On() {\n\treturn\n}\n")
	if len(lines) != 5 {
		test.Fatalf("read %d lines, want 5", len(lines))
	}
	if text := lines[3].Text(); text != "    return" {
		test.Errorf("line 4 reads %q", text)
	}
	if lines[0].Spans[0].Kind == lines[0].Spans[len(lines[0].Spans)-1].Kind {
		test.Errorf("the keyword and the name were not told apart: %+v", lines[0].Spans)
	}
	if language := highlight.Language("build.gradle.kts", ""); language != "Kotlin" {
		test.Errorf("build.gradle.kts is %s", language)
	}
}

func TestLatinOneTextIsRead(test *testing.T) {
	if text := asText([]byte{'c', 0xe1, 'f'}); text != "cáf" {
		test.Errorf("read %q", text)
	}
}

func TestTheChangedWordsOfAReplacedLineAreFound(test *testing.T) {
	rows := []Row{
		{Line: git.DiffLine{Kind: git.Removal, OldNumber: 1}},
		{Line: git.DiffLine{Kind: git.Insertion, NewNumber: 1}},
		{Line: git.DiffLine{Kind: git.Removal, OldNumber: 2}},
		{Line: git.DiffLine{Kind: git.Insertion, NewNumber: 2}},
	}
	texts := map[git.DiffLine]string{
		rows[0].Line: `version = "1.5.26"`, rows[1].Line: `version = "1.5.27"`,
		rows[2].Line: `alpha`, rows[3].Line: `something else entirely`,
	}
	spans := InlineChanges(rows, func(line git.DiffLine) []rune { return []rune(texts[line]) })
	if spans[0] != (Span{16, 17}) || spans[1] != (Span{16, 17}) {
		test.Errorf("the version's changed digit is at %+v and %+v, want 16–17", spans[0], spans[1])
	}
	if _, marked := spans[3]; marked {
		test.Errorf("a line replaced wholly has a changed part marked: %+v", spans[3])
	}
}

func TestTheGutterMarksAddedChangedAndRemovedLines(test *testing.T) {
	diff := []git.DiffLine{
		{Kind: git.Context, OldNumber: 1, NewNumber: 1},
		{Kind: git.Removal, OldNumber: 2},
		{Kind: git.Insertion, NewNumber: 2},
		{Kind: git.Insertion, NewNumber: 3},
		{Kind: git.Context, OldNumber: 3, NewNumber: 4},
		{Kind: git.Removal, OldNumber: 4},
		{Kind: git.Context, OldNumber: 5, NewNumber: 5},
		{Kind: git.Insertion, NewNumber: 6},
		{Kind: git.Removal, OldNumber: 6},
	}
	marks := LineMarks(diff)
	want := map[int]Mark{2: ModifiedMark, 3: ModifiedMark, 5: RemovedAbove, 6: AddedMark}
	for number, mark := range want {
		if marks[number] != mark {
			test.Errorf("line %d is marked %d, want %d", number, marks[number], mark)
		}
	}
}

func TestTokensTakeTheRoleTheyAreColouredBy(test *testing.T) {
	cases := map[chroma.TokenType]highlight.Role{
		chroma.Keyword: highlight.Keyword, chroma.LiteralStringDouble: highlight.String,
		chroma.LiteralNumberInteger: highlight.Number, chroma.CommentSingle: highlight.Comment,
		chroma.NameFunction: highlight.Function, chroma.NameDecorator: highlight.Attribute,
		chroma.Name: highlight.Plain,
	}
	for kind, want := range cases {
		if role := highlight.RoleOf(kind); role != want {
			test.Errorf("%v takes %s, want %s", kind, role, want)
		}
	}
}

func TestAJavaTreeMarksRootsAndJoinsPackages(test *testing.T) {
	tree := BuildJavaTree([]string{
		"build.gradle",
		"core/build.gradle",
		"core/src/main/java/app/coast/lamp/Lamp.java",
		"core/src/main/java/app/coast/lamp/beam/Beam.java",
		"core/src/main/resources/lamp.properties",
		"core/src/test/java/app/coast/lamp/LampTest.java",
	}, nil)
	core := tree.Folders[0]
	if core.Name != "core" || core.Role != ModuleFolder {
		test.Fatalf("the module is %q with role %d", core.Name, core.Role)
	}
	main := core.Folders[0].Folders[0] // core/src/main
	java, resources := main.Folders[0], main.Folders[1]
	if java.Role != SourceRoot || resources.Role != ResourceRoot {
		test.Errorf("src/main's roots are %d and %d", java.Role, resources.Role)
	}
	packages := java.Folders[0]
	if packages.Name != "app.coast.lamp" || packages.Role != PackageFolder || packages.Path != "core/src/main/java/app/coast/lamp" {
		test.Errorf("the packages are %q (%d) at %s", packages.Name, packages.Role, packages.Path)
	}
	testRoot := core.Folders[0].Folders[1].Folders[0]
	if testRoot.Role != TestRoot {
		test.Errorf("src/test/java has role %d", testRoot.Role)
	}
}

func TestTheKindOfAClassIsReadFromItsDeclaration(test *testing.T) {
	cases := map[string]ClassKind{
		"public interface Lamp extends Light {":               InterfaceDeclaration,
		"@SuppressWarnings(\"x\")\npublic final class Lamp {": ClassDeclaration,
		"enum class Lamp { ON, OFF }":                         EnumDeclaration,
		"public @interface Lamp {}":                           AnnotationDeclaration,
		"public record Lamp(int watts) {}":                    RecordDeclaration,
		"object Lamp {":                                       ObjectDeclaration,
		"data class Beam(val x: Int)\nfun lamp() = 1":         NoClass,
	}
	for source, want := range cases {
		if got := KindOf("src/Lamp.kt", []byte(source)); got != want {
			test.Errorf("%q read as %d, want %d", source, got, want)
		}
	}
}
