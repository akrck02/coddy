package navigation

import (
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/workspace"
)

// shownHost is a window that counts how often the uses are laid out, and keeps the last note.
type shownHost struct {
	shown int
	noted string
}

func (host *shownHost) Projects() []*workspace.Project      { return nil }
func (host *shownHost) LineOf(path string, line int) string { return "" }
func (host *shownHost) ClientFor(project, path string) (*lsp.Client, string, bool) {
	return nil, "", false
}
func (host *shownHost) ShowUses(search *Search, note string, opening bool) {
	host.shown++
	host.noted = note
}
func (host *shownHost) NoteUses(note string) { host.noted = note }

// The note of a search changes with each word of a server's progress: saying it again must not lay
// out the uses, the sidebar and the marks again, which made the uses mode crawl.
func TestANewNoteAloneDoesNotLayTheUsesOutAgain(test *testing.T) {
	host := &shownHost{}
	uses := NewUses(host, nil)
	search := &Search{known: true, found: map[string]lsp.Location{}}
	uses.search = search
	uses.show(search, true)
	search.unchecked++ // the note says one more place no server reads; the uses are the same
	uses.note(search)
	if host.shown != 1 {
		test.Errorf("a new note laid the uses out %d times more", host.shown-1)
	}
	if !strings.Contains(host.noted, "in files no server reads") {
		test.Errorf("the note shown is %q", host.noted)
	}
	search.found["somewhere:9"] = lsp.Location{}
	uses.note(search)
	if host.shown != 2 {
		test.Error("a use found was not laid out")
	}
}
