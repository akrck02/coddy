package session

import (
	"container/list"

	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/workspace"
)

// DefaultDocumentMegabytes is how much memory, in megabytes, the documents kept may take unless
// the person chooses otherwise: a few hundred files of a usual size, as a file of code takes about
// 100 to 350 KB once read with its two versions, its diff and its colours.
const DefaultDocumentMegabytes = 64

// The budgets of the documents kept a person may choose, in megabytes: from the least to the
// most, in whole steps.
const (
	DocumentMegabytesLeast = 32
	DocumentMegabytesMost  = 512
	DocumentMegabytesStep  = 32
)

// bytesPerMegabyte is how many bytes a megabyte holds.
const bytesPerMegabyte = 1 << 20

// About how many bytes the parts of a document take besides their text, measured roughly on
// 64-bit Go: a document's fields, a coloured line's slice and columns, a span's column, kind and
// string header, a diff line's three numbers.
const (
	documentBytes = 512
	lineBytes     = 32
	spanBytes     = 40
	diffLineBytes = 24
)

// documentCache keeps the documents read, the most recently opened first, up to a budget of bytes.
type documentCache struct {
	budget  int
	used    int
	order   *list.List // of *cached, the most recently opened at the front
	entries map[workspace.FileReference]*list.Element
}

// cached is one document kept, with what its file was on disk when read and its size.
type cached struct {
	read  Read
	bytes int
}

func newDocumentCache(budget int) documentCache {
	return documentCache{budget: budget, order: list.New(), entries: map[workspace.FileReference]*list.Element{}}
}

// keep remembers a document read, in the place of an older one of the same file.
func (cache *documentCache) keep(read Read) {
	cache.forget(read.Reference)
	entry := &cached{read: read, bytes: sizeOf(read.Document)}
	cache.entries[read.Reference] = cache.order.PushFront(entry)
	cache.used += entry.bytes
	cache.setBudget(cache.budget)
}

// take gives a document kept, and puts it first, unless its file is no longer what it was on disk:
// then it is forgotten.
func (cache *documentCache) take(reference workspace.FileReference, now diskStamp) (*workspace.Document, bool) {
	element, found := cache.entries[reference]
	if !found {
		return nil, false
	}
	entry := element.Value.(*cached)
	if !entry.read.stamp.modified.Equal(now.modified) || entry.read.stamp.size != now.size {
		cache.forget(reference)
		return nil, false
	}
	cache.order.MoveToFront(element)
	return entry.read.Document, true
}

// setBudget changes how many bytes the documents kept may take, forgetting the least recently
// opened at once until they fit. The most recent one stays, as keep leaves it, whatever its size.
func (cache *documentCache) setBudget(budget int) {
	cache.budget = budget
	for cache.used > cache.budget && cache.order.Len() > 1 {
		cache.forget(cache.order.Back().Value.(*cached).read.Reference)
	}
}

// forget forgets the document of one file, if kept.
func (cache *documentCache) forget(reference workspace.FileReference) {
	element, found := cache.entries[reference]
	if !found {
		return
	}
	cache.used -= element.Value.(*cached).bytes
	cache.order.Remove(element)
	delete(cache.entries, reference)
}

// forgetWhere forgets the documents of the files that match.
func (cache *documentCache) forgetWhere(matches func(workspace.FileReference) bool) {
	for reference := range cache.entries {
		if matches(reference) {
			cache.forget(reference)
		}
	}
}

// forgetAll forgets every document.
func (cache *documentCache) forgetAll() {
	cache.order.Init()
	clear(cache.entries)
	cache.used = 0
}

// sizeOf is about how many bytes a document takes in memory.
func sizeOf(document *workspace.Document) int {
	size := documentBytes + len(document.Text) + len(document.Diff)*diffLineBytes
	for _, lines := range [][]highlight.Line{document.OldLines, document.NewLines} {
		for _, line := range lines {
			size += lineBytes + len(line.Spans)*spanBytes
			for _, span := range line.Spans {
				size += len(span.Text)
			}
		}
	}
	return size
}
