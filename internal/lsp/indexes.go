package lsp

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// indexName is how the index of one set of workspace folders is named: by a hash, 32 hexadecimal
// digits, as kotlin-lsp named them itself and the reader names those it places (indexFolder).
// Nothing else in a data folder is ever removed, but its Index.Retired.
var indexName = regexp.MustCompile(`^[0-9a-f]{32}$`)

// indexRest is how long an index must have gone unwritten before it may be removed: one a server
// of another window writes now is not touched.
const indexRest = 10 * time.Minute

// PruneIndexes removes, from a server's data folder, the indexes it built for sets of folders used
// longest ago, keeping its Index.Keep most recent: a server that builds one index per set of
// folders (kotlin-lsp, several gigabytes for the big workspace) would otherwise keep every one it ever built.
// It gives the folders removed. The data folder is the reader's own, never a repository.
func PruneIndexes(dataFolder string, server Server, now time.Time) ([]string, error) {
	if dataFolder == "" || server.Index.Folder == "" || server.Index.Keep <= 0 {
		return nil, nil
	}
	indexes := indexesOf(filepath.Join(dataFolder, server.Index.Folder))
	sort.Slice(indexes, func(first, second int) bool { return indexes[first].used.After(indexes[second].used) })
	old := indexes[min(server.Index.Keep, len(indexes)):]
	for _, name := range server.Index.Retired {
		if path := filepath.Join(dataFolder, name); name != "" && name != server.Index.Folder {
			if _, err := os.Stat(path); err == nil {
				old = append(old, index{path, lastWritten(path)})
			}
		}
	}
	var removed []string
	for _, each := range old {
		if now.Sub(each.used) < indexRest {
			continue
		}
		if err := os.RemoveAll(each.path); err != nil {
			return removed, err
		}
		removed = append(removed, each.path)
	}
	return removed, nil
}

// index is an index of a data folder and when it was last written.
type index struct {
	path string
	used time.Time
}

// indexesOf are the indexes in a folder of indexes, none when there is no such folder yet.
func indexesOf(folder string) []index {
	entries, _ := os.ReadDir(folder)
	var indexes []index
	for _, entry := range entries {
		if entry.IsDir() && indexName.MatchString(entry.Name()) {
			path := filepath.Join(folder, entry.Name())
			indexes = append(indexes, index{path, lastWritten(path)})
		}
	}
	return indexes
}

// indexFolder is where a server that is told where to keep its index (Index.Option) keeps the one
// of the folders it starts with, in that order: a folder of its Indexes named by their hash; its
// data folder when it has no Indexes.
func indexFolder(dataFolder string, server Server, folders []string) string {
	if server.Index.Folder == "" {
		return dataFolder
	}
	hash := md5.Sum([]byte(strings.Join(folders, "\n")))
	return filepath.Join(dataFolder, server.Index.Folder, hex.EncodeToString(hash[:]))
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
