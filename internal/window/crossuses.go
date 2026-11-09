package window

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/lsp"
)

// usesSearch is a list of uses being made: those the file's server knows, then places in other
// projects that hold the name, each checked by asking its own server where the name is declared.
type usesSearch struct {
	name  string
	home  int
	asked asking
	at    lsp.Position
	busy  bool // the file's server was still reading when asked: it is asked again once done
	// declaration is where the name is declared, in a project open; known tells whether it is
	declaration lsp.Location
	known       bool
	found       map[string]lsp.Location // the uses, by path and line
	pending     []candidate
	checking    map[string]bool // the servers asked about some places now, by key
	checked     int
	total       int
	projects    map[string]bool // the projects the places are in
	left        int             // places past the limit, not checked
	unchecked   int             // places no server reads
}

// candidate is a place of another file that holds the name, to check.
type candidate struct {
	path, project string
	at            lsp.Position
}

const (
	// usesCheckLimit is how many places of other files are checked at most, the nearest first
	usesCheckLimit = 300
	// usesBatch is how many places one job asks a server about, so the list grows as it goes
	usesBatch = 20
	// checkWait is how long one place's question may take once its server is ready
	checkWait = 30 * time.Second
)

// inSightPaths are the folders of the projects in sight that are read.
func (application *Window) inSightPaths() []string {
	var paths []string
	for index, project := range application.projects {
		if application.Searchable(index) {
			paths = append(paths, project.Path)
		}
	}
	return paths
}

// wordSearch searches for a name as a whole word in the files a server of the table reads.
func (application *Window) wordSearch() git.GrepOptions {
	options := git.GrepOptions{WorkingTree: application.settings.WorkingTree, Word: true}
	for extension, language := range lsp.LanguageIDs() {
		if _, found := lsp.ServerFor(application.servers.table, language); found {
			options.Files = append(options.Files, "*"+extension)
		}
	}
	sort.Strings(options.Files)
	return options
}

// grepAll searches every project for a name at once, as many as are read together, by folder.
func (application *Window) grepAll(stop cancellation, projects []string, name string, options git.GrepOptions) map[string][]git.Hit {
	found := make([][]git.Hit, len(projects))
	var group sync.WaitGroup
	for position, project := range projects {
		group.Add(1)
		go func() {
			defer group.Done()
			application.work.read(stop, func() { found[position], _ = git.Grep(project, name, options) })
		}()
	}
	group.Wait()
	byProject := map[string][]git.Hit{}
	for position, project := range projects {
		byProject[project] = found[position]
	}
	return byProject
}

// startUses lists the uses the file's server gave, and starts checking the other places that hold
// the name, when its declaration is known.
func (application *Window) startUses(search *usesSearch, locations, declared []lsp.Location, hits map[string][]git.Hit) {
	search.found, search.checking, search.projects = map[string]lsp.Location{}, map[string]bool{}, map[string]bool{}
	for _, location := range locations {
		search.found[placeKey(location)] = location
	}
	if len(declared) > 0 {
		search.declaration, _ = application.fromJar(declared[0], search.name)
		search.known = application.inProjects(search.declaration)
	}
	if search.known {
		application.addCandidates(search, hits)
	}
	application.uses = search
	application.finder.ShowUsesFrom(application, "uses of "+search.name, search.home, application.usesListed(search))
	application.finder.Note = application.usesNote(search)
	application.advanceUses()
}

// inProjects tells whether a location is a file of a project open.
func (application *Window) inProjects(location lsp.Location) bool {
	_, found := application.referenceOf(lsp.PathOf(location.URI))
	return found
}

// placeKey names a place by its file and line.
func placeKey(location lsp.Location) string {
	return lsp.PathOf(location.URI) + ":" + strconv.Itoa(location.Range.Start.Line)
}

// addCandidates keeps the places that hold the name and the server did not give, the nearest
// first: the name's own project, then the projects with changes, then the others.
func (application *Window) addCandidates(search *usesSearch, hits map[string][]git.Hit) {
	var places []candidate
	for project, found := range hits {
		for _, hit := range found {
			path := filepath.Join(project, filepath.FromSlash(hit.Path))
			at := lsp.Position{Line: hit.Line - 1, Character: lsp.UTF16Column(hit.Text, len([]rune(hit.Text[:min(hit.Column, len(hit.Text))])))}
			if _, known := search.found[path+":"+strconv.Itoa(at.Line)]; !known {
				places = append(places, candidate{path: path, project: project, at: at})
			}
		}
	}
	rank := application.projectRanks(search.home)
	sort.SliceStable(places, func(first, second int) bool {
		one, other := places[first], places[second]
		if rank[one.project] != rank[other.project] {
			return rank[one.project] < rank[other.project]
		}
		return one.path < other.path || one.path == other.path && one.at.Line < other.at.Line
	})
	if len(places) > usesCheckLimit {
		search.left, places = len(places)-usesCheckLimit, places[:usesCheckLimit]
	}
	search.pending, search.total = places, len(places)
	for _, place := range places {
		search.projects[place.project] = true
	}
}

// projectRanks orders the projects for checking: the home one, those with changes, the others.
func (application *Window) projectRanks(home int) map[string]int {
	ranks := map[string]int{}
	for index, project := range application.projects {
		switch {
		case index == home:
			ranks[project.Path] = 0
		case len(project.Changes) > 0:
			ranks[project.Path] = 1
		default:
			ranks[project.Path] = 2
		}
	}
	return ranks
}

// usesListed are the uses found so far, as the finder lists them.
func (application *Window) usesListed(search *usesSearch) []finder.Use {
	var uses []finder.Use
	for _, location := range search.found {
		if use, found := application.useOf(location); found {
			uses = append(uses, use)
		}
	}
	return uses
}

// advanceUses asks the servers about the next places of the search, a batch per server not
// already asked, starting the servers it needs; it runs every frame and is cheap when nothing
// waits. A file's server that was reading when asked is asked again once it is done.
func (application *Window) advanceUses() {
	search := application.uses
	if search == nil {
		return
	}
	if search.busy && search.asked.client.Ready() {
		application.askUsesAgain(search)
	}
	if len(search.pending) == 0 {
		return
	}
	var waiting []candidate
	batches, clients := map[string][]candidate{}, map[string]*lsp.Client{}
	for _, place := range search.pending {
		client, key, state := application.clientOf(place)
		switch {
		case state == "none":
			search.unchecked++
		case client == nil || search.checking[key] || len(batches[key]) == usesBatch:
			waiting = append(waiting, place)
		default:
			batches[key], clients[key] = append(batches[key], place), client
		}
	}
	search.pending = waiting
	for key, batch := range batches {
		search.checking[key] = true
		application.checkPlaces(search, key, clients[key], batch)
	}
	application.finder.Note = application.usesNote(search)
}

// clientOf is the running server of a place's file, starting it when it is not; state is "none"
// when no server reads the file, or its server is not installed or failed.
func (application *Window) clientOf(place candidate) (*lsp.Client, string, string) {
	servers := &application.servers
	server, found := lsp.ServerFor(servers.table, lsp.LanguageOf(place.path))
	if !found || servers.missing[server.Name] || servers.failed[server.Name] != "" {
		return nil, "", "none"
	}
	root, known := servers.roots[place.path]
	if !known {
		root = server.RootOf(place.project, place.path)
		servers.roots[place.path] = root
	}
	key := serverKey(server, root)
	application.wakeServer(key)
	client := servers.running[key]
	if client == nil && !servers.starting[key] && !application.Loading() {
		application.startServer(server, root, key, place.project)
	}
	return client, key, "running"
}

// checkPlaces asks a server where the name at each of some places is declared, in the
// background; a place whose answer is the declaration searched is a use.
func (application *Window) checkPlaces(search *usesSearch, key string, client *lsp.Client, places []candidate) {
	application.work.run("uses "+key, func(stop cancellation) func() {
		answers := make([][]lsp.Location, len(places))
		for index, place := range places {
			if stop.Err() != nil {
				return nil
			}
			answers[index] = askDeclaration(stop, client, place)
		}
		return func() {
			if application.uses != search {
				return // a newer search
			}
			delete(search.checking, key)
			application.keepChecked(search, places, answers)
		}
	})
}

// askDeclaration asks a server where the name at a place is declared, telling it the file when it
// does not have it open, and closing it after.
func askDeclaration(stop cancellation, client *lsp.Client, place candidate) []lsp.Location {
	ctx, cancel := context.WithTimeout(stop, requestWait)
	defer cancel()
	if !client.IsOpen(place.path) {
		text, err := os.ReadFile(place.path)
		if err != nil {
			return nil
		}
		_ = client.Tell(place.path, string(text), 1)
		defer func() { _ = client.Close(place.path) }()
	}
	asking, done := context.WithTimeout(ctx, checkWait)
	defer done()
	locations, _ := client.Definition(asking, place.path, place.at)
	return locations
}

// keepChecked keeps the places whose declaration is the one searched, and lists them.
func (application *Window) keepChecked(search *usesSearch, places []candidate, answers [][]lsp.Location) {
	declaration := lsp.PathOf(search.declaration.URI)
	for index, place := range places {
		search.checked++
		for _, answer := range answers[index] {
			mapped, fromJar := application.fromJar(answer, search.name)
			sameLine := mapped.Range.Start.Line == search.declaration.Range.Start.Line
			if lsp.PathOf(mapped.URI) == declaration && (sameLine || fromJar) {
				end := place.at
				end.Character += len([]rune(search.name))
				use := lsp.Location{URI: lsp.FileURI(place.path), Range: lsp.Range{Start: place.at, End: end}}
				search.found[placeKey(use)] = use
				break
			}
		}
	}
	application.finder.UpdateUses(application, application.usesListed(search))
	application.finder.Note = application.usesNote(search)
}

// askUsesAgain asks the file's server again for the uses, now that it has read the projects.
func (application *Window) askUsesAgain(search *usesSearch) {
	search.busy = false
	asked, at := search.asked, search.at
	application.work.run("references again", func(stop cancellation) func() {
		ctx, cancel := context.WithTimeout(stop, requestWait)
		defer cancel()
		locations, err := asked.client.References(ctx, asked.path, at)
		return func() {
			if err != nil || application.uses != search {
				return
			}
			for _, location := range locations {
				search.found[placeKey(location)] = location
			}
			application.finder.UpdateUses(application, application.usesListed(search))
			application.finder.Note = application.usesNote(search)
		}
	})
}

// usesNote says how the list of uses is still being made, or what it left out.
func (application *Window) usesNote(search *usesSearch) string {
	var words []string
	if !search.known {
		return "only " + search.asked.client.Server.Name + "'s: its declaration is not in a project open"
	}
	if search.busy {
		words = append(words, "more once "+statusOf(search.asked.client.Server.Name, search.asked.client.Progress(), time.Now()))
	}
	if search.checked < search.total-search.unchecked {
		words = append(words, fmt.Sprintf("checking %d of %d places in %s", search.checked, search.total-search.unchecked,
			plural(len(search.projects), "project")))
	}
	if search.left > 0 {
		words = append(words, fmt.Sprintf("%d more places not checked", search.left))
	}
	if search.unchecked > 0 {
		words = append(words, fmt.Sprintf("%d in files no server reads", search.unchecked))
	}
	return strings.Join(words, " · ")
}
