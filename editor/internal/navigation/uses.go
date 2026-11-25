package navigation

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/work"
	"github.com/akrck02/coddy/editor/internal/workspace"
)

const (
	// usesCheckLimit is how many places of other files are checked at most, the nearest first
	usesCheckLimit = 300
	// usesBatch is how many places one job asks a server about, so the list grows as it goes
	usesBatch = 20
	// checkWait is how long one place's question may take: it is asked only once its server has
	// read the projects, as one importing them for minutes would let it run out, answering nothing
	checkWait = 30 * time.Second
)

// Uses searches the uses of names across the projects open, one search at a time: a newer
// search leaves the answers of the one before unheard.
type Uses struct {
	host   Host
	work   *work.Queue
	search *Search
}

// NewUses is a searcher of uses that asks a window and runs its jobs on its queue.
func NewUses(host Host, queue *work.Queue) Uses { return Uses{host: host, work: queue} }

// Last is the last search made, nil before the first.
func (uses *Uses) Last() *Search { return uses.search }

// Grep searches every project for a name as a whole word at once, as many as are read together,
// by folder.
func Grep(stop context.Context, queue *work.Queue, projects []string, name string, options git.GrepOptions) map[string][]git.Hit {
	found := make([][]git.Hit, len(projects))
	var group sync.WaitGroup
	for position, project := range projects {
		group.Add(1)
		go func() {
			defer group.Done()
			queue.Read(stop, func() {
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

// Begin lists the uses the file's server gave, and starts checking the other places that hold
// the name, when its declaration is known.
func (uses *Uses) Begin(question Question, answer Answer) {
	search := &Search{Question: question, busy: answer.Busy}
	search.found, search.checking, search.projects = map[string]lsp.Location{}, map[string]bool{}, map[string]bool{}
	for _, location := range answer.Locations {
		search.found[placeKey(location)] = location
	}
	if len(answer.Declared) > 0 {
		projects := uses.host.Projects()
		search.declaration, _ = FromJar(projects, answer.Declared[0], search.Name)
		_, search.known = ReferenceOf(projects, lsp.PathOf(search.declaration.URI))
	}
	if search.known {
		uses.addCandidates(search, answer.Hits)
	}
	uses.search = search
	uses.show(search, true)
	uses.Advance()
}

// addCandidates keeps the places that hold the name and the server did not give, the nearest
// first: the name's own project, then the projects with changes, then the others.
func (uses *Uses) addCandidates(search *Search, hits map[string][]git.Hit) {
	places := candidatesOf(search, hits)
	rank := projectRanks(uses.host.Projects(), search.Home)
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

// candidatesOf are the places git found that hold the name as a word and the server did not give.
func candidatesOf(search *Search, hits map[string][]git.Hit) []candidate {
	var places []candidate
	for project, found := range hits {
		for _, hit := range found {
			path := filepath.Join(project, filepath.FromSlash(hit.Path))
			for _, column := range wordColumns(hit.Text, search.Name) {
				at := lsp.Position{Line: hit.Line - 1, Character: lsp.UTF16Column(hit.Text, len([]rune(hit.Text[:column])))}
				if _, known := search.found[placeKey(lsp.Location{URI: lsp.FileURI(path), Range: lsp.Range{Start: at}})]; !known {
					places = append(places, candidate{path: path, project: project, at: at})
				}
			}
		}
	}
	return places
}

// projectRanks orders the projects for checking: the home one, those with changes, the others.
func projectRanks(projects []*workspace.Project, home int) map[string]int {
	ranks := map[string]int{}
	for index, project := range projects {
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
