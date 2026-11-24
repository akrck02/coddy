package navigation

import (
	"fmt"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/servers"
)

// Question is a search of uses as asked: the name, the place of the file open it was asked at,
// that file's server, and the project the name is in.
type Question struct {
	Name  string
	Home  int
	Asked Asking
	At    lsp.Position
}

// Answer is what the file's server and git gave a question: the uses the server knows, where the
// name is declared, and the places of each project, by folder, that hold the name.
type Answer struct {
	Locations, Declared []lsp.Location
	Hits                map[string][]git.Hit
	// Busy tells that the server was still reading the projects: it is asked again once done
	Busy bool
}

// Search is a list of uses being made: those the file's server knows, then places in other
// projects that hold the name, each checked by asking its own server where the name is declared.
type Search struct {
	Question
	busy bool
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

// Busy tells whether the file's server was still reading the projects when asked, and is to be
// asked again once it is done.
func (search *Search) Busy() bool { return search.busy }

// Noted is the note shown last on how the search goes.
func (search *Search) Noted() string { return search.noted }

// shown tells which uses a search has: how many found, probable and checked.
func (search *Search) shown() [3]int {
	return [3]int{len(search.found), len(search.probable), search.checked}
}

// note says how the list of uses is still being made, or what it left out.
func (search *Search) note() string {
	asked := search.Asked.Client
	if !search.known {
		return "only " + asked.Server.Name + "'s: its declaration is not in a project open"
	}
	var words []string
	if search.busy {
		words = append(words, "more once "+servers.StatusOf(asked.Server.Name, asked.Progress(), time.Now()))
	}
	if waiting := search.waitingFor; waiting != nil && waiting != asked {
		words = append(words, "some wait for "+servers.StatusOf(waiting.Server.Name, waiting.Progress(), time.Now()))
	}
	if search.checked < search.total-search.unchecked {
		words = append(words, fmt.Sprintf("checking %d of %d places in %s", search.checked, search.total-search.unchecked,
			counted(len(search.projects), "project")))
	}
	if probable := len(search.probable); probable > 0 {
		words = append(words, counted(probable, "probable use")+" not confirmed yet, marked ?")
	}
	if search.left > 0 {
		words = append(words, fmt.Sprintf("%d more places not checked", search.left))
	}
	if search.unchecked > 0 {
		words = append(words, fmt.Sprintf("%d in files no server reads", search.unchecked))
	}
	return strings.Join(words, " · ")
}

// counted is a number of things, the word in the plural unless there is one.
// TODO(akrck02): one way to count things for every package (proposal 11, T12).
func counted(count int, thing string) string {
	if count == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", count, thing)
}
