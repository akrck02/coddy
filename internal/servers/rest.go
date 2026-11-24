package servers

import (
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// A server not asked anything for a while rests: it is stopped, giving back its memory, and starts
// again on the next question, from its index, in seconds. Opening a file or pointing at a name does
// not wake it; following a name or asking its uses does.

// DefaultRest is how long a server may go unasked before it rests, unless Fleet.Rest says another.
const DefaultRest = 15 * time.Minute

// shortestRestCheck is the least time between two checks for servers that may rest.
const shortestRestCheck = 10 * time.Millisecond

// watchRest checks, now and then while servers run, for those that may rest.
func (fleet *Fleet) watchRest() {
	if fleet.watchingRest || len(fleet.running) == 0 {
		return
	}
	fleet.watchingRest = true
	tick := make(chan struct{})
	time.AfterFunc(max(fleet.Rest/15, shortestRestCheck), func() { close(tick) })
	fleet.work.WhenDone(tick, func() {
		fleet.watchingRest = false
		fleet.restIdle(time.Now())
		fleet.watchRest()
	})
}

// restIdle stops the servers that were not asked anything for a while, once they have read the
// projects and report no work: one stopped while it imports throws its import away, and kotlin-lsp
// its index too (a cold index of six minutes after each import cut short, in the big workspace).
func (fleet *Fleet) restIdle(now time.Time) {
	for key, client := range fleet.running {
		if !client.Ready() || now.Sub(client.QuietSince()) < fleet.Rest {
			continue
		}
		fleet.Pause(key, client)
	}
}

// Pause stops a server until it is asked something again: gone unasked, or stopped from the
// servers panel. It rests, and the next question wakes it.
func (fleet *Fleet) Pause(key string, client *lsp.Client) {
	delete(fleet.running, key) // so its end is not taken for a crash
	delete(fleet.told, key)
	fleet.resting[key] = client.Server.Name
	fleet.stop(key, client)
}

// Wake lets a resting server start again, as it is asked something.
func (fleet *Fleet) Wake(key string) {
	delete(fleet.resting, key)
}
