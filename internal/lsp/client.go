package lsp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/akrck02/go-renderer/failure"
)

// Client is one language server running, started and initialized, and what it was told.
type Client struct {
	Server  Server
	Root    string // its root, or "" for one per window
	conn    *Conn
	command *exec.Cmd

	lock      sync.Mutex
	telling   sync.Mutex     // held while a document's text is sent, so texts go out in order
	versions  map[string]int // the documents opened, by URI, with the version last sent
	ready     chan struct{}  // closed once the server has read the projects
	readyOnce sync.Once
	working   map[string]bool // the work the server reports running, by its token
	folders   map[string]bool
}

// Start runs a server in a root (or, with folders, for those workspace folders) and initializes
// it. dataFolder is where it may keep its index, for a server that asks for one.
func Start(ctx context.Context, server Server, root string, folders []string, dataFolder string) (*Client, error) {
	path, err := exec.LookPath(server.Command)
	if err != nil {
		return nil, failure.Wrap(ServerNotFound, server.Name, err)
	}
	arguments := append([]string(nil), server.Arguments...)
	if server.DataFlag != "" && dataFolder != "" {
		arguments = append(arguments, server.DataFlag, dataFolder)
	}
	command := exec.Command(path, arguments...)
	command.Dir = firstOf(root, folders)
	command.Env = append(os.Environ(), server.Environment...)
	input, _ := command.StdinPipe()
	output, _ := command.StdoutPipe()
	if err := command.Start(); err != nil {
		return nil, failure.Wrap(ServerNotFound, server.Name, err)
	}
	client := &Client{Server: server, Root: root, command: command, versions: map[string]int{}, folders: map[string]bool{},
		ready: make(chan struct{}), working: map[string]bool{}}
	if server.ReadyStatus == "" {
		client.becomeReady()
	}
	client.conn = NewConn(output, input, client.handle)
	go func() { _ = command.Wait() }()
	if err := client.initialize(ctx, root, folders); err != nil {
		client.kill()
		return nil, err
	}
	return client, nil
}

// firstOf is the root, or the first folder when there is no root.
func firstOf(root string, folders []string) string {
	if root == "" && len(folders) > 0 {
		return folders[0]
	}
	return root
}

// initialize says who the reader is and what it can do, and waits for the server to be ready.
func (client *Client) initialize(ctx context.Context, root string, folders []string) error {
	if root != "" {
		folders = []string{root}
	}
	workspaceFolders := make([]map[string]string, len(folders))
	for index, folder := range folders {
		workspaceFolders[index] = map[string]string{"uri": FileURI(folder), "name": filepath.Base(folder)}
		client.folders[folder] = true
	}
	params := map[string]any{
		"processId":        os.Getpid(),
		"clientInfo":       map[string]string{"name": "code-review"},
		"rootUri":          FileURI(firstOf(root, folders)),
		"workspaceFolders": workspaceFolders,
		"capabilities":     capabilities,
	}
	if client.Server.Options != nil {
		params["initializationOptions"] = client.Server.Options
	}
	if err := client.conn.Call(ctx, "initialize", params, nil); err != nil {
		return err
	}
	return client.conn.Notify("initialized", struct{}{})
}

// capabilities are what the reader asks of a server: definitions, references, hover as Markdown,
// workspace folders that change, and progress to show while it reads.
var capabilities = map[string]any{
	"textDocument": map[string]any{
		"synchronization": map[string]any{"didSave": false},
		"definition":      map[string]any{"linkSupport": false},
		"references":      map[string]any{},
		"hover":           map[string]any{"contentFormat": []string{"markdown", "plaintext"}},
	},
	"workspace": map[string]any{"workspaceFolders": true, "configuration": true, "applyEdit": false},
	"window":    map[string]any{"workDoneProgress": true},
}

// handle answers what the server asks on its own. It never applies an edit: the reader lets no
// server change a file the person reads or edits.
func (client *Client) handle(method string, params json.RawMessage) (any, error) {
	switch method {
	case "workspace/applyEdit":
		return map[string]any{"applied": false, "failureReason": "code-review does not let a language server edit files"}, nil
	case "$/progress":
		client.noteProgress(params)
	case "language/status":
		var status struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(params, &status) == nil && status.Type == client.Server.ReadyStatus {
			client.becomeReady()
		}
	case "workspace/configuration":
		var asked struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(params, &asked)
		return make([]any, len(asked.Items)), nil // no settings: each server's defaults
	}
	return nil, nil // registrations, progress and messages need no more than a reply
}

// becomeReady marks the server as having read the projects.
func (client *Client) becomeReady() { client.readyOnce.Do(func() { close(client.ready) }) }

// Ready tells whether the server has read the projects and reports no work running: it answers
// requests in full.
func (client *Client) Ready() bool {
	select {
	case <-client.ready:
		return !client.Busy()
	default:
		return false
	}
}

// waitReady waits for the server to have read the projects, as a request asked before would be
// answered with nothing; it gives up with the request's context, or when the server stops.
func (client *Client) waitReady(ctx context.Context) error {
	select {
	case <-client.ready:
		return nil
	case <-ctx.Done():
		return failure.Wrap(RequestFailed, "waiting for "+client.Server.Name+" to read the projects", ctx.Err())
	case <-client.conn.Done():
		return failure.Wrap(ServerStopped, client.Server.Name, client.conn.Err())
	}
}

// Tell tells the server a document's text as the reader has it, edits included, at a version
// that grows with each new text (from 1): didOpen the first time, didChange afterwards. A text
// older than the one sent already is dropped, so texts told from several goroutines never leave
// the server on an old one.
func (client *Client) Tell(path, text string, version int) error {
	uri := FileURI(path)
	client.telling.Lock()
	defer client.telling.Unlock()
	client.lock.Lock()
	sent, open := client.versions[uri]
	if version <= sent {
		client.lock.Unlock()
		return nil
	}
	client.versions[uri] = version
	client.lock.Unlock()
	if open {
		return client.conn.Notify("textDocument/didChange", map[string]any{
			"textDocument":   map[string]any{"uri": uri, "version": version},
			"contentChanges": []map[string]string{{"text": text}},
		})
	}
	return client.conn.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": LanguageOf(path), "version": version, "text": text,
	}})
}

// AddFolder adds a workspace folder to a server that runs for the whole window.
func (client *Client) AddFolder(folder string) error {
	client.lock.Lock()
	known := client.folders[folder]
	client.folders[folder] = true
	client.lock.Unlock()
	if known {
		return nil
	}
	return client.conn.Notify("workspace/didChangeWorkspaceFolders", map[string]any{"event": map[string]any{
		"added":   []map[string]string{{"uri": FileURI(folder), "name": filepath.Base(folder)}},
		"removed": []map[string]string{},
	}})
}

// HasFolder tells whether a server already reads a workspace folder.
func (client *Client) HasFolder(folder string) bool {
	client.lock.Lock()
	defer client.lock.Unlock()
	return client.folders[folder]
}

// Definition is where the name at a place of a document is declared.
func (client *Client) Definition(ctx context.Context, path string, at Position) ([]Location, error) {
	return client.locations(ctx, "textDocument/definition", path, at, nil)
}

// References are every place the name at a place of a document is used, its declaration included.
func (client *Client) References(ctx context.Context, path string, at Position) ([]Location, error) {
	return client.locations(ctx, "textDocument/references", path, at, map[string]bool{"includeDeclaration": true})
}

// locations asks a request whose answer is a location, a list of them, or nothing.
func (client *Client) locations(ctx context.Context, method, path string, at Position, context any) ([]Location, error) {
	params := map[string]any{"textDocument": TextDocumentIdentifier{URI: FileURI(path)}, "position": at}
	if context != nil {
		params["context"] = context
	}
	raw, err := client.ask(ctx, method, params, func(raw json.RawMessage) bool {
		found, _ := decodeLocations(raw)
		return len(found) == 0
	})
	if err != nil {
		return nil, err
	}
	return decodeLocations(raw)
}

// decodeLocations reads a Location, a list of them, or a list of LocationLinks.
func decodeLocations(raw json.RawMessage) ([]Location, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, nil
	}
	if strings.HasPrefix(text, "{") {
		raw = json.RawMessage("[" + text + "]")
	}
	var items []struct {
		Location
		TargetURI   string `json:"targetUri"`
		TargetRange Range  `json:"targetSelectionRange"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, failure.Wrap(BadMessage, "locations", err)
	}
	locations := make([]Location, len(items))
	for index, item := range items {
		locations[index] = item.Location
		if item.TargetURI != "" {
			locations[index] = Location{URI: item.TargetURI, Range: item.TargetRange}
		}
	}
	return locations, nil
}

// Hover is the text the server shows for the name at a place: its type and documentation, as
// Markdown; "" when it has none.
func (client *Client) Hover(ctx context.Context, path string, at Position) (string, error) {
	params := TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: FileURI(path)}, Position: at}
	raw, err := client.ask(ctx, "textDocument/hover", params, func(raw json.RawMessage) bool { return hoverOf(raw) == "" })
	if err != nil {
		return "", err
	}
	return hoverOf(raw), nil
}

// hoverOf is the text of a hover's answer; "" for none.
func hoverOf(raw json.RawMessage) string {
	var answer struct {
		Contents json.RawMessage `json:"contents"`
	}
	if json.Unmarshal(raw, &answer) != nil {
		return ""
	}
	return hoverText(answer.Contents)
}

// hoverText reads a hover's contents: MarkupContent, a MarkedString, or a list of them.
func hoverText(raw json.RawMessage) string {
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return plain
	}
	var markup struct {
		Language string `json:"language"`
		Value    string `json:"value"`
	}
	if json.Unmarshal(raw, &markup) == nil && markup.Value != "" {
		if markup.Language != "" {
			return "```" + markup.Language + "\n" + markup.Value + "\n```"
		}
		return markup.Value
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return ""
	}
	parts := make([]string, 0, len(list))
	for _, item := range list {
		if text := hoverText(item); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// Done is closed once the server has stopped.
func (client *Client) Done() <-chan struct{} { return client.conn.Done() }

// Stop asks the server to shut down and exit, and ends it if it has not within a moment.
func (client *Client) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), stopWait)
	defer cancel()
	if client.conn.Call(ctx, "shutdown", nil, nil) == nil {
		_ = client.conn.Notify("exit", nil)
	}
	select {
	case <-client.conn.Done():
	case <-ctx.Done():
	}
	client.kill()
}

// stopWait is how long a server gets to stop on its own.
const stopWait = 2 * time.Second

// kill ends the server's process, if it still runs.
func (client *Client) kill() {
	if client.command.Process != nil {
		_ = client.command.Process.Kill()
	}
}
