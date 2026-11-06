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

// spot is a place the person was, to go back to: a file and a line of it, from 1.
type spot struct {
	reference workspace.FileReference
	line      int
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
	if !view.IsOpen() {
		return spot{}, false
	}
	return spot{reference: view.Reference, line: view.LineInSight()}, true
}

// readyServer is the running server of the file open and the file's path, after telling it the
// text as it is now; otherwise it says why there is none.
func (application *Window) readyServer() (*lsp.Client, string, bool) {
	client, path, why := application.runningServer()
	if why != "" {
		application.toast = why
	}
	return client, path, why == ""
}

// runningServer is readyServer without a word: why there is no server, if there is none.
func (application *Window) runningServer() (client *lsp.Client, path, why string) {
	server, root, path, found := application.serverOfOpenFile()
	servers := &application.servers
	switch {
	case !found:
		return nil, "", "No language server reads this file"
	case servers.missing[server.Name]:
		return nil, "", server.Name + " is not installed: " + server.Install
	}
	application.serveOpenFile()
	if client = servers.running[serverKey(server, root)]; client == nil {
		return nil, "", server.Name + " is starting: try again in a moment"
	}
	return client, path, ""
}

// positionOf is a place of the file open as the protocol counts it.
func (application *Window) positionOf(place viewer.Place) lsp.Position {
	line := application.viewer.LineText(place.Line)
	return lsp.Position{Line: place.Line, Character: lsp.UTF16Column(line, place.Column)}
}

// GoToDefinition asks the server of the file open where the name at a place is declared, and
// opens it there, keeping where the person was to go back to.
func (application *Window) GoToDefinition(place viewer.Place) {
	client, path, ready := application.readyServer()
	if !ready {
		return
	}
	at := application.positionOf(place)
	from, _ := application.here()
	application.work.run("definition", func(stop cancellation) func() {
		ctx, cancel := context.WithTimeout(stop, requestWait)
		defer cancel()
		locations, err := client.Definition(ctx, path, at)
		return func() {
			switch {
			case err != nil:
				application.toast = "The declaration could not be found: " + err.Error()
			case len(locations) == 0:
				application.toast = "No declaration found"
			default:
				application.goToLocation(from, locations[0])
			}
		}
	})
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
		application.toast = "Outside the projects open: " + path
		return
	}
	trail := &application.trail
	if from.line > 0 {
		trail.back = append(trail.back, from)
		if len(trail.back) > trailLength {
			trail.back = trail.back[1:]
		}
	}
	trail.forward = nil
	start := location.Range.Start
	application.viewer.GoToColumn = lsp.CharacterColumn(lineOf(path, start.Line, application), start.Character)
	application.openFileAt(reference, start.Line+1)
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
// again (step 1).
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
	*from = (*from)[:len(*from)-1]
	if current, open := application.here(); open {
		*to = append(*to, current)
	}
	application.openFileAt(target.reference, target.line)
}
