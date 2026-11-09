package window

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// restartPause is how long the projects in sight must stay the same before a server that does not
// follow workspace folders is started again for them: a few clicks of the filter restart it once.
const restartPause = 2 * time.Second

// followSight keeps a server for the whole window reading the projects in sight: a server that
// follows workspace folders is told of those that came and went; one that does not is started
// again for them, once the projects are read and the set has stayed the same a moment.
func (application *Window) followSight(client *lsp.Client, server lsp.Server, key string) {
	wanted := application.foldersFor(server)
	if client.FollowsFolders() {
		application.tellSight(client, server, wanted)
		return
	}
	reading := client.Folders()
	slices.Sort(wanted)
	set := strings.Join(wanted, "\n")
	if application.Loading() || slices.Equal(wanted, reading) || application.servers.restarting[key] == set {
		return
	}
	application.servers.restarting[key] = set
	application.work.run("restart "+key, func(stop cancellation) func() {
		if !application.work.synchronous {
			select {
			case <-time.After(restartPause):
			case <-stop.Done():
				return nil
			}
		}
		return func() { application.restartServer(server, key, client) }
	})
}

// tellSight adds to a server the projects that came into sight, and takes away those that left.
func (application *Window) tellSight(client *lsp.Client, server lsp.Server, wanted []string) {
	for _, project := range wanted {
		if !application.servers.adding[project] && !client.HasFolder(project) {
			application.addFolder(client, server.Name, project)
		}
	}
	for _, project := range client.Folders() {
		if !slices.Contains(wanted, project) && !application.servers.adding[project] {
			application.work.run("", func(cancellation) func() {
				_ = client.RemoveFolder(project)
				return nil
			})
		}
	}
}

// restartServer stops a server that does not follow workspace folders, so the next frame that
// needs it starts it for the projects in sight. Its index stays in its data folder.
func (application *Window) restartServer(server lsp.Server, key string, client *lsp.Client) {
	servers := &application.servers
	delete(servers.restarting, key)
	wanted := application.foldersFor(server)
	slices.Sort(wanted)
	if servers.running[key] != client || slices.Equal(wanted, client.Folders()) {
		return // it stopped meanwhile, or the projects came back to those it reads
	}
	delete(servers.running, key)
	for told := range servers.told {
		if strings.HasPrefix(told, key+"|") {
			delete(servers.told, told)
		}
	}
	application.toast = server.Name + " starts again to read the " + strconv.Itoa(len(wanted)) + " projects in sight"
	go client.Stop()
}
