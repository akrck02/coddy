package window

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// serverState is where a language server stands, as its dot shows it.
type serverState int

const (
	serverReady serverState = iota
	serverWorking
	serverStarting // starting, or waiting for another to import first
	serverResting
	serverBroken // not installed, or did not start
)

// serverView is one language server as the panel and the status bar show it: where it stands, a
// few words of what it does, and how far it is.
type serverView struct {
	key, name string
	client    *lsp.Client // nil while it does not run
	state     serverState
	headline  string  // "Importing · 12 of 32 projects", "Ready", "Not installed"
	fraction  float32 // how far, from 0 to 1; -1 when it cannot be told
	progress  lsp.Progress
	why       string // why it does not run, for one that does not
}

// serverViews are every server the window knows of, running first, each once, by name.
func (application *Window) serverViews() []serverView {
	servers := &application.servers
	views, seen := []serverView{}, map[string]bool{}
	add := func(view serverView) {
		if !seen[view.name] {
			seen[view.name] = true
			views = append(views, view)
		}
	}
	for key, client := range servers.running {
		add(runningView(key, client))
	}
	for key := range servers.starting {
		add(serverView{key: key, name: strings.Fields(key)[0], state: serverStarting, headline: "Starting", fraction: -1})
	}
	for name, other := range servers.queued {
		if other != "" {
			add(serverView{name: name, state: serverStarting, headline: "Waits for " + other, fraction: -1,
				why: "It starts once " + other + " has imported the projects, so the two do not run Gradle together."})
		}
	}
	application.addStoppedViews(add)
	sort.SliceStable(views, func(first, second int) bool { return views[first].name < views[second].name })
	return views
}

// addStoppedViews adds the servers that do not run: resting, not installed, or failed.
func (application *Window) addStoppedViews(add func(serverView)) {
	servers := &application.servers
	for _, name := range servers.resting {
		add(serverView{name: name, state: serverResting, headline: "Resting", fraction: -1,
			why: "Stopped while nobody asks it anything: following a name or asking its uses starts it again."})
	}
	for name := range servers.missing {
		add(serverView{name: name, state: serverBroken, headline: "Not installed", fraction: -1,
			why: "Install it with: " + application.installOf(name)})
	}
	for name, why := range servers.failed {
		add(serverView{name: name, state: serverBroken, headline: "Did not start", fraction: -1, why: why})
	}
}

// runningView is a running server: ready, or what it works on and how far it is.
func runningView(key string, client *lsp.Client) serverView {
	view := viewOf(client.Progress())
	view.key, view.name, view.client = key, client.Server.Name, client
	return view
}

// viewOf is where a server stands by what it says of itself.
func viewOf(progress lsp.Progress) serverView {
	view := serverView{progress: progress, fraction: -1}
	if progress.Phase == "ready" {
		view.state, view.headline, view.fraction = serverReady, "Ready", 1
		return view
	}
	view.state, view.headline = serverWorking, capitalised(progress.Phase)
	if progress.Phase == "starting" {
		view.state = serverStarting
	}
	done, total := projectsDone(progress)
	switch {
	case progress.Percent >= 0:
		view.fraction = float32(progress.Percent) / 100
		view.headline += " · " + strconv.Itoa(progress.Percent) + " %"
	case total > 0:
		view.fraction = float32(done) / float32(total)
	}
	if total > 1 {
		view.headline += fmt.Sprintf(" · %d of %d projects", done, total)
	}
	return view
}

// projectsDone are how many of a server's projects it has read, of how many: as it counts them,
// or as the panel counts the projects it says it is done with.
func projectsDone(progress lsp.Progress) (int, int) {
	if progress.Total > 0 {
		return progress.Done, progress.Total
	}
	done := 0
	for _, project := range progress.Projects {
		if project.State == "read" || project.State == "imported" {
			done++
		}
	}
	return done, len(progress.Projects)
}

// capitalised is a word with its first letter in capitals.
func capitalised(word string) string {
	if word == "" {
		return word
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

// elapsed is how long a working server has been at it, "" for one that is not working.
func (view serverView) elapsed(now time.Time) string {
	if view.client == nil || view.state == serverReady {
		return ""
	}
	return clock(now.Sub(view.progress.Started))
}

// dotColour is the colour of a server's state: green when ready, the accent while it works, quiet
// while it starts or rests, the warning's when it cannot run.
func dotColour(palette ui.Theme, state serverState) draw2d.Color {
	switch state {
	case serverReady:
		return palette.Positive
	case serverWorking:
		return palette.Primary
	case serverBroken:
		return palette.Warning
	}
	return palette.InkQuiet
}

// buildDot is a server's state as a small round mark.
func buildDot(context *ui.Context, state serverState, size float32) {
	colour := dotColour(context.Theme, state)
	context.Paint(ui.Layout{Width: ui.Fixed(size), Height: ui.Fixed(size)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		if state == serverResting {
			drawing.Rectangle(area, size/2, draw2d.Color{}, colour, 1.5)
			return
		}
		drawing.Rectangle(area, size/2, colour, draw2d.Color{}, 0)
	})
}

// buildBar is how far a server is, as a thin bar across its card.
func (application *Window) buildBar(context *ui.Context, fraction float32, colour draw2d.Color) {
	palette := context.Theme
	context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(4)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		drawing.Rectangle(area, 2, application.look.Hover.Mix(palette.Ink, 0.12), draw2d.Color{}, 0)
		if fraction > 0 {
			done := area
			done.Width = area.Width * min(fraction, 1)
			drawing.Rectangle(done, 2, colour, draw2d.Color{}, 0)
		}
	})
}
