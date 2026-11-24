package finder

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/akrck02/code-review/internal/git"
)

// searchedProject is a project the content search reads: its place in the window and its folder.
type searchedProject struct {
	project int
	path    string
}

// searchContent looks for a text in every project shown, a few at a time, after a short pause
// so that typing a word does not start a search for each of its letters.
func (finder *Finder) searchContent(window Window, text string) {
	if tooShort(text) {
		window.Cancel(contentSearch)
		finder.Hits, finder.searching, finder.truncated = nil, false, false
		return
	}
	projects := searchableProjects(window)
	options := git.GrepOptions{WorkingTree: window.WorkingTree(), PerFile: contentPerFile}
	synchronous := window.Synchronous()
	finder.searching = true
	window.Run(contentSearch, func(stop context.Context) func() {
		if !synchronous {
			select {
			case <-time.After(contentPause):
			case <-stop.Done():
				return nil // a newer search, typed during the pause
			}
		}
		found := grepProjects(stop, window, projects, text, options)
		return func() { finder.takeHits(window, projects, found) }
	})
}

// searchableProjects lists the projects the window lets the finder search.
func searchableProjects(window Window) []searchedProject {
	var projects []searchedProject
	for index, project := range window.Projects() {
		if window.Searchable(index) {
			projects = append(projects, searchedProject{index, project.Path})
		}
	}
	return projects
}

// grepProjects searches every project at once, as many as the window lets read together, and
// returns each one's hits in the same order.
func grepProjects(stop context.Context, window Window, projects []searchedProject, text string, options git.GrepOptions) [][]git.Hit {
	found := make([][]git.Hit, len(projects))
	var group sync.WaitGroup
	for position, project := range projects {
		group.Add(1)
		go func() {
			defer group.Done()
			window.Read(stop, func() {
				found[position], _ = git.Grep(stop, project.path, text, options) // a project git cannot search shows no lines
			})
		}()
	}
	group.Wait()
	return found
}

// takeHits lists the hits found, the development's own files first, up to the limit.
func (finder *Finder) takeHits(window Window, projects []searchedProject, found [][]git.Hit) {
	finder.Hits, finder.truncated = nil, false
	for position, hits := range found {
		index := projects[position].project
		if index >= len(window.Projects()) {
			continue
		}
		project := window.Projects()[index]
		for _, hit := range hits {
			change, changed := project.ChangeOf(hit.Path)
			finder.Hits = append(finder.Hits, newContentHit(window, index, change, changed, hit, 0))
		}
	}
	// the development's own files first: they are what is being reviewed
	sort.SliceStable(finder.Hits, func(first, second int) bool {
		return finder.Hits[first].changed && !finder.Hits[second].changed
	})
	if len(finder.Hits) > contentHitLimit {
		finder.Hits, finder.truncated = finder.Hits[:contentHitLimit], true
	}
	finder.searching = false
	finder.selected = 0
	finder.list.Offset = 0
}
