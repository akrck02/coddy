package servers

import (
	"context"
	"time"

	"github.com/akrck02/code-review/internal/gradle"
	"github.com/akrck02/code-review/internal/lsp"
)

// usage is what a running server takes: memory, its own and its processes', and its index on disk.
type usage struct {
	memory, index int64
	indexKnown    bool
	daemons       int // the Gradle daemons of its own Gradle home running
}

// measureEvery is how often Measure measures the servers again.
const measureEvery = 2 * time.Second

// Measure measures the running servers in the background, at most every two seconds, and redraws
// the window once it is due again: call it each frame while what they take is shown. An index,
// which takes a while to measure, is measured only once per server.
func (fleet *Fleet) Measure(now time.Time) {
	if fleet.measuring || now.Sub(fleet.measuredAt) < measureEvery {
		return
	}
	fleet.measuring = true
	clients, before := map[string]*lsp.Client{}, map[string]usage{}
	for key, client := range fleet.running {
		clients[key], before[key] = client, fleet.usage[key]
	}
	fleet.work.Run("measure servers", func(context.Context) func() {
		measured := map[string]usage{}
		for key, client := range clients {
			measured[key] = measure(client, before[key])
		}
		return func() {
			fleet.usage, fleet.measuring, fleet.measuredAt = measured, false, time.Now()
			time.AfterFunc(measureEvery, fleet.wake) // a frame that no longer shows it asks nothing
		}
	})
}

// measure is what a server takes now, its index as measured before when it was.
func measure(client *lsp.Client, before usage) usage {
	taken := before
	taken.memory = lsp.MemoryOf(client.Process())
	if client.Server.Import.OwnGradle && client.DataFolder() != "" {
		taken.daemons = gradle.Daemons(client.DataFolder())
	}
	if !taken.indexKnown {
		taken.index, taken.indexKnown = lsp.IndexesSize(client.DataFolder(), client.Server), true
	}
	return taken
}

// Usage writes what a server takes, by key: "memory 1.2 GB · index 4.1 GB"; "" until it is measured.
func (fleet *Fleet) Usage(key string) string {
	taken, measured := fleet.usage[key]
	if !measured {
		return ""
	}
	text := "memory " + bytesOf(taken.memory)
	if taken.index > 0 {
		text += " · index " + bytesOf(taken.index)
	}
	return text
}

// Daemons is how many Gradle daemons of a server's own Gradle home run, by key, as last measured.
func (fleet *Fleet) Daemons(key string) int { return fleet.usage[key].daemons }
