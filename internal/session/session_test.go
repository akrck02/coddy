package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/workspace"
)

// sessionWithFile is a session with one project, a plain folder holding one file.
func sessionWithFile(test *testing.T, content string) (Session, workspace.FileReference, string) {
	folder := test.TempDir()
	path := filepath.Join(folder, "harbour.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		test.Fatal(err)
	}
	opened := New("", true)
	opened.Projects = []*workspace.Project{{Name: "harbour", Path: folder}}
	return opened, workspace.FileReference{Project: 0, Path: "harbour.txt"}, path
}

func TestADocumentChangedOnDiskIsReadAgain(test *testing.T) {
	opened, reference, path := sessionWithFile(test, "one boat\n")
	opened.Keep(ReadDocument(opened.Projects[0], reference, true))
	if _, cached := opened.Document(reference); !cached {
		test.Fatal("a document just read is not kept")
	}
	if err := os.WriteFile(path, []byte("two boats and a lighthouse\n"), 0o644); err != nil {
		test.Fatal(err)
	}
	if _, cached := opened.Document(reference); cached {
		test.Error("a document whose file changed on disk is still given from the cache")
	}
}

func TestACacheOverItsBudgetForgetsTheLeastRecentlyUsedDocument(test *testing.T) {
	document := &workspace.Document{Text: "a line of text"}
	size := sizeOf(document)
	cache := newDocumentCache(2 * size)
	first, second, third := workspace.FileReference{Path: "first"}, workspace.FileReference{Path: "second"}, workspace.FileReference{Path: "third"}
	cache.keep(Read{Reference: first, Document: document})
	cache.keep(Read{Reference: second, Document: document})
	cache.take(first, diskStamp{}) // the first is now the most recently used
	cache.keep(Read{Reference: third, Document: document})
	if _, kept := cache.take(second, diskStamp{}); kept {
		test.Error("the least recently used document is kept past the budget")
	}
	for _, reference := range []workspace.FileReference{first, third} {
		if _, kept := cache.take(reference, diskStamp{}); !kept {
			test.Errorf("%s is forgotten, though it was used more recently", reference.Path)
		}
	}
	if cache.used != 2*size {
		test.Errorf("the cache counts %d bytes, want %d", cache.used, 2*size)
	}
}

func TestASmallerBudgetForgetsTheLeastRecentlyUsedDocumentsAtOnce(test *testing.T) {
	opened := New("", true)
	opened.SetDocumentMegabytes(64)
	text := strings.Repeat("a line of text that fills the cache\n", 30000) // about a megabyte
	references := []workspace.FileReference{{Path: "first"}, {Path: "second"}, {Path: "third"}}
	for _, reference := range references {
		opened.documents.keep(Read{Reference: reference, Document: &workspace.Document{Text: text}})
	}
	opened.documents.setBudget(2 * sizeOf(&workspace.Document{Text: text}))
	if documents, _ := opened.DocumentUse(); documents != 2 {
		test.Fatalf("after the budget shrank to two documents, %d are kept", documents)
	}
	if _, kept := opened.documents.take(references[0], diskStamp{}); kept {
		test.Error("the document opened longest ago is kept")
	}
}

func TestABudgetIsKeptWithinItsEndsInWholeSteps(test *testing.T) {
	cases := map[int]int{0: DefaultDocumentMegabytes, 100: 96, 8: DocumentMegabytesLeast, 4000: DocumentMegabytesMost, 128: 128}
	for asked, want := range cases {
		opened := New("", true)
		opened.SetDocumentMegabytes(asked)
		if opened.DocumentMegabytes() != want {
			test.Errorf("asking %d MB keeps %d MB, want %d", asked, opened.DocumentMegabytes(), want)
		}
	}
}

func TestAProjectReadAgainForgetsItsDocuments(test *testing.T) {
	opened, reference, _ := sessionWithFile(test, "one boat\n")
	opened.Keep(ReadDocument(opened.Projects[0], reference, true))
	opened.Loaded(&workspace.Project{Name: "harbour", Path: opened.Projects[0].Path, Loaded: true})
	if !opened.Projects[0].Loaded {
		test.Error("the project read is not put in the place of the old one")
	}
	if _, cached := opened.Document(reference); cached {
		test.Error("a document read from the project before it was read again is still kept")
	}
}

func TestABaseOfOneProjectLeavesTheOthersOnTheBaseForAll(test *testing.T) {
	opened := New("origin/main", true)
	opened.SetBase("/harbour", "v1.0")
	if base := opened.BaseFor("/harbour"); base != "v1.0" {
		test.Errorf("the project with its own base is compared against %q", base)
	}
	if base := opened.BaseFor("/lighthouse"); base != "origin/main" {
		test.Errorf("another project is compared against %q, not the base for all", base)
	}
	opened.SetBaseForAll("develop")
	if base := opened.BaseFor("/harbour"); base != "develop" {
		test.Errorf("after a base for all, the project keeps its own %q", base)
	}
}
