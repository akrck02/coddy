package git

import (
	"errors"
	"testing"
)

func TestOnlyReadOnlyCommandsRun(test *testing.T) {
	for _, command := range []string{"commit", "push", "fetch", "checkout", "reset", "add", "update-index", "gc", "status"} {
		if _, err := run(".", command); !errors.Is(err, CommandNotReadOnly) {
			test.Errorf("git %s was allowed to run", command)
		}
	}
}

func TestNameStatusReadsRenamesAndDeletions(test *testing.T) {
	changes := parseNameStatus([]string{"M", "a.go", "R087", "old/b.go", "new/b.go", "D", "c.go", "A", "d.go"})
	if len(changes) != 4 {
		test.Fatalf("read %d changes, want 4", len(changes))
	}
	rename := changes[1]
	if rename.Status != Renamed || rename.OldPath != "old/b.go" || rename.Path != "new/b.go" {
		test.Errorf("rename read as %+v", rename)
	}
	if changes[2].Status != Deleted || changes[3].Status != Added {
		test.Errorf("statuses read as %v and %v", changes[2].Status, changes[3].Status)
	}
}

func TestAUnifiedDiffIsNumberedOnBothSides(test *testing.T) {
	output := "diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n one\n-two\n+TWO\n three\n\\ No newline at end of file\n"
	diff, err := parseUnifiedDiff([]byte(output))
	if err != nil {
		test.Fatal(err)
	}
	want := []DiffLine{{Context, 1, 1}, {Removal, 2, 0}, {Insertion, 0, 2}, {Context, 3, 3}}
	if len(diff.Lines) != len(want) {
		test.Fatalf("read %d lines, want %d", len(diff.Lines), len(want))
	}
	for index, line := range want {
		if diff.Lines[index] != line {
			test.Errorf("line %d is %+v, want %+v", index, diff.Lines[index], line)
		}
	}
}

func TestABinaryDiffIsMarked(test *testing.T) {
	diff, _ := parseUnifiedDiff([]byte("diff --git a/x b/x\nBinary files a/x and b/x differ\n"))
	if !diff.Binary {
		test.Error("a binary diff was not marked")
	}
}

func TestNumStatReadsRenamesAndBinaries(test *testing.T) {
	counts := parseNumStat([]string{"3\t1\ta.go", "0\t0\t", "old/b.go", "new/b.go", "-\t-\tlogo.png"})
	if counts["a.go"] != (LineCounts{Added: 3, Removed: 1}) {
		test.Errorf("a.go counted as %+v", counts["a.go"])
	}
	if _, found := counts["new/b.go"]; !found {
		test.Errorf("the rename is not counted by its new path: %+v", counts)
	}
	if !counts["logo.png"].Binary {
		test.Errorf("the picture is not marked binary")
	}
}

func TestGrepOutputIsReadIntoHits(test *testing.T) {
	hits := parseGrep([]byte("HEAD:src/a.go\x0012\x005\x00\treturn lamp\nnotes.txt\x003\x001\x00lamp: on\n"), true)
	if len(hits) != 2 {
		test.Fatalf("read %d hits, want 2", len(hits))
	}
	if hits[0] != (Hit{Path: "src/a.go", Line: 12, Column: 4, Text: "\treturn lamp"}) {
		test.Errorf("read %+v", hits[0])
	}
}
