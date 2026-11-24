package lsp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// A server that keeps a data folder notes there how long it took to read the projects, from its
// start to ready with no work running: the next time, the reader says about how long is left.

// readingFile is the file of the data folder that holds it, in seconds.
const readingFile = "reading-took"

// readingCheck is how often a starting server is looked at; it is done once ready on two looks in
// a row, as between two pieces of work it may report none for a moment.
const readingCheck = 2 * time.Second

// LastReading is how long a server took to read the projects the time before; 0 when unknown.
func LastReading(dataFolder string) time.Duration {
	content, err := os.ReadFile(filepath.Join(dataFolder, readingFile))
	if err != nil {
		return 0
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// LastReading is how long the server took to read the projects the time before; 0 when unknown.
func (client *Client) LastReading() time.Duration { return client.lastReading }

// rememberReading notes how long the server takes to read the projects, once it has.
func (client *Client) rememberReading() {
	ticker := time.NewTicker(readingCheck)
	defer ticker.Stop()
	var readyAt time.Time
	for {
		select {
		case <-client.exited:
			return
		case now := <-ticker.C:
			if !client.Ready() {
				readyAt = time.Time{}
				continue
			}
			if readyAt.IsZero() {
				readyAt = now
				continue
			}
			took := int(readyAt.Sub(client.started).Seconds())
			_ = os.WriteFile(filepath.Join(client.data, readingFile), []byte(strconv.Itoa(max(took, 1))), 0o644) // only a guess for next time
			return
		}
	}
}
