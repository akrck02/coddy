package servers

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/gradle"
	"github.com/akrck02/code-review/internal/language"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/workspace"
)

// restartPause is how long the projects in sight must stay the same before a server that does not
// follow workspace folders is started again for them: a few clicks of the filter restart it once.
const restartPause = 2 * time.Second

// followSight keeps a server for the whole window reading the projects in sight: a server that
// follows workspace folders is told of those that came and went; one that does not is started
// again for them, once the projects are read and the set has stayed the same a moment.
func (fleet *Fleet) followSight(client *lsp.Client, server lsp.Server, key string) {
	wanted := fleet.foldersFor(server)
	if client.FollowsFolders() {
		fleet.tellSight(client, server, wanted)
		return
	}
	reading := client.Folders()
	slices.Sort(wanted)
	set := strings.Join(wanted, "\n")
	if fleet.host.Loading() || slices.Equal(wanted, reading) || fleet.restarting[key] == set {
		return
	}
	if !client.Ready() {
		return // never in the middle of an import, which a restart throws away: once it is done
	}
	fleet.restarting[key] = set
	fleet.work.Run("restart "+key, func(stop context.Context) func() {
		if !fleet.work.Synchronous() {
			select {
			case <-time.After(restartPause):
			case <-stop.Done():
				return nil
			}
		}
		return func() { fleet.restart(server, key, client) }
	})
}

// tellSight adds to a server the projects that came into sight, and takes away those that left.
func (fleet *Fleet) tellSight(client *lsp.Client, server lsp.Server, wanted []string) {
	for _, project := range wanted {
		if !fleet.adding[project] && !client.HasFolder(project) {
			fleet.addFolder(client, server.Name, project)
		}
	}
	for _, project := range client.Folders() {
		if !slices.Contains(wanted, project) && !fleet.adding[project] {
			fleet.work.Run("", func(context.Context) func() {
				_ = client.RemoveFolder(project) // a server that stopped meanwhile reads nothing
				return nil
			})
		}
	}
}

// addFolder tells a server for the whole window of one more project, once what it holds is noted.
func (fleet *Fleet) addFolder(client *lsp.Client, server, project string) {
	fleet.adding[project] = true
	options := fleet.optionsOf([]string{project})
	fleet.work.Run("add "+project, func(context.Context) func() {
		snapshots := takeSnapshots(server, options)
		_ = client.AddFolder(project) // a server that stopped meanwhile is told again as it starts
		return func() {
			delete(fleet.adding, project)
			fleet.keepSnapshots(snapshots)
		}
	})
}

// restart stops a server that does not follow workspace folders, so the next frame that needs it
// starts it for the projects in sight. Its index stays in its data folder.
func (fleet *Fleet) restart(server lsp.Server, key string, client *lsp.Client) {
	delete(fleet.restarting, key)
	wanted := fleet.foldersFor(server)
	slices.Sort(wanted)
	if fleet.running[key] != client || slices.Equal(wanted, client.Folders()) {
		return // it stopped meanwhile, or the projects came back to those it reads
	}
	delete(fleet.running, key)
	delete(fleet.told, key)
	fleet.host.Toast(server.Name + " starts again to read the " + strconv.Itoa(len(wanted)) + " projects in sight")
	fleet.stop(key, client)
}

// foldersFor are the workspace folders of a server that runs for the whole window: every project
// in sight with a file of its languages (every project, for one that reads all), and the project
// of the file open. A server with roots gets its root instead.
func (fleet *Fleet) foldersFor(server lsp.Server) []string {
	if !server.OnePerWindow {
		return nil
	}
	folders, worked := fleet.inSight[server.Name]
	if !worked {
		for _, project := range fleet.host.Projects() {
			if (server.ReadsAll || fleet.host.InSight(project)) && hasFileOf(project, server) && fleet.importable(server, project.Path) {
				folders = append(folders, project.Path)
			}
		}
		fleet.inSight[server.Name] = folders
	}
	open := fleet.openProject
	if open != "" && !slices.Contains(folders, open) && fleet.importable(server, open) {
		folders = append(slices.Clip(folders), open)
	}
	return folders
}

// importable tells whether a server can read a project: one that imports Gradle builds leaves out
// those it would fail to import each time it starts (gradle.CannotImport), noting why.
func (fleet *Fleet) importable(server lsp.Server, project string) bool {
	if !server.Import.OwnGradle {
		return true
	}
	why, known := fleet.cannotImport[project]
	if !known {
		why = gradle.CannotImport(project)
		fleet.cannotImport[project] = why
	}
	return why == ""
}

// hasFileOf tells whether a project has a file a server reads.
func hasFileOf(project *workspace.Project, server lsp.Server) bool {
	for _, path := range project.Files {
		if slices.Contains(server.Languages, language.ServerLanguageOf(path)) {
			return true
		}
	}
	return false
}
