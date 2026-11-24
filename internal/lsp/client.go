package lsp

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/akrck02/code-review/internal/gradle"
	"github.com/akrck02/code-review/internal/language"
	"github.com/akrck02/go-renderer/failure"
)

// Client is one language server running, started and initialized, and what it was told.
type Client struct {
	Server  Server
	Root    string // its root, or "" for one per window
	conn    *Conn
	command *exec.Cmd
	exited  chan struct{} // closed once its process has ended
	data    string        // its data folder

	lock      sync.Mutex
	telling   sync.Mutex     // held while a document's text is sent, so texts go out in order
	versions  map[string]int // the documents opened, by URI, with the version last sent
	ready     chan struct{}  // closed once the server has read the projects
	readyOnce sync.Once
	working   map[string]*work // the work the server reports running, by its token
	folders   map[string]bool
	// what it says of itself: jdtls's status, the projects kotlin-lsp imported and the one it
	// imports, its messages; changes signals each change
	started     time.Time
	status      string
	statusSince time.Time
	imported    map[string]time.Time // when each project began to be imported
	importing   string
	log         []Message
	readyAt     time.Time
	idleAt      time.Time // when the work it reported last ended
	askedAt     time.Time // when it was last asked something, or answered
	changes     chan struct{}
	// followsFolders tells whether the server says it follows workspace folders added and removed
	// while it runs; one that does not reads those it started with.
	followsFolders bool
	// lastReading is how long the server took to read the projects the time before, to say how
	// long it may take now; released, the Gradle daemons it stopped last, and when
	lastReading time.Duration
	released    int
	releasedAt  time.Time
	// for a server that is given the models of its projects (Models): the folder standing in for
	// each project read from its model, and the fingerprint of the build files of each project it
	// imports with Gradle, to keep its model once imported (models.go)
	standIns  map[string]string
	toImport  map[string]string
	modelsAt  time.Time // when the models were last kept
	modelsErr string    // why they could not be, "" when they were
}

// Start runs a server in a root (or, with folders, for those workspace folders) and initializes
// it. dataFolder is where it may keep its index, for a server that asks for one.
func Start(ctx context.Context, server Server, root string, folders []string, dataFolder string) (*Client, error) {
	path, err := Find(server)
	if err != nil {
		return nil, err
	}
	command := commandOf(path, server, dataFolder)
	command.Dir = firstOf(root, folders)
	input, _ := command.StdinPipe()
	output, _ := command.StdoutPipe()
	if err := command.Start(); err != nil {
		return nil, failure.Wrap(ServerNotFound, server.Name, err)
	}
	client := &Client{Server: server, Root: root, command: command, exited: make(chan struct{}), data: dataFolder, versions: map[string]int{}, folders: map[string]bool{},
		ready: make(chan struct{}), working: map[string]*work{}, started: time.Now(), statusSince: time.Now(),
		imported: map[string]time.Time{}, changes: make(chan struct{}, 1)}
	if server.Import.Models && dataFolder != "" && root == "" {
		client.chooseModels(folders)
	}
	if server.Ready.Status == "" && !server.Ready.AfterWork {
		client.becomeReady()
	}
	client.conn = NewConn(output, input, client.handle)
	go func() {
		_ = command.Wait()
		close(client.exited)
	}()
	if err := client.initialize(ctx, root, folders); err != nil {
		client.kill()
		return nil, err
	}
	client.watch()
	return client, nil
}

// watch starts, once a server is initialized, what follows it while it runs: its readiness, its
// Gradle daemons, the models of the projects it imports, how long it takes to read them.
func (client *Client) watch() {
	server := client.Server
	if server.Import.OwnGradle && client.data != "" {
		go client.releaseGradle()
	}
	if server.Ready.AfterWork {
		go client.readyWithoutWork()
	}
	if len(client.toImport) > 0 {
		go client.keepModels()
	}
	if client.data != "" {
		client.lastReading = LastReading(client.data)
		go client.rememberReading()
	}
}

// commandOf is the command that runs a server: its arguments, its data folder, its memory and its
// Gradle home.
func commandOf(path string, server Server, dataFolder string) *exec.Cmd {
	arguments := append([]string(nil), server.Arguments...)
	if server.DataFlag != "" && dataFolder != "" {
		arguments = append(arguments, server.DataFlag, dataFolder)
	}
	environment := append(os.Environ(), server.Environment...)
	if server.Import.OwnGradle && dataFolder != "" {
		environment = append(environment, gradle.PrepareHome(dataFolder)...)
	}
	switch {
	case server.Memory.Most != "" && server.Memory.Flag != "":
		arguments = append(arguments, server.Memory.Flag+server.Memory.Most)
	case server.Memory.Most != "" && server.Memory.Variable != "":
		options := append([]string{"-Xmx" + server.Memory.Most}, server.Memory.JVMOptions...)
		environment = append(environment, server.Memory.Variable+"="+strings.Join(options, " "))
	}
	command := exec.Command(path, arguments...)
	command.Env = environment
	return command
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
	workspaceFolders, given := make([]map[string]string, len(folders)), make([]string, len(folders))
	for index, folder := range folders {
		given[index] = client.serverFolder(folder)
		workspaceFolders[index] = map[string]string{"uri": FileURI(given[index]), "name": filepath.Base(folder)}
		client.folders[folder] = true
	}
	params := map[string]any{
		"processId":        os.Getpid(),
		"clientInfo":       map[string]string{"name": "code-review"},
		"rootUri":          FileURI(firstOf(root, given)),
		"workspaceFolders": workspaceFolders,
		"capabilities":     capabilities,
	}
	if options := client.initializationOptions(folders); options != nil {
		params["initializationOptions"] = options
	}
	var answer struct {
		Capabilities struct {
			Workspace struct {
				WorkspaceFolders struct {
					Supported bool `json:"supported"`
				} `json:"workspaceFolders"`
			} `json:"workspace"`
		} `json:"capabilities"`
	}
	if err := client.conn.Call(ctx, "initialize", params, &answer); err != nil {
		return err
	}
	client.followsFolders = answer.Capabilities.Workspace.WorkspaceFolders.Supported
	return client.conn.Notify("initialized", struct{}{})
}

// initializationOptions are the server's own options, with its settings when it has some, and its
// workspace folders for a server that reads them there. Its index is the projects', whether each
// is read from its model or imported: a stand-in takes its project's place among the folders, and
// one project's build changing does not make another index.
func (client *Client) initializationOptions(folders []string) map[string]any {
	server := client.Server
	ownIndex := server.Index.Option != "" && client.data != ""
	if server.Settings == nil && !server.FoldersInOptions && !ownIndex && len(client.standIns) == 0 {
		return server.Options
	}
	options := maps.Clone(server.Options)
	if options == nil {
		options = map[string]any{}
	}
	if server.Settings != nil {
		options["settings"] = server.Settings
	}
	if ownIndex {
		options[server.Index.Option] = indexFolder(client.data, server, folders)
	}
	if len(client.standIns) > 0 {
		options["buildTools"] = client.buildTools()
	}
	if server.FoldersInOptions {
		uris := make([]string, len(folders))
		for index, folder := range folders {
			uris[index] = FileURI(client.serverFolder(folder))
		}
		options["workspaceFolders"] = uris
	}
	return options
}

// capabilities are what the reader asks of a server: definitions, references, hover as Markdown,
// workspace folders that change, and progress to show while it reads.
var capabilities = map[string]any{
	"textDocument": map[string]any{
		"synchronization": map[string]any{"didSave": false},
		"definition":      map[string]any{"linkSupport": false},
		"references":      map[string]any{},
		"hover":           map[string]any{"contentFormat": []string{"markdown", "plaintext"}},
		"documentSymbol":  map[string]any{"hierarchicalDocumentSymbolSupport": true},
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
		client.noteStatus(params)
	case "intellij/importLog":
		client.noteImport(params)
	case "window/logMessage", "window/showMessage":
		client.noteLog(params)
	case "workspace/configuration":
		var asked struct {
			Items []struct {
				Section string `json:"section"`
			} `json:"items"`
		}
		_ = json.Unmarshal(params, &asked)
		answers := make([]any, len(asked.Items)) // null: the server's defaults
		for index, item := range asked.Items {
			answers[index] = client.Server.setting(item.Section)
		}
		return answers, nil
	}
	return nil, nil // registrations, progress and messages need no more than a reply
}

// becomeReady marks the server as having read the projects.
func (client *Client) becomeReady() {
	client.readyOnce.Do(func() {
		client.lock.Lock()
		client.readyAt = time.Now()
		client.lock.Unlock()
		close(client.ready)
	})
	client.changed()
}

// isReady tells whether the server said it has read the projects, busy or not.
func (client *Client) isReady() bool {
	select {
	case <-client.ready:
		return true
	default:
		return false
	}
}

// Ready tells whether the server has read the projects and reports no work running: it answers
// requests in full.
func (client *Client) Ready() bool { return client.isReady() && !client.Busy() }

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
		"uri": uri, "languageId": language.ServerLanguageOf(path), "version": version, "text": text,
	}})
}

// IsOpen tells whether a document was told to the server.
func (client *Client) IsOpen(path string) bool {
	client.lock.Lock()
	defer client.lock.Unlock()
	_, open := client.versions[FileURI(path)]
	return open
}

// CloseOlder closes a document told to the server, unless a newer text of it than version was told
// since: a file left for another closes, but not when it was opened again meanwhile.
func (client *Client) CloseOlder(path string, version int) error {
	client.lock.Lock()
	sent, open := client.versions[FileURI(path)]
	client.lock.Unlock()
	if !open || sent > version {
		return nil
	}
	return client.Close(path)
}

// Close tells the server a document told to it is no longer open: it reads it from disk again.
func (client *Client) Close(path string) error {
	uri := FileURI(path)
	client.telling.Lock()
	defer client.telling.Unlock()
	client.lock.Lock()
	_, open := client.versions[uri]
	delete(client.versions, uri)
	client.lock.Unlock()
	if !open {
		return nil
	}
	return client.conn.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}})
}

// AddFolder adds a workspace folder to a server that runs for the whole window.
func (client *Client) AddFolder(folder string) error { return client.changeFolder(folder, true) }

// RemoveFolder takes a workspace folder away from a server that runs for the whole window.
func (client *Client) RemoveFolder(folder string) error { return client.changeFolder(folder, false) }

// changeFolder adds or removes a workspace folder, when it is not so already.
func (client *Client) changeFolder(folder string, add bool) error {
	client.lock.Lock()
	known := client.folders[folder]
	if add {
		client.folders[folder] = true
	} else {
		delete(client.folders, folder)
	}
	client.lock.Unlock()
	if known == add {
		return nil
	}
	changed := []map[string]string{{"uri": FileURI(client.serverFolder(folder)), "name": filepath.Base(folder)}}
	event := map[string]any{"added": changed, "removed": []map[string]string{}}
	if !add {
		event = map[string]any{"added": []map[string]string{}, "removed": changed}
	}
	return client.conn.Notify("workspace/didChangeWorkspaceFolders", map[string]any{"event": event})
}

// FollowsFolders tells whether the server follows workspace folders added and removed while it
// runs; one that does not must be started again to read others.
func (client *Client) FollowsFolders() bool { return client.followsFolders }

// Folders are the workspace folders the server reads, sorted.
func (client *Client) Folders() []string {
	client.lock.Lock()
	defer client.lock.Unlock()
	folders := make([]string, 0, len(client.folders))
	for folder := range client.folders {
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	return folders
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

// Exited is closed once the server's process has ended.
func (client *Client) Exited() <-chan struct{} { return client.exited }

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
	case <-client.exited:
	case <-ctx.Done():
	}
	client.kill()
	if client.Server.Import.OwnGradle && client.data != "" {
		gradle.StopDaemons(client.data)
	}
}

// stopWait is how long a server gets to stop on its own: jdtls saves its workspace as it stops
// (2.4 s on the big workspace), and one killed before it is done imports every Gradle project again next time.
const stopWait = 30 * time.Second

// kill ends the server's process, if it still runs.
func (client *Client) kill() {
	if client.command.Process != nil {
		_ = client.command.Process.Kill()
	}
}
