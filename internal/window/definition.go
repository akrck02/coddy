package window

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
)

// spot is a place the person was, to go back to: a file, by its path on disk as projects may be
// opened, closed and sorted again meanwhile, and a line of it, from 1.
type spot struct {
	path string
	line int
}

// trail is where following names has been: the places to go back to, and forward again.
type trail struct{ back, forward []spot }

// trailLength is how many places back the trail keeps.
const trailLength = 100

// requestWait is how long a request waits for its answer: a server still reading the projects
// (jdtls on the big workspace takes minutes) answers once it is done.
const requestWait = 3 * time.Minute

// here is the spot the person is at: the file open and the line to come back to.
func (application *Window) here() (spot, bool) {
	view := &application.viewer
	if !view.IsOpen() || view.Project == nil {
		return spot{}, false
	}
	path := filepath.Join(view.Project.Path, filepath.FromSlash(view.Reference.Path))
	return spot{path: path, line: view.LineInSight()}, true
}

// asking is what a request about the file open needs: its server, its path, and its text as it is
// now with the version it was told at.
type asking struct {
	client  *lsp.Client
	path    string
	text    string
	version int
}

// tellFirst sends the text of the file as it was when the request was made, on the request's own
// goroutine, so the request never reaches the server before it; a text sent already is not sent again.
func (asked asking) tellFirst() { _ = asked.client.Tell(asked.path, asked.text, asked.version) }

// readyServer is the running server of the file open, to ask it about the file; otherwise it says
// why there is none.
func (application *Window) readyServer() (asking, bool) {
	asked, why := application.runningServer(true)
	if why != "" {
		application.toast = why
	}
	return asked, why == ""
}

// runningServer is readyServer without a word: why there is no server, if there is none. A
// resting server starts again when waking, as for a question asked; not for a pointer resting.
func (application *Window) runningServer(waking bool) (asking, string) {
	server, root, path, found := application.serverOfOpenFile()
	if found && waking {
		application.wakeServer(serverKey(server, root))
	}
	servers := &application.servers
	switch {
	case !found:
		return asking{}, "No language server reads this file"
	case servers.missing[server.Name]:
		return asking{}, server.Name + " is not installed: " + server.Install
	case servers.failed[server.Name] != "":
		return asking{}, server.Name + " did not start: " + servers.failed[server.Name]
	}
	client, told, text := application.serveOpenFile()
	if client == nil && application.Loading() {
		return asking{}, server.Name + " starts once the projects are read"
	}
	if client == nil && servers.resting[serverKey(server, root)] != "" {
		return asking{}, server.Name + " is resting: follow a name to wake it"
	}
	if client == nil {
		return asking{}, server.Name + " is starting: try again in a moment"
	}
	return asking{client: client, path: path, text: text, version: told.version}, ""
}

// positionOf is a place of the file open as the protocol counts it.
func (application *Window) positionOf(place viewer.Place) lsp.Position {
	line := application.viewer.LineText(place.Line)
	return lsp.Position{Line: place.Line, Character: lsp.UTF16Column(line, place.Column)}
}

// GoToDefinition asks the server of the file open where the name at a place is declared, and
// opens it there, keeping where the person was to go back to. Asked on the declaration itself, it
// searches the name's uses instead, as IntelliJ's ⌘B does.
func (application *Window) GoToDefinition(place viewer.Place) {
	asked, ready := application.readyServer()
	if !ready {
		return
	}
	at := application.positionOf(place)
	name := wordAt(application.viewer.LineText(place.Line), place.Column)
	from, _ := application.here()
	home := application.viewer.Reference.Project
	application.waitFor(asked.client)
	application.work.run("definition", func(stop cancellation) func() {
		ctx, cancel := context.WithTimeout(stop, requestWait)
		defer cancel()
		asked.tellFirst()
		locations, err := asked.client.Definition(ctx, asked.path, at)
		return func() {
			application.waiting = nil
			switch {
			case err != nil:
				application.toast = "The declaration could not be found: " + err.Error()
			case len(locations) == 0:
				application.toast = "No declaration found"
			case declares(locations[0], asked.path, at):
				application.searchUses(asked, at, name, home)
			default:
				location, _ := application.fromJar(locations[0], name)
				application.goToLocation(from, location)
			}
		}
	})
}

// declares tells whether a location is the declaration at a place of a file: the name asked about
// is the one declared there.
func declares(location lsp.Location, path string, at lsp.Position) bool {
	start, end := location.Range.Start, location.Range.End
	return lsp.PathOf(location.URI) == path && start.Line == at.Line && end.Line == at.Line &&
		start.Character <= at.Character && at.Character <= end.Character
}

// waitFor says, until the answer comes, that a question waits for a server still reading.
func (application *Window) waitFor(client *lsp.Client) {
	application.waiting = nil
	if !client.Ready() {
		application.waiting = client
	}
}

// definitionAsked goes to the declaration of the name at the caret, or clicked last.
func (application *Window) definitionAsked() {
	place, found := application.viewer.PlaceAsked()
	if !found {
		application.toast = "⌘-click a name to go to its declaration"
		return
	}
	application.GoToDefinition(place)
}

// goToLocation opens a location a server gave, when it is in a project open, and keeps the spot
// the person left on the trail.
func (application *Window) goToLocation(from spot, location lsp.Location) {
	path := lsp.PathOf(location.URI)
	reference, found := application.referenceOf(path)
	if !found {
		application.toast = "Outside the projects open: " + outsideName(location.URI, path)
		return
	}
	start := location.Range.Start
	column := lsp.CharacterColumn(lineOf(path, start.Line, application), start.Character)
	application.leavingFor(reference, func() {
		trail := &application.trail
		if from.line > 0 {
			trail.back = append(trail.back, from)
			if len(trail.back) > trailLength {
				trail.back = trail.back[1:]
			}
		}
		trail.forward = nil
		application.viewer.GoToColumn = column
		application.openFileAt(reference, start.Line+1)
	})
}

// lineOf is a line of a file, from 0, as the reader has it: edits included when it is the file open.
func lineOf(path string, line int, application *Window) string {
	view := &application.viewer
	if view.IsOpen() && view.Project != nil && filepath.Join(view.Project.Path, filepath.FromSlash(view.Reference.Path)) == path {
		return view.LineText(line)
	}
	lines := workspace.LinesOfFile(path)
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
}

// referenceOf is the file of a project open at a path on disk: in the project nearest to it, as
// projects may hold others.
func (application *Window) referenceOf(path string) (workspace.FileReference, bool) {
	best, bestLength := -1, 0
	for index, project := range application.projects {
		relative, err := filepath.Rel(project.Path, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if len(project.Path) > bestLength {
			best, bestLength = index, len(project.Path)
		}
	}
	if best < 0 {
		return workspace.FileReference{}, false
	}
	relative, _ := filepath.Rel(application.projects[best].Path, path)
	return workspace.FileReference{Project: best, Path: filepath.ToSlash(relative)}, true
}

// stepTrail goes back (step -1) to where the person was before following a name, or forward
// again (step 1), once the edits not saved of the file open are kept or dropped.
func (application *Window) stepTrail(step int) {
	trail := &application.trail
	from, to := &trail.back, &trail.forward
	if step > 0 {
		from, to = to, from
	}
	if len(*from) == 0 {
		return
	}
	target := (*from)[len(*from)-1]
	reference, found := application.referenceOf(target.path)
	if !found {
		*from = (*from)[:len(*from)-1] // its project was closed
		application.toast = "No longer open: " + target.path
		return
	}
	application.leavingFor(reference, func() {
		*from = (*from)[:len(*from)-1]
		if current, open := application.here(); open {
			*to = append(*to, current)
		}
		application.openFileAt(reference, target.line)
	})
}

// leavingFor does what goes to another file once the edits not saved of the one open are kept or
// dropped, as the person answers; a person who cancels stays, and nothing is done.
func (application *Window) leavingFor(reference workspace.FileReference, going func()) {
	if application.viewer.Reference != reference {
		application.keepingEdits(going)
		return
	}
	going()
}

// outsideName names a declaration outside the projects open: its file, or the class in a jar.
func outsideName(uri, path string) string {
	if pkg, class, isJar := lsp.ClassInJar(uri); isJar {
		return pkg + "." + class + ", in a jar whose project is not open"
	}
	if path == "" {
		return uri
	}
	return path
}
