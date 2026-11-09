package window

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/text"
	"github.com/akrck02/code-review/internal/workspace"
)

// languageServers are the language servers the window runs: each starts the first time a file of
// its language is opened, is told about the files opened and their edits, and stops with the window.
type languageServers struct {
	table    []lsp.Server
	running  map[string]*lsp.Client // by key: the server's name, and its root when it has one
	starting map[string]bool
	told     map[string]told   // what each server was told of each file, by key and path
	version  int               // grows with each text told, for every file and server
	missing  map[string]bool   // servers not installed, by name
	failed   map[string]string // servers that did not start, or kept stopping, by name, with why
	crashes  map[string]int    // how many times each server stopped on its own
	stopped  bool              // the window closed: a server that starts now stops at once
	// snapshots are what each project held when a server started in it, by the project's folder;
	// written is what servers wrote, kept across starts of the reader
	snapshots map[string]*workspace.Snapshot
	written   *workspace.Written
	adding    map[string]bool   // projects being added to a server for the whole window
	roots     map[string]string // the root of each file asked, as finding it reads the disk
	// inSight are, by server name, the projects in sight with a file it reads, worked out again
	// when the projects change; restarting is the set a server is started again for, by key
	inSight    map[string][]string
	restarting map[string]string
	// resting are the servers stopped for going unasked, by key with their name (rest.go); rest
	// is how long that takes, and watchingRest tells whether a check is due
	resting      map[string]string
	rest         time.Duration
	watchingRest bool
	// usage is what each running server takes, by key, measured while the panel is open (usage.go)
	usage      map[string]serverUsage
	measuring  bool
	measuredAt time.Time
}

// startWait is how long a server gets to start and answer initialize.
const startWait = 2 * time.Minute

func newLanguageServers(table []lsp.Server, writtenPath string) languageServers {
	if table == nil {
		table = lsp.Servers
	}
	return languageServers{table: table, running: map[string]*lsp.Client{}, starting: map[string]bool{},
		told: map[string]told{}, missing: map[string]bool{}, failed: map[string]string{}, crashes: map[string]int{}, snapshots: map[string]*workspace.Snapshot{},
		written: workspace.OpenWritten(writtenPath), adding: map[string]bool{}, roots: map[string]string{},
		inSight: map[string][]string{}, restarting: map[string]string{}, resting: map[string]string{}, rest: serverRest, usage: map[string]serverUsage{}}
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
	root, known := application.servers.roots[path]
	if !known {
		root = server.RootOf(view.Project.Path, path)
		application.servers.roots[path] = root
	}
	return server, root, path, true
}

// told is the text of a file a server was told: the document read and the buffer editing it, at
// a version of the buffer, and the version it was told at.
type told struct {
	document    *workspace.Document
	buffer      *text.Buffer
	editVersion int
	version     int
}

// serveOpenFile starts the server of the file open if it is not running, and tells it the file's
// text, edits included, whenever that changed: an edit, or the file read again from disk. It runs
// every frame and is cheap when nothing changed. It gives the server and what it was told last.
func (application *Window) serveOpenFile() (*lsp.Client, told, string) {
	servers := &application.servers
	server, root, path, found := application.serverOfOpenFile()
	if !found || servers.missing[server.Name] || servers.failed[server.Name] != "" {
		return nil, told{}, ""
	}
	key := serverKey(server, root)
	client := servers.running[key]
	if client == nil {
		// not while the projects are read: a file shown a moment then would start a server for
		// nothing, and one for the whole window would start without the projects still unread
		if !servers.starting[key] && servers.resting[key] == "" && !application.Loading() {
			application.startServer(server, root, key, application.viewer.Project.Path)
		}
		return nil, told{}, ""
	}
	if server.OnePerWindow {
		application.followSight(client, server, key)
	}
	now, text := application.textNow()
	last := servers.told[key+"|"+path]
	if last.document == now.document && last.buffer == now.buffer && last.editVersion == now.editVersion {
		return client, last, text
	}
	servers.version++
	now.version = servers.version
	servers.told[key+"|"+path] = now
	application.work.run("", func(cancellation) func() {
		_ = client.Tell(path, text, now.version) // an older text, sent late, is dropped
		return nil
	})
	return client, now, text
}

// textNow is the text of the file open, edits included, and what it is made of.
func (application *Window) textNow() (told, string) {
	view := &application.viewer
	now := told{document: view.Document, buffer: view.Edit}
	if view.Edit == nil {
		return now, view.Document.Text
	}
	now.editVersion = view.Edit.Version()
	return now, view.Edit.Text()
}

// startServer starts a server in the background for a project, once what its projects hold is
// noted; once it answers, the next frame tells it the file.
func (application *Window) startServer(server lsp.Server, root, key, project string) {
	servers := &application.servers
	servers.starting[key] = true
	folders := application.foldersFor(server)
	projects := folders
	if !server.OnePerWindow {
		projects = []string{project}
	}
	data := application.dataFolder(server)
	options := application.optionsOf(projects)
	application.work.run("start "+key, func(cancellation) func() {
		removed, _ := lsp.PruneIndexes(data, server, time.Now()) // an index it cannot remove stays
		if why := roomFor(server, data); why != "" {
			return func() {
				delete(servers.starting, key)
				servers.failed[server.Name] = why
				application.toast = server.Name + " did not start: " + why
			}
		}
		snapshots := takeSnapshots(server.Name, options)
		ctx, cancel := context.WithTimeout(context.Background(), startWait)
		defer cancel()
		client, err := lsp.Start(ctx, server, root, folders, data)
		return func() {
			if len(removed) > 0 {
				application.toast = server.Name + ": removed " + plural(len(removed), "old index") + " of projects no longer open"
			}
			if application.serverStarted(server, key, client, err) {
				application.keepSnapshots(snapshots) // a server that never ran wrote nothing
			}
		}
	})
}

// freeSpace is how many bytes are free on a folder's disk; a test gives another.
var freeSpace = lsp.FreeSpace

// roomFor says why a server that asks for free space on its data folder's disk cannot start there;
// "" when it can.
func roomFor(server lsp.Server, data string) string {
	if server.NeedsFree == 0 || data == "" {
		return ""
	}
	free, err := freeSpace(data)
	if err != nil || free >= server.NeedsFree {
		return "" // a disk it cannot measure is not kept from it
	}
	return fmt.Sprintf("it needs %s free on the disk of %s, which has %s (its index takes %s): "+
		"free some, or keep its index on another disk with -server-data", bytesOf(int64(server.NeedsFree)), data,
		bytesOf(int64(free)), bytesOf(lsp.IndexesSize(data, server)))
}

// bytesOf writes a size as a person reads it: "4.1 GB", "300 MB".
func bytesOf(size int64) string {
	switch {
	case size >= 1<<30:
		return strconv.FormatFloat(float64(size)/(1<<30), 'f', 1, 64) + " GB"
	case size >= 1<<20:
		return strconv.Itoa(int(size>>20)) + " MB"
	default:
		return strconv.Itoa(int(size>>10)) + " KB"
	}
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

// serverStarted keeps a server that started, and watches it, or says why it did not; it tells
// whether the server runs.
func (application *Window) serverStarted(server lsp.Server, key string, client *lsp.Client, err error) bool {
	servers := &application.servers
	delete(servers.starting, key)
	switch {
	case errors.Is(err, lsp.ServerNotFound):
		servers.missing[server.Name] = true
		application.toast = server.Name + " is not installed: " + server.Install
	case err != nil:
		servers.failed[server.Name] = err.Error() // not tried again until the reader starts again
		application.toast = server.Name + " did not start: " + err.Error()
	case servers.stopped:
		go client.Stop()
	default:
		servers.running[key] = client
		application.work.whenDone(client.Done(), func() { application.serverEnded(server.Name, key, client) })
		go watchProgress(client)
		application.watchRest()
		return true
	}
	return false
}

// crashLimit is how many times a server may stop on its own before the reader stops starting it.
const crashLimit = 3

// serverEnded forgets a server that stopped on its own, so the next file that needs it starts it
// again; one that keeps stopping is left stopped.
func (application *Window) serverEnded(name, key string, client *lsp.Client) {
	servers := &application.servers
	if servers.stopped || servers.running[key] != client {
		return // the window stopped it
	}
	delete(servers.running, key)
	for told := range servers.told {
		if strings.HasPrefix(told, key+"|") {
			delete(servers.told, told)
		}
	}
	servers.crashes[name]++
	if servers.crashes[name] >= crashLimit {
		servers.failed[name] = "it stopped " + strconv.Itoa(crashLimit) + " times"
		application.toast = name + " stopped again: it is left stopped until the reader starts again"
		return
	}
	application.toast = name + " stopped; it starts again when a file needs it"
}

// foldersFor are the workspace folders of a server that runs for the whole window: every project
// in sight with a file of its languages (every project, for one that reads all), and the project
// of the file open. A server with roots gets its root instead.
func (application *Window) foldersFor(server lsp.Server) []string {
	if !server.OnePerWindow {
		return nil
	}
	folders, worked := application.servers.inSight[server.Name]
	if !worked {
		for _, project := range application.projects {
			if (server.ReadsAll || !application.hidden(project)) && hasFileOf(project, server) {
				folders = append(folders, project.Path)
			}
		}
		application.servers.inSight[server.Name] = folders
	}
	view := &application.viewer
	if view.IsOpen() && view.Project != nil && !slices.Contains(folders, view.Project.Path) {
		folders = append(slices.Clip(folders), view.Project.Path)
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

// dataFolder is where a server that asks for one keeps its index: in the folder the settings
// name (another disk, say), else in the reader's own folder; never in a repository.
func (application *Window) dataFolder(server lsp.Server) string {
	if server.DataFlag == "" {
		return ""
	}
	folder := application.settings.ServerData
	if folder == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		folder = filepath.Join(config, "code-review", "servers")
	}
	return filepath.Join(folder, server.Name)
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
