package servers

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/work"
)

// quietHost is a window with no projects, which never reads any.
type quietHost struct{ toasts []string }

func (host *quietHost) Loading() bool                           { return false }
func (host *quietHost) Projects() []*codebase.Project           { return nil }
func (host *quietHost) InSight(*codebase.Project) bool          { return true }
func (host *quietHost) LoadOptions(string) codebase.LoadOptions { return codebase.LoadOptions{} }
func (host *quietHost) Toast(words string)                      { host.toasts = append(host.toasts, words) }

// A file of a language no server of the table reads starts nothing, and a question about it says so.
func TestAFileNoServerReadsStartsNothing(test *testing.T) {
	table := []lsp.Server{{Name: "lampls", Command: "no-such-lamp-server-here", Languages: []string{"go"}}}
	fleet := New(Settings{Table: table}, &quietHost{}, work.New(true, func() {}), func() {})
	open := OpenFile{Project: "/lighthouse", Path: "/lighthouse/notes.txt", Document: &codebase.Document{Text: "a note"}}
	if client, _ := fleet.Serve(open); client != nil || fleet.Starting("lampls") {
		test.Fatal("a text file started a server")
	}
	if _, _, why := fleet.Asked(open, true); why != "No language server reads this file" {
		test.Errorf("asking about a text file says %q", why)
	}
}

// A server that is not on the computer is noted missing, told once, and not tried again.
func TestAMissingServerIsToldOnceAndNotTriedAgain(test *testing.T) {
	table := []lsp.Server{{Name: "lampls", Command: "no-such-lamp-server-here", Languages: []string{"go"}, Install: "brew install lampls"}}
	host := &quietHost{}
	fleet := New(Settings{Table: table}, host, work.New(true, func() {}), func() {})
	open := OpenFile{Project: "/lighthouse", Path: "/lighthouse/lamp.go", Document: &codebase.Document{Text: "package lamp"}}
	for range 3 {
		fleet.Serve(open)
		fleet.work.Take()
	}
	if !fleet.Missing("lampls") || len(host.toasts) != 1 {
		test.Fatalf("missing %v, told %q", fleet.Missing("lampls"), host.toasts)
	}
	if _, _, why := fleet.Asked(open, true); why != "lampls is not installed: brew install lampls" {
		test.Errorf("asking says %q", why)
	}
}
