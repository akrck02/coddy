package servers

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/english"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/workspace"
)

// State is where a language server stands.
type State int

const (
	// Ready servers answer at once.
	Ready State = iota
	// Working servers read the projects: they answer, maybe not with everything.
	Working
	// Starting servers start, or wait for another to import first.
	Starting
	// Resting servers were stopped for going unasked; a question starts them again.
	Resting
	// Broken servers are not installed, or did not start.
	Broken
)

// View is one language server as a person is shown it: where it stands, a few words of what it
// does, and how far it is.
type View struct {
	Key, Name string
	Client    *lsp.Client // nil while it does not run
	State     State
	Headline  string  // "Importing · 12 of 32 projects", "Ready", "Not installed"
	Fraction  float32 // how far, from 0 to 1; -1 when it cannot be told
	Progress  lsp.Progress
	Why       string // why it does not run, for one that does not
	// Installable tells that it can be installed from here: it is missing and its installer is here
	Installable bool
	LeftOut     []LeftOutProject
}

// LeftOutProject is a project a server does not read, and why.
type LeftOutProject struct{ Name, Why string }

// shownViews are the views made last, and what they were made from: kept while none of it
// changes, so the status bar reads the servers every frame and makes nothing.
type shownViews struct {
	made     bool
	revision int    // the fleet's revision
	versions uint64 // the running servers' versions, added up: each only grows
	second   int64  // the second the views were made in, while a server is at work; else 0
	projects []*workspace.Project
	views    []View
}

// Views are every server the fleet knows of, each once, sorted by name. They are made again
// only when a server or the projects change, and each second while a server is at work, as how
// long it has worked on each project shows; the views given are kept, so they must not be changed.
func (fleet *Fleet) Views() []View {
	shown := &fleet.shown
	versions, second, projects := fleet.runningVersions(), shown.workingSecond(), fleet.host.Projects()
	if shown.made && shown.revision == fleet.revision && shown.versions == versions && shown.second == second &&
		slices.Equal(shown.projects, projects) {
		return shown.views
	}
	shown.views = fleet.makeViews()
	shown.made, shown.revision, shown.versions, shown.second = true, fleet.revision, versions, shown.workingSecond()
	shown.projects = append(shown.projects[:0], projects...)
	return shown.views
}

// touched tells the views that a map they are made from changed.
func (fleet *Fleet) touched() { fleet.revision++ }

// runningVersions are the versions of the running servers, added up: as each only grows, the sum
// changes whenever one of them does.
func (fleet *Fleet) runningVersions() uint64 {
	var sum uint64
	for _, client := range fleet.running {
		sum += client.Version()
	}
	return sum
}

// workingSecond is the second it is now while a server shown is at work, whose times change
// each second; 0 while none is.
func (shown *shownViews) workingSecond() int64 {
	for _, view := range shown.views {
		if view.Client != nil && (view.State == Working || view.State == Starting) {
			return time.Now().Unix()
		}
	}
	return 0
}

// makeViews makes the views of every server the fleet knows of, each once, sorted by name.
func (fleet *Fleet) makeViews() []View {
	var views []View // none most of the time: then nothing is made, each frame
	add := func(view View) {
		for _, known := range views {
			if known.Name == view.Name {
				return // a few servers at most: looking through them is cheaper than a map
			}
		}
		views = append(views, view)
	}
	for key, client := range fleet.running {
		view := RunningView(client)
		view.Key, view.LeftOut = key, fleet.leftOutOf(client.Server)
		add(view)
	}
	for key := range fleet.starting {
		add(View{Key: key, Name: strings.Fields(key)[0], State: Starting, Headline: "Starting", Fraction: -1})
	}
	for name, other := range fleet.queued {
		if other != "" {
			add(View{Name: name, State: Starting, Headline: "Waits for " + other, Fraction: -1,
				Why: "It starts once " + other + " has imported the projects, so the two do not run Gradle together."})
		}
	}
	fleet.addStoppedViews(add)
	sort.SliceStable(views, func(first, second int) bool { return views[first].Name < views[second].Name })
	return views
}

// addStoppedViews adds the servers that do not run: resting, not installed, or failed.
func (fleet *Fleet) addStoppedViews(add func(View)) {
	for _, name := range fleet.resting {
		add(View{Name: name, State: Resting, Headline: "Resting", Fraction: -1,
			Why: "Stopped while nobody asks it anything: following a name or asking its uses starts it again."})
	}
	for name := range fleet.missing {
		add(fleet.missingView(name))
	}
	for name, why := range fleet.failed {
		add(View{Name: name, State: Broken, Headline: "Did not start", Fraction: -1, Why: why})
	}
}

// missingView is a server not installed: being installed, or how to install it, and why the last
// install did not work.
func (fleet *Fleet) missingView(name string) View {
	if fleet.installing[name] {
		return View{Name: name, State: Starting, Headline: "Installing", Fraction: -1,
			Why: "Running " + fleet.InstallCommand(name) + "…"}
	}
	view := View{Name: name, State: Broken, Headline: "Not installed", Fraction: -1,
		Why: "Install it with: " + fleet.InstallCommand(name), Installable: fleet.installable[name]}
	if why := fleet.installFailed[name]; why != "" {
		view.Headline, view.Why = "Install failed", why
	}
	return view
}

// leftOutOf are the projects of the window a server leaves out, by name: those of its languages
// whose build it cannot import (gradle.CannotImport).
func (fleet *Fleet) leftOutOf(server lsp.Server) []LeftOutProject {
	if !server.Import.OwnGradle {
		return nil
	}
	var left []LeftOutProject
	for _, project := range fleet.host.Projects() {
		if why := fleet.cannotImport[project.Path]; why != "" && hasFileOf(project, server) {
			left = append(left, LeftOutProject{Name: filepath.Base(project.Path), Why: why})
		}
	}
	sort.Slice(left, func(first, second int) bool { return left[first].Name < left[second].Name })
	return left
}

// RunningView is a running server: ready, or what it works on and how far it is. Its key is "".
func RunningView(client *lsp.Client) View {
	view := ViewOf(client.Progress())
	view.Name, view.Client = client.Server.Name, client
	return view
}

// ViewOf is where a server stands by what it says of itself.
func ViewOf(progress lsp.Progress) View {
	view := View{Progress: progress, Fraction: -1}
	if progress.Phase == "ready" {
		view.State, view.Headline, view.Fraction = Ready, "Ready", 1
		return view
	}
	view.State, view.Headline = Working, english.Capitalised(progress.Phase)
	if progress.Phase == "starting" {
		view.State = Starting
	}
	done, total := ProjectsDone(progress)
	switch {
	case progress.Percent >= 0:
		view.Fraction = float32(progress.Percent) / 100
		view.Headline += " · " + strconv.Itoa(progress.Percent) + " %"
	case total > 1: // one project, still being read, is not 100 %
		view.Fraction = float32(done) / float32(total)
	}
	if total > 1 {
		view.Headline += fmt.Sprintf(" · %d of %d projects", done, total)
	}
	return view
}

// ProjectsDone are how many of a server's projects it has read, of how many: as it counts them,
// or as the projects it says it is done with count.
func ProjectsDone(progress lsp.Progress) (int, int) {
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

// Elapsed is how long a working server has been at it, "" for one that is not working.
func (view View) Elapsed(now time.Time) string {
	if view.Client == nil || view.State == Ready {
		return ""
	}
	return Clock(now.Sub(view.Progress.Started))
}
