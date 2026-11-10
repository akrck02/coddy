package window

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	ui "github.com/akrck02/go-ui"
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

// statusPart is what the status bar says of one server.
type statusPart struct{ name, text string }

// serverParts are what the status bar says of the servers, sorted: for each, what it is doing, on
// what, how far and for how long, until it is ready; those not installed, and those that failed.
func (application *Window) serverParts() []statusPart {
	servers := &application.servers
	var parts []statusPart
	for key := range servers.starting {
		name := strings.Fields(key)[0]
		parts = append(parts, statusPart{name, name + " · starting"})
	}
	for key, client := range servers.running {
		name := strings.Fields(key)[0]
		parts = append(parts, statusPart{name, statusOf(name, client.Progress(), time.Now())})
	}
	for name, other := range servers.queued {
		if server, found := serverNamed(servers.table, name); found && application.importingBefore(server) == "" {
			delete(servers.queued, name) // the other is done: the next frame that needs it starts it
			continue
		}
		parts = append(parts, statusPart{name, name + " waits for " + other + " to import"})
	}
	for _, name := range servers.resting {
		parts = append(parts, statusPart{name, name + " resting"})
	}
	for name := range servers.missing {
		parts = append(parts, statusPart{name, name + " not installed"})
	}
	for name := range servers.failed {
		parts = append(parts, statusPart{name, name + " failed"})
	}
	sort.Slice(parts, func(first, second int) bool { return parts[first].text < parts[second].text })
	return slices.CompactFunc(parts, func(first, second statusPart) bool { return first.text == second.text })
}

// serverStatus is the status bar's words about the servers, together.
func (application *Window) serverStatus() string {
	var texts []string
	for _, part := range application.serverParts() {
		texts = append(texts, part.text)
	}
	return strings.Join(texts, "   ")
}

// buildServerStatus shows each server in the status bar as its state's dot and its name, with how
// far it is while it works; its tooltip says the rest, and a click opens the servers panel.
func (application *Window) buildServerStatus(context *ui.Context, small ui.TextStyle) {
	hovered := ui.Look{Fill: application.look.Hover, Radius: 4}
	looks := ui.Looks{Hovered: hovered, Pressed: hovered}
	for _, view := range application.serverViews() {
		if context.Clickable("server status "+view.name, ui.Layout{Padding: ui.PadXY(6, 2), Gap: 5, AlignY: ui.Center}, looks, func() {
			buildDot(context, view.state, 7)
			context.Text(view.name+chipProgress(view), small)
		}) {
			application.serversOpen = true
		}
		context.Tooltip(view.name + " · " + view.headline)
	}
}

// chipProgress is how far a working server is, after its name in the status bar: " 71 %", " 12/32".
func chipProgress(view serverView) string {
	if view.state != serverWorking {
		return ""
	}
	if view.progress.Percent >= 0 {
		return " " + strconv.Itoa(view.progress.Percent) + " %"
	}
	if done, total := projectsDone(view.progress); total > 1 {
		return fmt.Sprintf(" %d/%d", done, total)
	}
	return ""
}

// statusOf says what one server is doing: "kotlin-lsp · importing harbour.api (7 of 22) ·
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

// serverNamed is the server of a name in a table.
func serverNamed(table []lsp.Server, name string) (lsp.Server, bool) {
	for _, server := range table {
		if server.Name == name {
			return server, true
		}
	}
	return lsp.Server{}, false
}
