package lsp

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// indexName is how a server names the index of one set of workspace folders: kotlin-lsp names it
// by a hash, 32 hexadecimal digits. Nothing else in its data folder is ever removed.
var indexName = regexp.MustCompile(`^[0-9a-f]{32}$`)

// indexRest is how long an index must have gone unwritten before it may be removed: one a server
// of another window writes now is not touched.
const indexRest = 10 * time.Minute

// PruneIndexes removes, from a server's data folder, the indexes it built for sets of folders used
// longest ago, keeping its KeepIndexes most recent: a server that builds one index per set of
// folders (kotlin-lsp, several gigabytes for the big workspace) would otherwise keep every one it ever built.
// It gives the folders removed. The data folder is the reader's own, never a repository.
func PruneIndexes(dataFolder string, server Server, now time.Time) ([]string, error) {
	if dataFolder == "" || server.Indexes == "" || server.KeepIndexes <= 0 {
		return nil, nil
	}
	folder := filepath.Join(dataFolder, server.Indexes)
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, nil // no index yet
	}
	type index struct {
		path string
		used time.Time
	}
	var indexes []index
	for _, entry := range entries {
		if entry.IsDir() && indexName.MatchString(entry.Name()) {
			path := filepath.Join(folder, entry.Name())
			indexes = append(indexes, index{path, lastWritten(path)})
		}
	}
	sort.Slice(indexes, func(first, second int) bool { return indexes[first].used.After(indexes[second].used) })
	var removed []string
	for _, old := range indexes[min(server.KeepIndexes, len(indexes)):] {
		if now.Sub(old.used) < indexRest {
			continue
		}
		if err := os.RemoveAll(old.path); err != nil {
			return removed, err
		}
		removed = append(removed, old.path)
	}
	return removed, nil
}

// lastWritten is when anything in a folder, three levels down, was written last: an index's
// database files change while a server reads with it.
func lastWritten(folder string) time.Time {
	var last time.Time
	_ = filepath.WalkDir(folder, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil && info.ModTime().After(last) {
			last = info.ModTime()
		}
		relative, _ := filepath.Rel(folder, path)
		if entry.IsDir() && depth(relative) >= 6 {
			return filepath.SkipDir
		}
		return nil
	})
	return last
}

// depth is how many folders deep a relative path is.
func depth(relative string) int {
	if relative == "." {
		return 0
	}
	count := 1
	for _, character := range relative {
		if character == filepath.Separator {
			count++
		}
	}
	return count
}
