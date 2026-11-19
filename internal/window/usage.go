package window

import (
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// serverUsage is what a running server takes: memory, its own and its processes', and its index
// on disk, measured while the servers panel is open.
type serverUsage struct {
	memory, index int64
	indexKnown    bool
	daemons       int // the Gradle daemons of its own Gradle home running
}

// measureEvery is how often the servers panel measures the servers again.
const measureEvery = 2 * time.Second

// measureServers measures the running servers in the background, now and then while the servers
// panel is open; an index, which takes a while to measure, only once per server.
func (application *Window) measureServers(now time.Time) {
	servers := &application.servers
	if !application.serversOpen || servers.measuring || now.Sub(servers.measuredAt) < measureEvery {
		return
	}
	servers.measuring = true
	clients, before := map[string]*lsp.Client{}, map[string]serverUsage{}
	for key, client := range servers.running {
		clients[key], before[key] = client, servers.usage[key]
	}
	application.work.run("measure servers", func(cancellation) func() {
		measured := map[string]serverUsage{}
		for key, client := range clients {
			usage := before[key]
			usage.memory = lsp.MemoryOf(client.Process())
			if client.Server.OwnGradle && client.DataFolder() != "" {
				usage.daemons = lsp.GradleDaemons(client.DataFolder())
			}
			if !usage.indexKnown {
				usage.index, usage.indexKnown = lsp.IndexesSize(client.DataFolder(), client.Server), true
			}
			measured[key] = usage
		}
		return func() {
			servers.usage, servers.measuring, servers.measuredAt = measured, false, time.Now()
			if application.serversOpen {
				time.AfterFunc(measureEvery, wake)
			}
		}
	})
}

// usageOf writes what a server takes: "memory 1.2 GB · index 4.1 GB"; "" until it is measured.
func (application *Window) usageOf(key string) string {
	usage, measured := application.servers.usage[key]
	if !measured {
		return ""
	}
	text := "memory " + bytesOf(usage.memory)
	if usage.index > 0 {
		text += " · index " + bytesOf(usage.index)
	}
	return text
}
