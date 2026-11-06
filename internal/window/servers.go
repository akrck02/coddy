package window

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/workspace"
)

// languageServers are the language servers the window runs: each starts the first time a file of
// its language is opened, is told about the files opened and their edits, and stops with the window.
type languageServers struct {
	table    []lsp.Server
	running  map[string]*lsp.Client // by key: the server's name, and its root when it has one
	starting map[string]bool
	told     map[string]int  // the version of each file each server was told, by key and path
	missing  map[string]bool // servers not installed, by name, said once
	stopped  bool            // the window closed: a server that starts now stops at once
	// snapshots are what each project held when a server started in it, by the project's folder;
	// written is what servers wrote, kept across starts of the reader
	snapshots map[string]*workspace.Snapshot
	written   *workspace.Written
	adding    map[string]bool // projects being added to a server for the whole window
}

// startWait is how long a server gets to start and answer initialize.
const startWait = 2 * time.Minute

func newLanguageServers(table []lsp.Server, writtenPath string) languageServers {
	if table == nil {
		table = lsp.Servers
	}
	return languageServers{table: table, running: map[string]*lsp.Client{}, starting: map[string]bool{},
		told: map[string]int{}, missing: map[string]bool{}, snapshots: map[string]*workspace.Snapshot{},
		written: workspace.OpenWritten(writtenPath), adding: map[string]bool{}}
}

// serverKey names a server running in a root ("" for one per window).
func serverKey(server lsp.Server, root string) string {
	if root == "" {
		return server.Name
	}
	return server.Name + " " + root
}

// serverOfOpenFile is the server for the file open, its root and path on disk, when one reads it
// and the file is on disk.
func (application *Window) serverOfOpenFile() (server lsp.Server, root, path string, found bool) {
	view := &application.viewer
	if !view.IsOpen() || view.Document == nil || view.Project == nil || !view.Document.Editable {
		return lsp.Server{}, "", "", false
	}
	path = filepath.Join(view.Project.Path, filepath.FromSlash(view.Reference.Path))
	server, found = lsp.ServerFor(application.servers.table, lsp.LanguageOf(path))
	if !found {
		return lsp.Server{}, "", "", false
	}
	return server, server.RootOf(view.Project.Path, path), path, true
}

// serveOpenFile starts the server of the file open if it is not running, and tells it the file's
// text, edits included, whenever that changed. It runs every frame and is cheap when nothing did.
func (application *Window) serveOpenFile() {
	servers := &application.servers
	server, root, path, found := application.serverOfOpenFile()
	if !found || servers.missing[server.Name] {
		return
	}
	key := serverKey(server, root)
	client := servers.running[key]
	if client == nil {
		if !servers.starting[key] {
			application.startServer(server, root, key)
		}
		return
	}
	if project := application.viewer.Project.Path; server.OnePerWindow && !servers.adding[project] && !client.HasFolder(project) {
		application.addFolder(client, server.Name, project)
	}
	version := 1
	if application.viewer.Edit != nil {
		version += application.viewer.Edit.Version()
	}
	if servers.told[key+"|"+path] == version {
		return
	}
	servers.told[key+"|"+path] = version
	text := application.viewer.Document.Text
	if application.viewer.Edit != nil {
		text = application.viewer.Edit.Text()
	}
	application.work.run("tell "+key+"|"+path, func(cancellation) func() {
		_ = client.Open(path, text)
		return nil
	})
}

// startServer starts a server in the background, once what its projects hold is noted; once it
// answers, the next frame tells it the file.
func (application *Window) startServer(server lsp.Server, root, key string) {
	servers := &application.servers
	servers.starting[key] = true
	folders := application.foldersFor(server)
	projects := folders
	if !server.OnePerWindow {
		projects = []string{application.viewer.Project.Path}
	}
	data := dataFolder(server)
	options := application.optionsOf(projects)
	application.work.run("start "+key, func(cancellation) func() {
		snapshots := takeSnapshots(server.Name, options)
		ctx, cancel := context.WithTimeout(context.Background(), startWait)
		defer cancel()
		client, err := lsp.Start(ctx, server, root, folders, data)
		return func() {
			application.keepSnapshots(snapshots)
			application.serverStarted(server, key, client, err)
		}
	})
}

// optionsOf are how each of some projects is read, worked out on the interface's goroutine for a job.
func (application *Window) optionsOf(projects []string) map[string]workspace.LoadOptions {
	options := map[string]workspace.LoadOptions{}
	for _, path := range projects {
		options[path] = application.loadOptions(path)
	}
	return options
}

// takeSnapshots reads projects afresh and notes what they hold, before a server starts in them.
func takeSnapshots(server string, options map[string]workspace.LoadOptions) map[string]*workspace.Snapshot {
	snapshots := map[string]*workspace.Snapshot{}
	for path, asked := range options {
		snapshot := workspace.TakeSnapshot(workspace.LoadProject(path, asked), server)
		snapshots[path] = &snapshot
	}
	return snapshots
}

// addFolder tells a server for the whole window of one more project, once what it holds is noted.
func (application *Window) addFolder(client *lsp.Client, server, project string) {
	application.servers.adding[project] = true
	options := application.optionsOf([]string{project})
	application.work.run("add "+project, func(cancellation) func() {
		snapshots := takeSnapshots(server, options)
		_ = client.AddFolder(project)
		return func() {
			delete(application.servers.adding, project)
			application.keepSnapshots(snapshots)
		}
	})
}

// keepSnapshots keeps the first snapshot of each project: a later one would take what a server
// wrote already as the person's.
func (application *Window) keepSnapshots(snapshots map[string]*workspace.Snapshot) {
	for path, snapshot := range snapshots {
		if _, taken := application.servers.snapshots[path]; !taken {
			application.servers.snapshots[path] = snapshot
		}
	}
}

// serverStarted keeps a server that started, or says why it did not.
func (application *Window) serverStarted(server lsp.Server, key string, client *lsp.Client, err error) {
	servers := &application.servers
	delete(servers.starting, key)
	switch {
	case errors.Is(err, lsp.ServerNotFound):
		servers.missing[server.Name] = true
		application.toast = server.Name + " is not installed: " + server.Install
	case err != nil:
		servers.missing[server.Name] = true // not tried again until the reader starts again
		application.toast = server.Name + " did not start: " + err.Error()
	case servers.stopped:
		go client.Stop()
	default:
		servers.running[key] = client
	}
}

// foldersFor are the workspace folders of a server that runs for the whole window: every project
// with a file of its languages. A server with roots gets its root instead.
func (application *Window) foldersFor(server lsp.Server) []string {
	if !server.OnePerWindow {
		return nil
	}
	var folders []string
	for _, project := range application.projects {
		if hasFileOf(project, server) {
			folders = append(folders, project.Path)
		}
	}
	return folders
}

// hasFileOf tells whether a project has a file a server reads.
func hasFileOf(project *workspace.Project, server lsp.Server) bool {
	for _, path := range project.Files {
		for _, language := range server.Languages {
			if lsp.LanguageOf(path) == language {
				return true
			}
		}
	}
	return false
}

// dataFolder is where a server that asks for one keeps its index: in the reader's own folder,
// never in a repository.
func dataFolder(server lsp.Server) string {
	if server.DataFlag == "" {
		return ""
	}
	folder, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(folder, "code-review", "servers", server.Name)
}

// stopServers stops every server, together, and waits for them.
func (application *Window) stopServers() {
	servers := &application.servers
	servers.stopped = true
	var waiting sync.WaitGroup
	for _, client := range servers.running {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			client.Stop()
		}()
	}
	waiting.Wait()
	servers.running = map[string]*lsp.Client{}
}

// serverStatus is what the status bar says of the servers: those starting, those running, and
// those not installed.
func (application *Window) serverStatus() string {
	servers := &application.servers
	var parts []string
	for key := range servers.starting {
		parts = append(parts, strings.Fields(key)[0]+" starting…")
	}
	for key := range servers.running {
		parts = append(parts, strings.Fields(key)[0])
	}
	for name := range servers.missing {
		parts = append(parts, name+" not installed")
	}
	sort.Strings(parts)
	return strings.Join(unique(parts), " · ")
}

// unique drops the repeats of a sorted list.
func unique(sorted []string) []string {
	var kept []string
	for index, each := range sorted {
		if index == 0 || each != sorted[index-1] {
			kept = append(kept, each)
		}
	}
	return kept
}
