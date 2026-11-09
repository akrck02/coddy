package window

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// watchProgress redraws the window when what a server says of itself changes, and each second
// while it is not ready, for the clock; it ends with the server.
func watchProgress(client *lsp.Client) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-client.Changes():
			wake()
		case <-ticker.C:
			if !client.Ready() {
				wake()
			}
		case <-client.Done():
			return
		}
	}
}

// serverStatus is what the status bar says of the servers: for each, what it is doing, on what,
// how far and for how long, until it is ready; those not installed, and those that failed.
func (application *Window) serverStatus() string {
	servers := &application.servers
	var parts []string
	for key := range servers.starting {
		parts = append(parts, strings.Fields(key)[0]+" · starting")
	}
	for key, client := range servers.running {
		parts = append(parts, statusOf(strings.Fields(key)[0], client.Progress(), time.Now()))
	}
	for name := range servers.missing {
		parts = append(parts, name+" not installed")
	}
	for name := range servers.failed {
		parts = append(parts, name+" failed")
	}
	sort.Strings(parts)
	return strings.Join(unique(parts), "   ")
}

// statusOf says what one server is doing: "kotlin-lsp · importing harbour.api.order (7 of 22) ·
// 3:12", "jdtls · indexing 45 % · 0:40", or "gopls ready".
func statusOf(name string, progress lsp.Progress, now time.Time) string {
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
	return name + " · " + doing + " · " + clock(now.Sub(progress.Started))
}

// clock writes a time as minutes and seconds: 3:07.
func clock(length time.Duration) string {
	seconds := int(length.Seconds())
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
