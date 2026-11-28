// Package uses is the uses mode: every use of a name, across the projects in sight, listed file by
// file in the sidebar and marked in each file, the lines away from them folded; n goes from one use
// to the next across files, v ticks one as seen. Its views show the same uses as a graph, and the
// projects as a graph of what they depend on.
package uses

import (
	"fmt"
	"sort"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/controls"
	"github.com/akrck02/coddy/editor/internal/mode"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/sidebar"
	"github.com/akrck02/coddy/editor/internal/viewer"
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
	// tabWords and pageWords are the titles' counts, written when what they count changes
	tabWords, pageWords pane.Words[usesTally]
	// fittedNote is the note as wide as the tab shows it, made when the note or the width change
	fittedNote controls.FittedText
	seen       map[mode.Use]bool
	// byFile are the uses of each file, as indexes in uses
	byFile map[codebase.FileReference][]int
	// current is the use n went to last, as an index in uses; -1 before the first
	current int
	view    view
	graphs  graphs
}

// New makes the uses mode, with no search made yet.
func New() *Mode {
	return &Mode{seen: map[mode.Use]bool{}, byFile: map[codebase.FileReference][]int{}, current: -1}
}

// Name is the sidebar's title in this mode.
func (uses *Mode) Name() string { return "Uses" }

// Icon is the mode's icon on the rail.
func (uses *Mode) Icon() string { return "scan-search" }

// Description is the rail's tooltip.
func (uses *Mode) Description() string {
	return "Uses (3): every use of a name across the projects, file by file, and their graphs (⌥F7 on a name)"
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
	return headerOf(uses.tally())
}

// probableCount is how many uses no server confirmed yet.
func (uses *Mode) probableCount() int {
	count := 0
	for _, use := range uses.uses {
		if use.Probable {
			count++
		}
	}
	return count
}

// FileButton ticks every use of the file as seen.
func (uses *Mode) FileButton(file codebase.FileReference) (viewer.Button, bool) {
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
	uses.forgetNameScene()
	uses.uses = append(uses.uses[:0], found...)
	uses.sortUses()
	uses.byFile = map[codebase.FileReference][]int{}
	for index, use := range uses.uses {
		uses.byFile[use.File] = append(uses.byFile[use.File], index)
	}
	uses.window.Relist()
}

// SetUsesNote says again how the search goes, the uses unchanged: the note changes with the
// servers' progress, many times a second, and the uses need not be laid out again for it.
func (uses *Mode) SetUsesNote(note string) { uses.note = note }

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
func (uses *Mode) UsesIn(file codebase.FileReference) map[int][]codebase.Span {
	indexes := uses.byFile[file]
	if len(indexes) == 0 {
		return nil
	}
	marked := map[int][]codebase.Span{}
	for _, index := range indexes {
		use := uses.uses[index]
		marked[use.Line] = append(marked[use.Line], codebase.Span{Start: use.Start, End: use.End})
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

// Decorate counts the uses of each file, folder and project, those not confirmed yet apart ("2+1?"),
// and marks a file whose uses were all seen.
func (uses *Mode) Decorate(row sidebar.Row) sidebar.Decoration {
	count, seen, probable := uses.countUnder(row)
	if count == 0 {
		return sidebar.Decoration{}
	}
	ink := sidebar.WarningInk
	if seen == count {
		ink = sidebar.PositiveInk
	}
	decoration := sidebar.Decoration{Words: []sidebar.Word{{Text: countText(count-probable, probable), Ink: ink}}}
	if row.Kind == sidebar.FileRow {
		decoration.Mark = sidebar.OpenMark
		if seen == count {
			decoration.Mark = sidebar.DoneMark
		}
	}
	return decoration
}

// countUnder counts the uses in a row's file, or under its folder or project, those seen, and
// those not confirmed yet.
func (uses *Mode) countUnder(row sidebar.Row) (count, seen, probable int) {
	for file, indexes := range uses.byFile {
		if file.Project != row.Project || !under(row, file.Path) {
			continue
		}
		for _, index := range indexes {
			count++
			if uses.seen[uses.uses[index]] {
				seen++
			}
			if uses.uses[index].Probable {
				probable++
			}
		}
	}
	return count, seen, probable
}

// countText writes a count of uses, those not confirmed yet apart: "3", "2+1?", "1?".
func countText(sure, probable int) string {
	switch {
	case probable == 0:
		return fmt.Sprint(sure)
	case sure == 0:
		return fmt.Sprint(probable, "?")
	}
	return fmt.Sprint(sure, "+", probable, "?")
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
func (uses *Mode) Press(file codebase.FileReference, moveOn bool) {
	seen := !uses.fileSeen(file)
	for _, index := range uses.byFile[file] {
		uses.seen[uses.uses[index]] = seen
	}
	uses.window.Relist() // the rows' ticks and the counts are made with the rows
	if seen && moveOn {
		uses.goToUnseen()
	}
}

// fileSeen tells whether every use of a file was seen.
func (uses *Mode) fileSeen(file codebase.FileReference) bool {
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

// Commands are the uses mode's keys, while it is on: F3 and ⇧F3 through the uses, ⌥V to tick one,
// ⌥R to search again, ⌥0 to fit the graph, and ⌥1 to ⌥3 for its views. Every one has Alt or is a
// function key, so that none is a letter typed by mistake.
func (uses *Mode) Commands() []mode.Command {
	commands := []mode.Command{
		{ID: "step-use", Icon: "arrow-down-up", Description: "Next / previous use, across files", OnlyWhenOn: true,
			Keys: []pane.Press{{Key: input.KeyF3, Label: "F3", Step: 1}, {Key: input.KeyF3, Shift: true, Label: "⇧F3", Step: -1}},
			Do:   uses.step},
		{ID: "tick-use", Icon: "check", Description: "Tick the use as seen and go to the next not seen", OnlyWhenOn: true,
			Keys: []pane.Press{{Key: input.KeyV, Alt: true, Label: "⌥V"}}, Do: func(int) { uses.tick() }},
		{ID: "search-again", Icon: "refresh-cw", Description: "Search the uses again", OnlyWhenOn: true,
			Keys: []pane.Press{{Key: input.KeyR, Alt: true, Label: "⌥R"}}, Do: func(int) { uses.searchAgain() }},
	}
	commands = append(commands, mode.Command{ID: "fit-graph", Icon: "scan", Description: "Fit the graph in the window", OnlyWhenOn: true,
		Keys: []pane.Press{{Key: input.Key0, Alt: true, Label: "⌥0"}}, Do: func(int) {
			uses.graphs.name.fitted, uses.graphs.projects.fitted = false, false
		}})
	for index, name := range viewNames {
		chosen := view(index)
		commands = append(commands, mode.Command{ID: fmt.Sprint("view-", index+1), Icon: "eye", Description: "Show the uses as " + name, OnlyWhenOn: true,
			Keys: []pane.Press{{Key: input.Key1 + input.Key(index), Alt: true, Label: fmt.Sprint("⌥", index+1)}},
			Do:   func(int) { uses.view = chosen }})
	}
	return commands
}

// step goes to the next use (step 1) or the previous one (-1), going round, opening its file.
func (uses *Mode) step(step int) {
	if len(uses.uses) == 0 {
		uses.window.Toast("No uses: press ⌥F7 on a name")
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
	uses.window.Toast("No search to make again: press ⌥F7 on a name")
}

// headerOf says how many uses were seen, and how many are only probable: "1 of 4 seen · 1 probable".
func headerOf(tally usesTally) string {
	header := fmt.Sprintf("%d of %d seen", tally.seen, tally.uses)
	if tally.probables > 0 {
		header += fmt.Sprintf(" · %d probable", tally.probables)
	}
	return header
}
