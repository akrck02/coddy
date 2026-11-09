package window

import (
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// A server not asked anything for a while rests: it is stopped, giving back its memory, and starts
// again on the next question, from its index, in seconds. Opening a file or pointing at a name does
// not wake it; following a name or asking its uses does.

// serverRest is how long a server may go unasked before it rests.
const serverRest = 15 * time.Minute

// watchRest checks, now and then while servers run, for those that may rest.
func (application *Window) watchRest() {
	servers := &application.servers
	if servers.watchingRest || len(servers.running) == 0 {
		return
	}
	servers.watchingRest = true
	tick := make(chan struct{})
	time.AfterFunc(max(servers.rest/15, 10*time.Millisecond), func() { close(tick) })
	application.work.whenDone(tick, func() {
		servers.watchingRest = false
		application.restServers(time.Now())
		application.watchRest()
	})
}

// restServers stops the servers that reported no work and were not asked anything for a while.
func (application *Window) restServers(now time.Time) {
	servers := &application.servers
	for key, client := range servers.running {
		if client.Busy() || now.Sub(client.QuietSince()) < servers.rest {
			continue
		}
		application.restServer(key, client)
	}
}

// restServer stops a server until it is asked something again: gone unasked, or stopped from the
// servers panel.
func (application *Window) restServer(key string, client *lsp.Client) {
	servers := &application.servers
	delete(servers.running, key) // so its end is not taken for a crash
	for told := range servers.told {
		if strings.HasPrefix(told, key+"|") {
			delete(servers.told, told)
		}
	}
	servers.resting[key] = client.Server.Name
	go client.Stop()
}

// wakeServer lets a resting server start again, as it is asked something.
func (application *Window) wakeServer(key string) {
	delete(application.servers.resting, key)
}
