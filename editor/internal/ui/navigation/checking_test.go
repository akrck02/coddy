package navigation

import (
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/sdk/jsonrpc"
)

// shownHost is a window that counts how often the uses are laid out, and keeps the last note.
type shownHost struct {
	shown int
	noted string
}

func (host *shownHost) Projects() []*codebase.Project       { return nil }
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

// A server asked again for the uses once it has read the projects may fail: the uses found stay,
// and the note says the list may lack some, instead of the search ending as if it were whole.
func TestAServerThatFailsWhenAskedAgainIsToldInTheNote(test *testing.T) {
	host := &shownHost{}
	uses := NewUses(host, nil)
	search := &Search{known: true, found: map[string]lsp.Location{"somewhere:9": {}}}
	search.Asked.Client = &lsp.Client{Server: lsp.Server{Name: "gopls"}}
	uses.search = search
	uses.answeredAgain(search, nil, jsonrpc.RequestFailed)
	if !strings.Contains(host.noted, "gopls could not be asked again") {
		test.Errorf("the note after a failed second ask is %q", host.noted)
	}
	if len(search.found) != 1 {
		test.Errorf("the uses found before were lost: %d left", len(search.found))
	}
}
