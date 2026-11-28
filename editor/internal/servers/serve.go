package servers

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/language"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/text"
)

// OpenFile is the file a person has open, as its server is told it. Path is "" while the file
// open is none a server can be told of: not on disk, or not editable.
type OpenFile struct {
	Project  string // the folder of the project of the file open, "" for none
	Path     string // the file on disk
	Document *codebase.Document
	Buffer   *text.Buffer // what edits it, nil while it is not edited
}

// Text is the text of the file open, edits included: built whole, so only when it is sent.
func (open OpenFile) Text() string {
	if open.Buffer == nil {
		return open.Document.Text
	}
	return open.Buffer.Text()
}

// told is the text of a file a server was told: its path, the document read and the buffer
// editing it, at a version of the buffer, and the version it was told at.
type told struct {
	path        string
	document    *codebase.Document
	buffer      *text.Buffer
	editVersion int
	version     int
}

// toldOf is what the file open is made of now, to tell whether its text changed since it was told.
func toldOf(open OpenFile) told {
	now := told{path: open.Path, document: open.Document, buffer: open.Buffer}
	if open.Buffer != nil {
		now.editVersion = open.Buffer.Version()
	}
	return now
}

// sameText tells whether two told texts are of the same file, document, buffer and edit.
func (now told) sameText(last told) bool {
	return now.path == last.path && now.document == last.document && now.buffer == last.buffer &&
		now.editVersion == last.editVersion
}

// serverFor is the server for a file of a project, and its root, when one reads it.
func (fleet *Fleet) serverFor(project, path string) (server lsp.Server, root string, found bool) {
	server, found = lsp.ServerFor(fleet.table, language.ServerLanguageOf(path))
	if !found {
		return lsp.Server{}, "", false
	}
	root, known := fleet.roots[path]
	if !known {
		root = server.RootOf(project, path)
		fleet.roots[path] = root
	}
	return server, root, true
}

// usable tells whether a server can run: it is installed and did not fail.
func (fleet *Fleet) usable(server lsp.Server) bool {
	return !fleet.missing[server.Name] && fleet.failed[server.Name] == ""
}

// Serve starts the server of the file open if it is not running, and tells it the file's text,
// edits included, whenever that changed: an edit, or the file read again from disk. It runs every
// frame and costs nothing when nothing changed: the text is only built to be sent. The file told
// before is closed on the server, so it keeps the open file only. It gives the server and the
// version of the text it was told last.
func (fleet *Fleet) Serve(open OpenFile) (*lsp.Client, int) {
	fleet.openProject = open.Project
	if open.Path == "" {
		return nil, 0
	}
	server, root, found := fleet.serverFor(open.Project, open.Path)
	if !found || !fleet.usable(server) {
		return nil, 0
	}
	key := Key(server, root)
	client := fleet.running[key]
	if client == nil {
		// not while the projects are read: a file shown a moment then would start a server for
		// nothing, and one for the whole window would start without the projects still unread
		if !fleet.starting[key] && fleet.resting[key] == "" && !fleet.host.Loading() {
			fleet.Start(server, root, key, open.Project)
		}
		return nil, 0
	}
	if server.OnePerWindow {
		fleet.followSight(client, server, key)
	}
	return client, fleet.tell(client, key, open)
}

// tell tells a running server the file open, when its text changed since it was told, and gives
// the version it was told at.
func (fleet *Fleet) tell(client *lsp.Client, key string, open OpenFile) int {
	now, last := toldOf(open), fleet.told[key]
	if now.sameText(last) {
		return last.version
	}
	text := open.Text()
	fleet.version++
	now.version = fleet.version
	fleet.told[key] = now
	fleet.work.Run("", func(context.Context) func() {
		if last.path != "" && last.path != now.path {
			_ = client.CloseOlder(last.path, last.version) // a server that stopped meanwhile has nothing open to close
		}
		_ = client.Tell(now.path, text, now.version) // an older text, sent late, is dropped
		return nil
	})
	return now.version
}

// Asked is the server of the file open, running, to ask it something, and the version of the text
// it was told; or why there is none. A resting server starts again when waking, as for a question
// asked; not for a pointer resting.
func (fleet *Fleet) Asked(open OpenFile, waking bool) (*lsp.Client, int, string) {
	server, root, found := lsp.Server{}, "", false
	if open.Path != "" {
		server, root, found = fleet.serverFor(open.Project, open.Path)
	}
	if found && waking {
		fleet.Wake(Key(server, root))
	}
	switch {
	case !found:
		return nil, 0, "No language server reads this file"
	case fleet.missing[server.Name]:
		return nil, 0, server.Name + " is not installed: " + server.Install
	case fleet.failed[server.Name] != "":
		return nil, 0, server.Name + " did not start: " + fleet.failed[server.Name]
	}
	client, version := fleet.Serve(open)
	switch {
	case client != nil:
		return client, version, ""
	case fleet.host.Loading():
		return nil, 0, server.Name + " starts once the projects are read"
	case fleet.resting[Key(server, root)] != "":
		return nil, 0, server.Name + " is resting: follow a name to wake it"
	}
	return nil, 0, server.Name + " is starting: try again in a moment"
}

// ClientFor is the running server of a file of a project, starting it when it is not, and its
// key; reads is false when no server reads the file, or its server is not installed or failed.
func (fleet *Fleet) ClientFor(project, path string) (client *lsp.Client, key string, reads bool) {
	server, found := lsp.ServerFor(fleet.table, language.ServerLanguageOf(path))
	if !found || !fleet.usable(server) {
		return nil, "", false
	}
	server, root, _ := fleet.serverFor(project, path)
	key = Key(server, root)
	fleet.Wake(key)
	client = fleet.running[key]
	if client == nil && !fleet.starting[key] && !fleet.host.Loading() {
		fleet.Start(server, root, key, project)
	}
	return client, key, true
}
