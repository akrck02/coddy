// Package uses is the uses mode: every use of a name, across the projects in sight, listed file by
// file in the sidebar and marked in each file, the lines away from them folded; n goes from one use
// to the next across files, v ticks one as seen. Its views show the same uses as a graph, and the
// projects as a graph of what they depend on.
package uses

import (
	"fmt"
	"sort"

	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/input"
)

// Host is what the uses mode needs from its window beyond any mode's: searching again.
type Host interface {
	mode.Window
	// SearchUsesAgain makes the last search of uses again, from the start.
	SearchUsesAgain()
	// UsersOf asks the server of a use's file for the declaration the use is in and who uses it;
	// done gets them once known. It tells whether it asked.
	UsersOf(use mode.Use, done func(declaration string, users []mode.Use)) bool
}

// Mode is the uses mode.
type Mode struct {
	window mode.Window
	name   string
	home   int
	uses   []mode.Use // in the order n goes through them
	note   string
	seen   map[mode.Use]bool
	// byFile are the uses of each file, as indexes in uses
	byFile map[workspace.FileReference][]int
	// current is the use n went to last, as an index in uses; -1 before the first
	current int
	view    view
	graphs  graphs
}

// New makes the uses mode, with no search made yet.
func New() *Mode {
	return &Mode{seen: map[mode.Use]bool{}, byFile: map[workspace.FileReference][]int{}, current: -1}
}

// Name is the sidebar's title in this mode.
func (uses *Mode) Name() string { return "Uses" }

// Icon is the mode's icon on the rail.
func (uses *Mode) Icon() string { return "scan-search" }

// Description is the rail's tooltip.
func (uses *Mode) Description() string {
	return "Uses (3): every use of a name across the projects, file by file, and their graphs (⇧F12 on a name)"
}

// Start gives the mode its window.
func (uses *Mode) Start(window mode.Window) { uses.window = window }

// ChangesOnly is false: the files listed are those with uses, changed or not.
func (uses *Mode) ChangesOnly() bool { return false }

// ShowsChanges is false: a file opens on its uses.
func (uses *Mode) ShowsChanges() bool { return false }

// Header says how many uses were ticked as seen.
func (uses *Mode) Header() string {
	if len(uses.uses) == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d seen", uses.seenCount(), len(uses.uses))
}

// Progress is none: the status bar counts the review's files, not uses.
func (uses *Mode) Progress() (done, total int) { return 0, 0 }

// FileButton ticks every use of the file as seen.
func (uses *Mode) FileButton(file workspace.FileReference) (viewer.Button, bool) {
	indexes := uses.byFile[file]
	if len(indexes) == 0 {
		return viewer.Button{}, false
	}
	return viewer.Button{Icon: "check-check", Tip: "Every use of the file seen", On: uses.fileSeen(file)}, true
}

// SetUses keeps the uses of a name found so far; a new name starts afresh, the same name keeps the
// uses ticked.
func (uses *Mode) SetUses(name string, home int, found []mode.Use, note string) {
	if name != uses.name || home != uses.home {
		uses.seen, uses.current = map[mode.Use]bool{}, -1
		uses.graphs = graphs{}
	}
	uses.name, uses.home, uses.note = name, home, note
	uses.uses = append(uses.uses[:0], found...)
	uses.sortUses()
	uses.byFile = map[workspace.FileReference][]int{}
	for index, use := range uses.uses {
		uses.byFile[use.File] = append(uses.byFile[use.File], index)
	}
	uses.window.Relist()
}

// sortUses orders the uses: the name's own project first, then the projects with changes, then
// the others; in each, by file and line.
func (uses *Mode) sortUses() {
	rank := uses.projectRanks()
	sort.SliceStable(uses.uses, func(first, second int) bool {
		one, other := uses.uses[first], uses.uses[second]
		if rank[one.File.Project] != rank[other.File.Project] {
			return rank[one.File.Project] < rank[other.File.Project]
		}
		if one.File != other.File {
			return one.File.Project < other.File.Project || one.File.Project == other.File.Project && one.File.Path < other.File.Path
		}
		return one.Line < other.Line || one.Line == other.Line && one.Start < other.Start
	})
}

// projectRanks ranks each project: 0 for the name's own, 1 with changes, 2 for the others.
func (uses *Mode) projectRanks() map[int]int {
	ranks := map[int]int{}
	for index, project := range uses.window.Projects() {
		switch {
		case index == uses.home:
			ranks[index] = 0
		case len(project.Changes) > 0:
			ranks[index] = 1
		default:
			ranks[index] = 2
		}
	}
	return ranks
}

// UsesIn are the uses of a file, by line, as spans of characters.
func (uses *Mode) UsesIn(file workspace.FileReference) map[int][]workspace.Span {
	indexes := uses.byFile[file]
	if len(indexes) == 0 {
		return nil
	}
	marked := map[int][]workspace.Span{}
	for _, index := range indexes {
		use := uses.uses[index]
		marked[use.Line] = append(marked[use.Line], workspace.Span{Start: use.Start, End: use.End})
	}
	return marked
}

// ListedFiles are the files of a project with uses, once a search was made.
func (uses *Mode) ListedFiles(project int) ([]string, bool) {
	var paths []string
	for file := range uses.byFile {
		if file.Project == project {
			paths = append(paths, file.Path)
		}
	}
	sort.Strings(paths)
	return paths, true
}

// ProjectOrder lists the projects as the uses go: the name's own, those with changes, the others.
func (uses *Mode) ProjectOrder() []int {
	rank := uses.projectRanks()
	order := make([]int, len(uses.window.Projects()))
	for index := range order {
		order[index] = index
	}
	sort.SliceStable(order, func(first, second int) bool { return rank[order[first]] < rank[order[second]] })
	return order
}

// Decorate counts the uses of each file, folder and project, and marks a file whose uses were all
// seen.
func (uses *Mode) Decorate(row sidebar.Row) sidebar.Decoration {
	count, seen := uses.countUnder(row)
	if count == 0 {
		return sidebar.Decoration{}
	}
	ink := sidebar.WarningInk
	if seen == count {
		ink = sidebar.PositiveInk
	}
	decoration := sidebar.Decoration{Words: []sidebar.Word{{Text: fmt.Sprint(count), Ink: ink}}}
	if row.Kind == sidebar.FileRow {
		decoration.Mark = sidebar.OpenMark
		if seen == count {
			decoration.Mark = sidebar.DoneMark
		}
	}
	return decoration
}

// countUnder counts the uses in a row's file, or under its folder or project, and those seen.
func (uses *Mode) countUnder(row sidebar.Row) (count, seen int) {
	for file, indexes := range uses.byFile {
		if file.Project != row.Project || !under(row, file.Path) {
			continue
		}
		for _, index := range indexes {
			count++
			if uses.seen[uses.uses[index]] {
				seen++
			}
		}
	}
	return count, seen
}

// under tells whether a file's path is the row's file, or inside its folder or project.
func under(row sidebar.Row, path string) bool {
	switch row.Kind {
	case sidebar.FileRow:
		return path == row.Path
	case sidebar.FolderRow:
		return len(path) > len(row.Path) && path[:len(row.Path)] == row.Path && path[len(row.Path)] == '/'
	case sidebar.ProjectRow:
		return true
	}
	return false
}

// Press ticks every use of a file as seen, or none again; moveOn goes on to the next use not seen.
func (uses *Mode) Press(file workspace.FileReference, moveOn bool) {
	seen := !uses.fileSeen(file)
	for _, index := range uses.byFile[file] {
		uses.seen[uses.uses[index]] = seen
	}
	if seen && moveOn {
		uses.goToUnseen()
	}
}

// fileSeen tells whether every use of a file was seen.
func (uses *Mode) fileSeen(file workspace.FileReference) bool {
	for _, index := range uses.byFile[file] {
		if !uses.seen[uses.uses[index]] {
			return false
		}
	}
	return len(uses.byFile[file]) > 0
}

// seenCount is how many uses were seen.
func (uses *Mode) seenCount() int {
	count := 0
	for _, use := range uses.uses {
		if uses.seen[use] {
			count++
		}
	}
	return count
}

// Commands are the uses mode's keys, while it is on: n and ⇧N through the uses, v to tick one, r to
// search again, and ⇧1 to ⇧3 for its views.
func (uses *Mode) Commands() []mode.Command {
	commands := []mode.Command{
		{Description: "Next / previous use, across files", OnlyWhenOn: true,
			Keys: []pane.Press{{Key: input.KeyN, Label: "n", Step: 1}, {Key: input.KeyN, Shift: true, Label: "⇧N", Step: -1}},
			Do:   uses.step},
		{Description: "Tick the use as seen and go to the next not seen", OnlyWhenOn: true,
			Keys: []pane.Press{pane.Key(input.KeyV, "v")}, Do: func(int) { uses.tick() }},
		{Description: "Search the uses again", OnlyWhenOn: true,
			Keys: []pane.Press{pane.Key(input.KeyR, "r")}, Do: func(int) { uses.searchAgain() }},
	}
	commands = append(commands, mode.Command{Description: "Fit the graph in the window", OnlyWhenOn: true,
		Keys: []pane.Press{pane.Key(input.Key0, "0")}, Do: func(int) {
			uses.graphs.name.fitted, uses.graphs.projects.fitted = false, false
		}})
	for index, name := range viewNames {
		chosen := view(index)
		commands = append(commands, mode.Command{Description: "Show the uses as " + name, OnlyWhenOn: true,
			Keys: []pane.Press{{Key: input.Key1 + input.Key(index), Shift: true, Label: fmt.Sprint("⇧", index+1)}},
			Do:   func(int) { uses.view = chosen }})
	}
	return commands
}

// step goes to the next use (step 1) or the previous one (-1), going round, opening its file.
func (uses *Mode) step(step int) {
	if len(uses.uses) == 0 {
		uses.window.Toast("No uses: press ⇧F12 on a name")
		return
	}
	next := uses.current + step
	if uses.current < 0 && step < 0 {
		next = len(uses.uses) - 1
	}
	uses.goTo((next + len(uses.uses)) % len(uses.uses))
}

// goTo opens a use, in the list view.
func (uses *Mode) goTo(index int) {
	uses.current, uses.view = index, listView
	use := uses.uses[index]
	uses.window.RevealLine(use.File, use.Line)
}

// tick ticks the use n went to last as seen, and goes to the next not seen.
func (uses *Mode) tick() {
	if uses.current < 0 || uses.current >= len(uses.uses) {
		uses.step(1)
		return
	}
	uses.seen[uses.uses[uses.current]] = true
	uses.goToUnseen()
}

// goToUnseen goes to the first use after the current one not seen yet, going round.
func (uses *Mode) goToUnseen() {
	for offset := 1; offset <= len(uses.uses); offset++ {
		index := (max(uses.current, 0) + offset) % len(uses.uses)
		if !uses.seen[uses.uses[index]] {
			uses.goTo(index)
			return
		}
	}
	uses.window.Toast("Every use has been seen")
}

// searchAgain makes the search again, when the window can.
func (uses *Mode) searchAgain() {
	if host, can := uses.window.(Host); can && uses.name != "" {
		host.SearchUsesAgain()
		return
	}
	uses.window.Toast("No search to make again: press ⇧F12 on a name")
}
