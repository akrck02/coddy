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
	"unicode"
	"unicode/utf8"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
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
	// probable are the places not checked yet whose file names the declaration for sure
	probable  map[string]lsp.Location
	pending   []candidate
	checking  map[string]bool // the servers asked about some places now, by key
	checked   int
	total     int
	projects  map[string]bool // the projects the places are in
	left      int             // places past the limit, not checked
	unchecked int             // places no server reads
	noted     string          // the note shown last
	// shownUses tells which uses were shown last: how many found, probable and checked
	shownUses [3]int
	// waitingFor is a server still reading the projects that places wait for: asked now, it would
	// answer nothing, and a use be lost
	waitingFor *lsp.Client
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
	// checkWait is how long one place's question may take: it is asked only once its server has
	// read the projects, as one importing them for minutes would let it run out, answering nothing
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
func (application *Window) grepAll(stop context.Context, projects []string, name string, options git.GrepOptions) map[string][]git.Hit {
	found := make([][]git.Hit, len(projects))
	var group sync.WaitGroup
	for position, project := range projects {
		group.Add(1)
		go func() {
			defer group.Done()
			application.work.Read(stop, func() {
				// a project git cannot search adds no places to check
				found[position], _ = git.Grep(stop, project, name, options)
			})
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
	application.showUses(search, true)
	application.advanceUses()
}

// inProjects tells whether a location is a file of a project open.
func (application *Window) inProjects(location lsp.Location) bool {
	_, found := application.referenceOf(lsp.PathOf(location.URI))
	return found
}

// placeKey names a place by its file, line and character.
func placeKey(location lsp.Location) string {
	start := location.Range.Start
	return lsp.PathOf(location.URI) + ":" + strconv.Itoa(start.Line) + ":" + strconv.Itoa(start.Character)
}

// wordColumns are where a name stands as a whole word in a line, in bytes: git grep gives the
// first only.
func wordColumns(line, name string) []int {
	var columns []int
	isName := func(at int) bool {
		character, _ := utf8.DecodeRuneInString(line[at:])
		return character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character)
	}
	for from := 0; from <= len(line)-len(name); {
		at := strings.Index(line[from:], name)
		if at < 0 {
			break
		}
		at += from
		before := at > 0 && isNameBefore(line, at)
		after := at+len(name) < len(line) && isName(at+len(name))
		if !before && !after {
			columns = append(columns, at)
		}
		from = at + len(name)
	}
	return columns
}

// isNameBefore tells whether the character before a byte of a line is part of a name.
func isNameBefore(line string, at int) bool {
	character, _ := utf8.DecodeLastRuneInString(line[:at])
	return character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character)
}

// addCandidates keeps the places that hold the name and the server did not give, the nearest
// first: the name's own project, then the projects with changes, then the others.
func (application *Window) addCandidates(search *usesSearch, hits map[string][]git.Hit) {
	var places []candidate
	for project, found := range hits {
		for _, hit := range found {
			path := filepath.Join(project, filepath.FromSlash(hit.Path))
			for _, column := range wordColumns(hit.Text, search.name) {
				at := lsp.Position{Line: hit.Line - 1, Character: lsp.UTF16Column(hit.Text, len([]rune(hit.Text[:column])))}
				if _, known := search.found[placeKey(lsp.Location{URI: lsp.FileURI(path), Range: lsp.Range{Start: at}})]; !known {
					places = append(places, candidate{path: path, project: project, at: at})
				}
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
	markProbable(search)
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
	search.waitingFor = nil
	for _, place := range search.pending {
		client, key, state := application.clientOf(place)
		switch {
		case state == "none":
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
		application.checkPlaces(search, key, clients[key], batch)
	}
	application.noteUses(search)
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
	application.work.Run("uses "+key, func(stop context.Context) func() {
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
			if application.uses != search {
				return // a newer search
			}
			delete(search.checking, key)
			application.keepChecked(search, places, answers)
		}
	})
}

// commentReader tells which places of files are in comments, colouring each file once.
type commentReader map[string]commented

// commented is a file read, as text and as coloured lines.
type commented struct {
	text     []string
	coloured []highlight.Line
}

// references are what comes before a name in a comment that links to it, as a use does: Javadoc's
// {@link Name} and @see, KDoc's [Name].
var references = []string{"{@link ", "{@linkplain ", "@see ", "@throws ", "@exception ", "["}

// mentionOnly tells whether a place is in a comment that only mentions the name, in words: a
// server may answer where a word of a comment is declared, but a word is not a use.
func (reader commentReader) mentionOnly(place candidate) bool {
	file, read := reader[place.path]
	if !read {
		content, _ := os.ReadFile(place.path)
		text := strings.ReplaceAll(string(content), "\r\n", "\n")
		file = commented{text: strings.Split(text, "\n"), coloured: highlight.Lines(place.path, text)}
		reader[place.path] = file
	}
	if place.at.Line >= len(file.text) || place.at.Line >= len(file.coloured) {
		return false
	}
	line := file.text[place.at.Line]
	characters := lsp.CharacterColumn(line, place.at.Character)
	if !file.coloured[place.at.Line].InComment(highlight.Column(line, characters)) {
		return false
	}
	before := string([]rune(line)[:characters])
	for _, reference := range references {
		if strings.HasSuffix(before, reference) {
			return false
		}
	}
	return true
}

// askDeclaration asks a server where the name at a place is declared, telling it the file when it
// does not have it open, and closing it after.
func askDeclaration(stop context.Context, client *lsp.Client, place candidate) []lsp.Location {
	ctx, cancel := context.WithTimeout(stop, requestWait)
	defer cancel()
	defer openFor(client, place.path)()
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
		delete(search.probable, placeKey(placeLocation(place, search.name)))
		for _, answer := range answers[index] {
			mapped, fromJar := application.fromJar(answer, search.name)
			sameLine := mapped.Range.Start.Line == search.declaration.Range.Start.Line
			if lsp.PathOf(mapped.URI) == declaration && (sameLine || fromJar) {
				use := placeLocation(place, search.name)
				search.found[placeKey(use)] = use
				break
			}
		}
	}
	application.showUses(search, false)
}

// askUsesAgain asks the file's server again for the uses, now that it has read the projects.
func (application *Window) askUsesAgain(search *usesSearch) {
	search.busy = false
	asked, at := search.asked, search.at
	application.work.Run("references again", func(stop context.Context) func() {
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
			application.showUses(search, false)
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
	if waiting := search.waitingFor; waiting != nil && waiting != search.asked.client {
		words = append(words, "some wait for "+statusOf(waiting.Server.Name, waiting.Progress(), time.Now()))
	}
	if search.checked < search.total-search.unchecked {
		words = append(words, fmt.Sprintf("checking %d of %d places in %s", search.checked, search.total-search.unchecked,
			plural(len(search.projects), "project")))
	}
	if probable := len(search.probable); probable > 0 {
		words = append(words, plural(probable, "probable use")+" not confirmed yet, marked ?")
	}
	if search.left > 0 {
		words = append(words, fmt.Sprintf("%d more places not checked", search.left))
	}
	if search.unchecked > 0 {
		words = append(words, fmt.Sprintf("%d in files no server reads", search.unchecked))
	}
	return strings.Join(words, " · ")
}
