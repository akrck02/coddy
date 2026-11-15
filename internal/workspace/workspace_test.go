package workspace

import (
	"os"
	"testing"

	"github.com/akrck02/code-review/internal/git"
)

// TestReadingARealRepository reads a repository named by REVIEW_REPOSITORY, to try the reading
// against real developments; without it the test is skipped.
func TestARealRepositoryIsReadWithItsChanges(test *testing.T) {
	path := os.Getenv("REVIEW_REPOSITORY")
	if path == "" {
		test.Skip("set REVIEW_REPOSITORY to a repository to read")
	}
	project := LoadProject(path, LoadOptions{WorkingTree: true})
	test.Logf("%s on %s against %s (%s), %d commits ahead, %d changed of %d files, error %q",
		project.Name, project.Branch, project.Base, project.MergeBase, project.Ahead,
		len(project.Changes), len(project.Files), project.Error)
	for index, change := range project.Changes {
		if index >= 6 {
			break
		}
		document := OpenDocument(project, change.Path, true)
		added, removed := Counts(document.Diff)
		test.Logf("  %s %s (%s) +%d -%d, %d rows folded, error %q", change.Status.Letter(), change.Path,
			document.Language, added, removed, len(DiffRows(document.Diff, 3, nil)), document.Error)
	}
}

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
