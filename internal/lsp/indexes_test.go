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
	server := Server{Name: "kls", Indexes: "workspaces", KeepIndexes: 2}
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
	removed, _ := PruneIndexes(data, Server{Indexes: "workspaces", KeepIndexes: 1}, time.Now())
	if len(removed) != 0 {
		test.Errorf("an index another window may be writing was removed: %v", removed)
	}
}
