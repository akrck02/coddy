// Package servers runs the language servers of a window: each starts the first time a file of its
// language is opened, is told about the file open and its edits, rests when nobody asks it
// anything, and stops with the window. The window gives what the servers need through Host.
package servers

import (
	"sync"
	"time"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/work"
)

// Host is what the servers need of the window that runs them.
type Host interface {
	// Loading tells whether projects are being read: a server waits for them before it starts.
	Loading() bool
	// Projects are the projects open, in the window's order.
	Projects() []*codebase.Project
	// InSight tells whether a project is shown, so a server for the whole window reads it.
	InSight(project *codebase.Project) bool
	// LoadOptions are how a project is read, to note what it holds before a server writes in it.
	LoadOptions(path string) codebase.LoadOptions
	// Toast tells the person something for a moment.
	Toast(words string)
}

// Settings are how a Fleet runs its servers.
type Settings struct {
	// Table is the servers to run, found on the PATH: those of the plugins on.
	Table []lsp.Server
	// Wrote is the file that keeps what servers wrote in the repositories.
	Wrote string
	// Data is the folder the servers keep their indexes in, one folder each; "" is the reader's own.
	Data string
}

// Fleet is the language servers a window runs, by key: the server's name, and its root when it has
// one (see Key). Its methods run on the interface's goroutine; slow work goes to the queue.
type Fleet struct {
	// Rest is how long a server may go unasked before it rests (rest.go); New sets DefaultRest.
	Rest time.Duration
	// FreeSpace is how many bytes are free on a folder's disk; New sets lsp.FreeSpace.
	FreeSpace func(folder string) (uint64, error)

	host     Host
	work     *work.Queue
	wake     func()
	table    []lsp.Server
	data     string
	running  map[string]*lsp.Client
	starting map[string]bool
	told     map[string]told // the file each server was told last, by key: the one open
	version  int             // grows with each text told, for every file and server
	missing  map[string]bool // servers not installed, by name
	// installable are the missing servers whose install command can run here, by name;
	// installing those being installed, and installFailed why an install did not work
	installable   map[string]bool
	installing    map[string]bool
	installFailed map[string]string
	failed        map[string]string // servers that did not start, or kept stopping, by name, with why
	crashes       map[string]int    // how many times each server stopped on its own
	stopped       bool              // the window closed: a server that starts now stops at once
	// snapshots are what each project held when a server started in it, by the project's folder;
	// written is what servers wrote, kept across starts of the reader
	snapshots map[string]*codebase.Snapshot
	written   *codebase.Written
	adding    map[string]bool   // projects being added to a server for the whole window
	roots     map[string]string // the root of each file asked, as finding it reads the disk
	// openProject is the folder of the project of the file open, as the last Serve was told
	openProject string
	// inSight are, by server name, the projects in sight with a file it reads, worked out again
	// when the projects change; restarting is the set a server is started again for, by key
	inSight    map[string][]string
	restarting map[string]string
	// resting are the servers stopped for going unasked, by key with their name (rest.go);
	// watchingRest tells whether a check is due
	resting      map[string]string
	watchingRest bool
	// usage is what each running server takes, by key, measured while asked (usage.go)
	usage      map[string]usage
	measuring  bool
	measuredAt time.Time
	// queued are the servers waiting for another to have imported the projects, by name, with
	// the other's
	queued map[string]string
	// stopping are the servers stopped that have not ended yet, by key: one starts again only once
	// the other has, as jdtls saves its workspace in the data folder they share as it stops
	stopping map[string]bool
	// cannotImport says, by project folder, why a server that imports builds leaves it out ("" for
	// none), worked out once per project as it reads the disk
	cannotImport map[string]string
	// revision grows each time a map the views are made from changes (touched), and shown keeps
	// the views made last with what they were made from (view.go)
	revision int
	shown    shownViews
}

// New is a fleet with no server running yet. Its jobs run on a queue; wake redraws the window
// when a server's progress changes.
func New(settings Settings, host Host, queue *work.Queue, wake func()) *Fleet {
	return &Fleet{Rest: DefaultRest, FreeSpace: lsp.FreeSpace, host: host, work: queue, wake: wake, table: settings.Table,
		data: settings.Data, running: map[string]*lsp.Client{}, starting: map[string]bool{}, told: map[string]told{},
		missing: map[string]bool{}, installable: map[string]bool{}, installing: map[string]bool{},
		installFailed: map[string]string{}, failed: map[string]string{}, crashes: map[string]int{},
		snapshots: map[string]*codebase.Snapshot{}, written: codebase.OpenWritten(settings.Wrote),
		adding: map[string]bool{}, roots: map[string]string{}, inSight: map[string][]string{},
		restarting: map[string]string{}, resting: map[string]string{}, usage: map[string]usage{},
		queued: map[string]string{}, stopping: map[string]bool{}, cannotImport: map[string]string{}}
}

// Key names a server running in a root ("" for one per window).
func Key(server lsp.Server, root string) string {
	if root == "" {
		return server.Name
	}
	return server.Name + " " + root
}

// Named is the server of a name in the fleet's table.
func (fleet *Fleet) Named(name string) (lsp.Server, bool) {
	for _, server := range fleet.table {
		if server.Name == name {
			return server, true
		}
	}
	return lsp.Server{}, false
}

// Reads tells whether a server of the table reads a language, by the protocol's name of it.
func (fleet *Fleet) Reads(language string) bool {
	_, found := lsp.ServerFor(fleet.table, language)
	return found
}

// Running is the server running for a key, nil for none.
func (fleet *Fleet) Running(key string) *lsp.Client { return fleet.running[key] }

// RunningCount is how many servers run.
func (fleet *Fleet) RunningCount() int { return len(fleet.running) }

// Starting tells whether the server of a key is starting.
func (fleet *Fleet) Starting(key string) bool { return fleet.starting[key] }

// Missing tells whether a server, by name, is not installed.
func (fleet *Fleet) Missing(name string) bool { return fleet.missing[name] }

// Failed says why a server, by name, did not start or was left stopped; "" when it was not.
func (fleet *Fleet) Failed(name string) string { return fleet.failed[name] }

// Crashes is how many times a server, by name, stopped on its own.
func (fleet *Fleet) Crashes(name string) int { return fleet.crashes[name] }

// Snapshot is what a project held when a server first started in it, nil when none has.
func (fleet *Fleet) Snapshot(project string) *codebase.Snapshot { return fleet.snapshots[project] }

// Written is what servers wrote in the repositories, kept across starts of the reader.
func (fleet *Fleet) Written() *codebase.Written { return fleet.written }

// CannotImport says why the servers that import builds leave a project out, by its folder; "" when
// they read it, or it was not looked at yet.
func (fleet *Fleet) CannotImport(project string) string { return fleet.cannotImport[project] }

// ProjectsChanged forgets what is worked out from the projects: the files' roots, as a go.mod may
// have come or gone, and the projects each server reads.
func (fleet *Fleet) ProjectsChanged() {
	clear(fleet.roots)
	clear(fleet.inSight)
}

// Stop stops every server, together, and waits for them.
func (fleet *Fleet) Stop() {
	fleet.stopped = true
	var waiting sync.WaitGroup
	for _, client := range fleet.running {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			client.Stop()
		}()
	}
	waiting.Wait()
	fleet.running = map[string]*lsp.Client{}
	fleet.touched()
}
