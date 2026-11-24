package finder

import (
	"path"
	"sort"
	"strings"

	"github.com/akrck02/code-review/internal/git"
)

// Use is a place a name is used, as a language server found it: a file of a project, the line from
// 1, where the name starts in it in bytes from 0, how many characters it takes, and the line's text.
type Use struct {
	Project      int
	Path         string
	Line, Column int
	Length       int
	Text         string
}

// ShowUses opens the finder on places a language server gave, named by what they are ("uses of
// Keeper"): the development's own files first, then by project, file and line.
func (finder *Finder) ShowUses(window Window, name string, uses []Use) {
	finder.Show(ReferencesScope)
	finder.Name, finder.Home, finder.Note = name, -1, ""
	finder.setUses(window, uses)
}

// ShowUsesFrom is ShowUses with the uses of one project, the name's own, listed first.
func (finder *Finder) ShowUsesFrom(window Window, name string, home int, uses []Use) {
	finder.ShowUses(window, name, uses)
	finder.Home = home
	finder.setUses(window, uses)
}

// UpdateUses replaces the uses listed, as more are found, keeping what was typed to narrow them.
func (finder *Finder) UpdateUses(window Window, uses []Use) {
	if !finder.Open || finder.Scope != ReferencesScope {
		return
	}
	finder.setUses(window, uses)
	finder.Stale = true
}

// setUses keeps the uses, sorted: the home project's first, then the development's own files,
// then by project, file and line.
func (finder *Finder) setUses(window Window, uses []Use) {
	finder.references = finder.references[:0]
	for _, use := range uses {
		if use.Project < 0 || use.Project >= len(window.Projects()) {
			continue
		}
		change, changed := window.Projects()[use.Project].ChangeOf(use.Path)
		hit := git.Hit{Path: use.Path, Line: use.Line, Column: use.Column, Text: use.Text}
		finder.references = append(finder.references, newContentHit(window, use.Project, change, changed, hit, use.Length))
	}
	sort.SliceStable(finder.references, func(first, second int) bool {
		one, other := finder.references[first], finder.references[second]
		if (one.project == finder.Home) != (other.project == finder.Home) {
			return one.project == finder.Home
		}
		if one.changed != other.changed {
			return one.changed
		}
		if one.project != other.project {
			return one.project < other.project
		}
		if one.Path != other.Path {
			return one.Path < other.Path
		}
		return one.Line < other.Line
	})
}

// narrowed are the uses whose project, file or line hold the query, ignoring case.
func narrowed(uses []ContentHit, query string, window Window) []ContentHit {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return uses
	}
	var kept []ContentHit
	for _, use := range uses {
		where := path.Join(window.Projects()[use.project].Name, use.Path)
		if strings.Contains(strings.ToLower(where+" "+use.Text), query) {
			kept = append(kept, use)
		}
	}
	return kept
}
