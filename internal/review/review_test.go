package review

import (
	"testing"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
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
