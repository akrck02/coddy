package lsp

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/akrck02/code-review/internal/gradle"
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
		_ = command.Wait() // how the server ended is told by its output and exited, not by its status
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
		_ = json.Unmarshal(params, &asked)       // a question that cannot be read gets no answers: the defaults
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

// Exited is closed once the server's process has ended.
func (client *Client) Exited() <-chan struct{} { return client.exited }

// Done is closed once the server has stopped.
func (client *Client) Done() <-chan struct{} { return client.conn.Done() }

// Stop asks the server to shut down and exit, and ends it if it has not within a moment.
func (client *Client) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), stopWait)
	defer cancel()
	if client.conn.Call(ctx, "shutdown", nil, nil) == nil {
		_ = client.conn.Notify("exit", nil) // a server that does not hear it is killed below
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
		_ = client.command.Process.Kill() // a process that already ended needs no killing
	}
}
