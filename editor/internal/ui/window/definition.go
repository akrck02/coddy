package window

import (
	"context"
	"path/filepath"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/ui/navigation"
	"github.com/akrck02/coddy/editor/internal/ui/viewer"
)

// here is the spot the person is at: the file open and the line to come back to.
func (application *Window) here() (navigation.Spot, bool) {
	view := &application.viewer
	if !view.IsOpen() || view.Project == nil {
		return navigation.Spot{}, false
	}
	path := filepath.Join(view.Project.Path, filepath.FromSlash(view.Reference.Path))
	return navigation.Spot{Path: path, Line: view.LineInSight()}, true
}

// readyServer is the running server of the file open, to ask it about the file; otherwise it says
// why there is none.
func (application *Window) readyServer() (navigation.Asking, bool) {
	asked, why := application.runningServer(true)
	if why != "" {
		application.toast = why
	}
	return asked, why == ""
}

// runningServer is readyServer without a word: why there is no server, if there is none. A
// resting server starts again when waking, as for a question asked; not for a pointer resting.
func (application *Window) runningServer(waking bool) (navigation.Asking, string) {
	open := application.servedFile()
	client, version, why := application.servers.Asked(open, waking)
	if client == nil {
		return navigation.Asking{}, why
	}
	return navigation.Asking{Client: client, Path: open.Path, Text: open.Text(), Version: version}, ""
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
	question := navigation.Question{Name: navigation.WordAt(application.viewer.LineText(place.Line), place.Column),
		Home: application.viewer.Reference.Project, Asked: asked, At: at}
	from, _ := application.here()
	application.waitFor(asked.Client)
	application.work.Run("definition", func(stop context.Context) func() {
		requestContext, cancel := context.WithTimeout(stop, navigation.RequestWait)
		defer cancel()
		asked.TellFirst()
		locations, err := asked.Client.Definition(requestContext, asked.Path, at)
		return func() {
			application.waiting = nil
			switch {
			case err != nil:
				application.toast = "The declaration could not be found: " + err.Error()
			case len(locations) == 0:
				application.toast = "No declaration found"
			case navigation.Declares(locations[0], asked.Path, at):
				application.searchUses(question)
			default:
				location, _ := navigation.FromJar(application.workspace.Projects, locations[0], question.Name)
				application.goToLocation(from, location)
			}
		}
	})
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
func (application *Window) goToLocation(from navigation.Spot, location lsp.Location) {
	path := lsp.PathOf(location.URI)
	reference, found := navigation.ReferenceOf(application.workspace.Projects, path)
	if !found {
		application.toast = "Outside the projects open: " + navigation.OutsideName(location.URI, path)
		return
	}
	start := location.Range.Start
	column := lsp.CharacterColumn(application.LineOf(path, start.Line), start.Character)
	application.leavingFor(reference, func() {
		application.trail.Leave(from)
		application.viewer.GoToColumn = column
		application.openFileAt(reference, start.Line+1)
	})
}

// LineOf is a line of a file, from 0, as the reader has it: edits included when it is the file open.
func (application *Window) LineOf(path string, line int) string {
	view := &application.viewer
	if view.IsOpen() && view.Project != nil && filepath.Join(view.Project.Path, filepath.FromSlash(view.Reference.Path)) == path {
		return view.LineText(line)
	}
	lines := codebase.LinesOfFile(path)
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
}

// stepTrail goes back (step -1) to where the person was before following a name, or forward
// again (step 1), once the edits not saved of the file open are kept or dropped.
func (application *Window) stepTrail(step int) {
	target, found := application.trail.Next(step)
	if !found {
		return
	}
	reference, open := navigation.ReferenceOf(application.workspace.Projects, target.Path)
	if !open {
		application.trail.Drop(step) // its project was closed
		application.toast = "No longer open: " + target.Path
		return
	}
	application.leavingFor(reference, func() {
		current, isOpen := application.here()
		application.trail.Take(step, current, isOpen)
		application.openFileAt(reference, target.Line)
	})
}

// leavingFor does what goes to another file once the edits not saved of the one open are kept or
// dropped, as the person answers; a person who cancels stays, and nothing is done.
func (application *Window) leavingFor(reference codebase.FileReference, going func()) {
	if application.viewer.Reference != reference {
		application.keepingEdits(going)
		return
	}
	going()
}
