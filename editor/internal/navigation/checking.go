package navigation

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/lsp"
)

// Advance asks the servers about the next places of the search, a batch per server not already
// asked, starting the servers it needs; it runs every frame and is cheap when nothing waits. A
// file's server that was reading when asked is asked again once it is done.
func (uses *Uses) Advance() {
	search := uses.search
	if search == nil {
		return
	}
	if search.busy && search.Asked.Client.Ready() {
		uses.askAgain(search)
	}
	if len(search.pending) == 0 {
		return
	}
	var waiting []candidate
	batches, clients := map[string][]candidate{}, map[string]*lsp.Client{}
	search.waitingFor = nil
	for _, place := range search.pending {
		client, key, reads := uses.host.ClientFor(place.project, place.path)
		switch {
		case !reads:
			search.unchecked++
		case client != nil && !client.Ready():
			waiting, search.waitingFor = append(waiting, place), client
		case client == nil || search.checking[key] || len(batches[key]) == usesBatch:
			waiting = append(waiting, place)
		default:
			batches[key], clients[key] = append(batches[key], place), client
		}
	}
	search.pending = waiting
	for key, batch := range batches {
		search.checking[key] = true
		uses.checkPlaces(search, key, clients[key], batch)
	}
	uses.note(search)
}

// checkPlaces asks a server where the name at each of some places is declared, in the
// background; a place whose answer is the declaration searched is a use.
func (uses *Uses) checkPlaces(search *Search, key string, client *lsp.Client, places []candidate) {
	uses.work.Run("uses "+key, func(stop context.Context) func() {
		answers := make([][]lsp.Location, len(places))
		comments := commentReader{}
		for index, place := range places {
			if stop.Err() != nil {
				return nil
			}
			if !comments.mentionOnly(place) {
				answers[index] = askDeclaration(stop, client, place)
			}
		}
		return func() {
			if uses.search != search {
				return // a newer search
			}
			delete(search.checking, key)
			uses.keepChecked(search, places, answers)
		}
	})
}

// askDeclaration asks a server where the name at a place is declared, telling it the file when it
// does not have it open, and closing it after.
func askDeclaration(stop context.Context, client *lsp.Client, place candidate) []lsp.Location {
	requestContext, cancel := context.WithTimeout(stop, RequestWait)
	defer cancel()
	defer OpenFor(client, place.path)()
	asking, done := context.WithTimeout(requestContext, checkWait)
	defer done()
	locations, _ := client.Definition(asking, place.path, place.at) // a place the server cannot answer is no use
	return locations
}

// keepChecked keeps the places whose declaration is the one searched, and lists them.
func (uses *Uses) keepChecked(search *Search, places []candidate, answers [][]lsp.Location) {
	declaration := lsp.PathOf(search.declaration.URI)
	projects := uses.host.Projects()
	for index, place := range places {
		search.checked++
		delete(search.probable, placeKey(placeLocation(place, search.Name)))
		for _, answer := range answers[index] {
			mapped, fromJar := FromJar(projects, answer, search.Name)
			sameLine := mapped.Range.Start.Line == search.declaration.Range.Start.Line
			if lsp.PathOf(mapped.URI) == declaration && (sameLine || fromJar) {
				use := placeLocation(place, search.Name)
				search.found[placeKey(use)] = use
				break
			}
		}
	}
	uses.show(search, false)
}

// askAgain asks the file's server again for the uses, now that it has read the projects.
func (uses *Uses) askAgain(search *Search) {
	search.busy = false
	asked, at := search.Asked, search.At
	uses.work.Run("references again", func(stop context.Context) func() {
		requestContext, cancel := context.WithTimeout(stop, RequestWait)
		defer cancel()
		locations, err := asked.Client.References(requestContext, asked.Path, at)
		return func() { uses.answeredAgain(search, locations, err) }
	})
}

// answeredAgain keeps the uses the server gave when asked again, or tells in the note why it gave
// none: the uses found before stay, but the person knows the list may lack some.
func (uses *Uses) answeredAgain(search *Search, locations []lsp.Location, err error) {
	if uses.search != search {
		return // a newer search
	}
	search.askedAgain = err
	for _, location := range locations {
		search.found[placeKey(location)] = location
	}
	uses.show(search, false)
}

// show shows the uses found so far, with the note on how the search goes; opening tells that the
// search has just begun.
func (uses *Uses) show(search *Search, opening bool) {
	search.noted, search.shownUses = search.note(), search.shown()
	uses.host.ShowUses(search, search.noted, opening)
}

// note says again how the search goes, when that changed.
// Only a change of the uses lays them out again: the note alone changes with each word of a
// server's progress, and laying out hundreds of uses, the sidebar and the marks for it each frame
// made the uses mode crawl while kotlin-lsp imported.
func (uses *Uses) note(search *Search) {
	if search.shown() != search.shownUses {
		uses.show(search, false)
		return
	}
	note := search.note()
	if note == search.noted {
		return
	}
	search.noted = note
	uses.host.NoteUses(note)
}
