package lsp

import (
	"time"

	"github.com/akrck02/coddy/editor/internal/gradle"
)

// A server that imports Gradle builds (jdtls, kotlin-lsp) is given a Gradle home of its own, in
// its data folder (gradle.PrepareHome), whose daemons the reader stops once the server has
// imported and with the server, never touching those of the person's builds or IDE.

// Once a server has imported the projects, its Gradle daemons only wait, half a gigabyte or more
// each, for an import that may never come: the reader stops them, and a later import starts them
// again.
const (
	// gradleCheck is how often a server is looked at for daemons to stop
	gradleCheck = 5 * time.Second
	// gradleSettle is how long a server must have been ready, with no work running, before its
	// daemons are stopped: between two steps of an import it may report none for a moment
	gradleSettle = 20 * time.Second
)

// releaseGradle stops the Gradle daemons of a server each time it is done importing, until it ends.
func (client *Client) releaseGradle() {
	ticker := time.NewTicker(gradleCheck)
	defer ticker.Stop()
	importing := true // a server imports as it starts
	for {
		select {
		case <-client.exited:
			return
		case <-ticker.C:
		}
		if client.Busy() {
			importing = true
			continue
		}
		if importing && client.settled(time.Now()) {
			importing = false
			stopped := gradle.StopDaemons(client.data)
			client.lock.Lock()
			client.released, client.releasedAt = stopped, time.Now()
			client.lock.Unlock()
			client.changed()
		}
	}
}

// settled tells whether a server is ready, with no work running, and has been for gradleSettle,
// nor begun importing a project meanwhile: kotlin-lsp is ready as it starts, and imports before
// it reports any work.
func (client *Client) settled(now time.Time) bool {
	client.lock.Lock()
	defer client.lock.Unlock()
	if !client.isReady() || len(client.working) > 0 {
		return false
	}
	quiet := later(client.readyAt, client.idleAt)
	for _, began := range client.imported {
		quiet = later(quiet, began)
	}
	return now.Sub(quiet) >= gradleSettle
}

// Released are the Gradle daemons the reader stopped once the server was done importing last, and
// when; none before.
func (client *Client) Released() (int, time.Time) {
	client.lock.Lock()
	defer client.lock.Unlock()
	return client.released, client.releasedAt
}
