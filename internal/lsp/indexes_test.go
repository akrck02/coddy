package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOldIndexesAreRemovedAndTheRecentOnesKept(test *testing.T) {
	data := test.TempDir()
	now := time.Now()
	ages := map[string]time.Duration{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 1 * time.Hour,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": 3 * time.Hour,
		"cccccccccccccccccccccccccccccccc": 9 * time.Hour,
		"not-an-index":                     20 * time.Hour,
	}
	for name, age := range ages {
		file := filepath.Join(data, "workspaces", name, "index", "rocks.sst")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			test.Fatal(err)
		}
		os.WriteFile(file, []byte("index"), 0o644)
		for _, path := range []string{file, filepath.Dir(file), filepath.Dir(filepath.Dir(file))} {
			os.Chtimes(path, now.Add(-age), now.Add(-age))
		}
	}
	server := Server{Name: "kls", Index: Index{Folder: "workspaces", Keep: 2}}
	removed, err := PruneIndexes(data, server, now)
	if err != nil || len(removed) != 1 || filepath.Base(removed[0]) != "cccccccccccccccccccccccccccccccc" {
		test.Fatalf("removed %v (%v), want the index used longest ago", removed, err)
	}
	for _, kept := range []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "not-an-index"} {
		if _, err := os.Stat(filepath.Join(data, "workspaces", kept)); err != nil {
			test.Errorf("%s was removed", kept)
		}
	}
}

func TestAnIndexWrittenAMomentAgoIsKept(test *testing.T) {
	data := test.TempDir()
	for _, name := range []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} {
		os.MkdirAll(filepath.Join(data, "workspaces", name), 0o755)
	}
	removed, _ := PruneIndexes(data, Server{Index: Index{Folder: "workspaces", Keep: 1}}, time.Now())
	if len(removed) != 0 {
		test.Errorf("an index another window may be writing was removed: %v", removed)
	}
}

// A server told where to keep its index keeps one per set of folders: started with fewer folders
// than the model in its index names, kotlin-lsp hangs.
func TestEachSetOfFoldersHasAnIndexOfItsOwn(test *testing.T) {
	server, _ := serverNamed("kotlin-lsp")
	data := "/data"
	both := indexFolder(data, server, []string{"/code/lamp", "/code/keeper"})
	if both != indexFolder(data, server, []string{"/code/lamp", "/code/keeper"}) {
		test.Error("the same folders were given two indexes")
	}
	if both == indexFolder(data, server, []string{"/code/lamp"}) {
		test.Error("fewer folders were given the index of more")
	}
	if filepath.Dir(both) != filepath.Join(data, server.Index.Folder) || !indexName.MatchString(filepath.Base(both)) {
		test.Errorf("the index is at %s, not one PruneIndexes knows", both)
	}
	options := (&Client{Server: server, data: data}).initializationOptions([]string{"/code/lamp"})
	if options[server.Index.Option] != indexFolder(data, server, []string{"/code/lamp"}) {
		test.Errorf("the server is told to keep its index at %v", options[server.Index.Option])
	}
}

// The folder where an older reader kept a server's index is removed once unused a while.
func TestARetiredIndexIsRemoved(test *testing.T) {
	data, now := test.TempDir(), time.Now()
	old := filepath.Join(data, "rocks", "v241")
	os.MkdirAll(old, 0o755)
	for _, path := range []string{old, filepath.Dir(old)} {
		os.Chtimes(path, now.Add(-time.Hour), now.Add(-time.Hour))
	}
	server := Server{Index: Index{Folder: "indexes", Keep: 1, Retired: []string{"rocks"}}}
	if removed, err := PruneIndexes(data, server, now); err != nil || len(removed) != 1 {
		test.Fatalf("removed %v (%v), want rocks", removed, err)
	}
	if _, err := os.Stat(filepath.Join(data, "rocks")); err == nil {
		test.Error("the retired index is still there")
	}
}
