package servers

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// watchProgress redraws the window when what a server says of itself changes, and each second
// while it is not ready, for the clock; it ends with the server.
func (fleet *Fleet) watchProgress(client *lsp.Client) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-client.Changes():
			fleet.wake()
		case <-ticker.C:
			if !client.Ready() {
				fleet.wake()
			}
		case <-client.Done():
			return
		}
	}
}

// Status is what the servers do, in a line: for each, sorted, what it is doing, on what, how far
// and for how long, until it is ready; those resting, not installed, and those that failed.
func (fleet *Fleet) Status() string {
	var texts []string
	for _, part := range fleet.statusParts() {
		texts = append(texts, part.text)
	}
	return strings.Join(texts, "   ")
}

// statusPart is what the status says of one server.
type statusPart struct{ name, text string }

// statusParts are what the status says of each server, sorted, each once.
func (fleet *Fleet) statusParts() []statusPart {
	var parts []statusPart
	for key := range fleet.starting {
		name := strings.Fields(key)[0]
		parts = append(parts, statusPart{name, name + " · starting"})
	}
	for key, client := range fleet.running {
		name := strings.Fields(key)[0]
		parts = append(parts, statusPart{name, StatusOf(name, client.Progress(), time.Now())})
	}
	for name, other := range fleet.queued {
		if server, found := fleet.Named(name); found && fleet.importingBefore(server) == "" {
			delete(fleet.queued, name) // the other is done: the next frame that needs it starts it
			fleet.touched()
			continue
		}
		parts = append(parts, statusPart{name, name + " waits for " + other + " to import"})
	}
	for _, name := range fleet.resting {
		parts = append(parts, statusPart{name, name + " resting"})
	}
	for name := range fleet.missing {
		parts = append(parts, statusPart{name, name + " not installed"})
	}
	for name := range fleet.failed {
		parts = append(parts, statusPart{name, name + " failed"})
	}
	sort.Slice(parts, func(first, second int) bool { return parts[first].text < parts[second].text })
	return slices.CompactFunc(parts, func(first, second statusPart) bool { return first.text == second.text })
}

// StatusOf says what one server is doing: "kotlin-lsp · importing harbour.api (7 of 22) ·
// 3:12", "jdtls · indexing 45 % · 0:40", or "gopls ready".
func StatusOf(name string, progress lsp.Progress, now time.Time) string {
	if progress.Phase == "ready" {
		return name + " ready"
	}
	doing := progress.Phase
	if detail := progress.Detail; detail != "" && !strings.HasSuffix(strings.TrimSpace(detail), "%") {
		doing += " " + detail
	}
	if progress.Phase == "importing" && progress.Total > 1 {
		doing += fmt.Sprintf(" (%d of %d)", progress.Done, progress.Total)
	}
	if progress.Percent >= 0 {
		doing += " " + strconv.Itoa(progress.Percent) + " %"
	}
	return name + " · " + doing + " · " + Clock(now.Sub(progress.Started))
}

// Clock writes a time as minutes and seconds: 3:07.
func Clock(length time.Duration) string {
	seconds := int(length.Seconds())
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
