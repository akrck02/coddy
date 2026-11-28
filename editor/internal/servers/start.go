package servers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/english"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/programs"
)

// startWait is how long a server gets to start and answer initialize.
const startWait = 2 * time.Minute

// crashLimit is how many times a server may stop on its own before the reader stops starting it.
const crashLimit = 3

// Start starts a server in the background for a project, once what its projects hold is noted;
// once it answers, the next frame tells it the file. One that waits for another is queued.
func (fleet *Fleet) Start(server lsp.Server, root, key, project string) {
	if other := fleet.importingBefore(server); fleet.stopping[key] || other != "" {
		// started on a later frame, once the one before has ended or the other is ready
		fleet.queued[server.Name] = other // "" while the one before ends: the status shows no wait
		fleet.touched()
		return
	}
	delete(fleet.queued, server.Name)
	fleet.starting[key] = true
	fleet.touched()
	folders := fleet.foldersFor(server)
	projects := folders
	if !server.OnePerWindow {
		projects = []string{project}
	}
	data := fleet.dataFolder(server)
	options := fleet.optionsOf(projects)
	fleet.work.Run("start "+key, func(context.Context) func() {
		removed, _ := lsp.PruneIndexes(data, server, time.Now()) // an index it cannot remove stays
		if why := fleet.roomFor(server, data); why != "" {
			return func() {
				delete(fleet.starting, key)
				fleet.failed[server.Name] = why
				fleet.touched()
				fleet.host.Toast(server.Name + " did not start: " + why)
			}
		}
		snapshots := takeSnapshots(server.Name, options)
		requestContext, cancel := context.WithTimeout(context.Background(), startWait)
		defer cancel()
		client, err := lsp.Start(requestContext, server, root, folders, data)
		return func() {
			if len(removed) > 0 {
				fleet.host.Toast(server.Name + ": removed " + english.Count(len(removed), "old index") + " of projects no longer open")
			}
			if fleet.started(server, key, client, err) {
				fleet.keepSnapshots(snapshots) // a server that never ran wrote nothing
			}
		}
	})
}

// roomFor says why a server that asks for free space on its data folder's disk cannot start there;
// "" when it can.
func (fleet *Fleet) roomFor(server lsp.Server, data string) string {
	if server.Index.NeedsFree == 0 || data == "" {
		return ""
	}
	free, err := fleet.FreeSpace(data)
	if err != nil || free >= server.Index.NeedsFree {
		return "" // a disk it cannot measure is not kept from it
	}
	return fmt.Sprintf("it needs %s free on the disk of %s, which has %s (its index takes %s): "+
		"free some, or keep its index on another disk with -server-data", bytesOf(int64(server.Index.NeedsFree)), data,
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
func (fleet *Fleet) optionsOf(projects []string) map[string]codebase.LoadOptions {
	options := map[string]codebase.LoadOptions{}
	for _, path := range projects {
		options[path] = fleet.host.LoadOptions(path)
	}
	return options
}

// takeSnapshots reads projects afresh and notes what they hold, before a server starts in them.
func takeSnapshots(server string, options map[string]codebase.LoadOptions) map[string]*codebase.Snapshot {
	snapshots := map[string]*codebase.Snapshot{}
	for path, asked := range options {
		snapshot := codebase.TakeSnapshot(codebase.LoadProject(context.Background(), path, asked), server)
		snapshots[path] = &snapshot
	}
	return snapshots
}

// keepSnapshots keeps the first snapshot of each project: a later one would take what a server
// wrote already as the person's.
func (fleet *Fleet) keepSnapshots(snapshots map[string]*codebase.Snapshot) {
	for path, snapshot := range snapshots {
		if _, taken := fleet.snapshots[path]; !taken {
			fleet.snapshots[path] = snapshot
		}
	}
}

// started keeps a server that started, and watches it, or says why it did not; it tells whether
// the server runs.
func (fleet *Fleet) started(server lsp.Server, key string, client *lsp.Client, err error) bool {
	delete(fleet.starting, key)
	fleet.touched()
	switch {
	case errors.Is(err, lsp.ServerNotFound):
		fleet.missing[server.Name] = true
		fleet.installable[server.Name] = programs.Runnable(server.Install)
		fleet.host.Toast(server.Name + " is not installed: " + server.Install)
	case err != nil:
		fleet.failed[server.Name] = err.Error() // not tried again until the reader starts again
		fleet.host.Toast(server.Name + " did not start: " + err.Error())
	case fleet.stopped:
		go client.Stop()
	default:
		fleet.running[key] = client
		fleet.work.WhenDone(client.Done(), func() { fleet.ended(server.Name, key, client) })
		go fleet.watchProgress(client)
		fleet.watchRest()
		return true
	}
	return false
}

// ended forgets a server that stopped on its own, so the next file that needs it starts it again;
// one that keeps stopping is left stopped.
func (fleet *Fleet) ended(name, key string, client *lsp.Client) {
	if fleet.stopped || fleet.running[key] != client {
		return // the window stopped it
	}
	delete(fleet.running, key)
	fleet.touched()
	delete(fleet.told, key)
	fleet.crashes[name]++
	if fleet.crashes[name] >= crashLimit {
		fleet.failed[name] = "it stopped " + strconv.Itoa(crashLimit) + " times"
		fleet.host.Toast(name + " stopped again: it is left stopped until the reader starts again")
		return
	}
	fleet.host.Toast(name + " stopped; it starts again when a file needs it")
}

// importingBefore is a server this one waits for that is starting, or importing the projects,
// "" for none.
func (fleet *Fleet) importingBefore(server lsp.Server) string {
	for key := range fleet.starting {
		if slices.Contains(server.Import.After, strings.Fields(key)[0]) {
			return strings.Fields(key)[0]
		}
	}
	for key, client := range fleet.running {
		if slices.Contains(server.Import.After, client.Server.Name) && !client.Ready() {
			return strings.Fields(key)[0]
		}
	}
	return ""
}

// dataFolder is where a server that asks for one keeps its index: in the folder the settings
// name (another disk, say), else in the reader's own folder; never in a repository.
func (fleet *Fleet) dataFolder(server lsp.Server) string {
	if server.DataFlag == "" {
		return ""
	}
	folder := fleet.data
	if folder == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		folder = filepath.Join(config, "coddy", "servers")
	}
	return filepath.Join(folder, server.Name)
}

// stop stops a server in the background, and notes it stopping until its process has ended.
func (fleet *Fleet) stop(key string, client *lsp.Client) {
	fleet.stopping[key] = true
	go client.Stop()
	fleet.work.WhenDone(client.Exited(), func() { delete(fleet.stopping, key) })
}
