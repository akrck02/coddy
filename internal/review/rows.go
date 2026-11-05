package review

import "github.com/akrck02/code-review/internal/git"

// Row is a row of a diff as shown: a line of code, or a fold standing for unchanged lines hidden
// between two changes.
type Row struct {
	Line git.DiffLine
	// Folded, when above 0, makes this row a fold of that many unchanged lines, starting at
	// FoldStart in the diff.
	Folded    int
	FoldStart int
}

// IsFold tells whether the row stands for hidden lines.
func (row Row) IsFold() bool { return row.Folded > 0 }

// DiffRows shows a diff with the unchanged lines more than context lines away from a change folded
// away, except the folds opened (by where they start in the diff). A negative context shows all.
func DiffRows(diff []git.DiffLine, context int, opened map[int]bool) []Row {
	rows := make([]Row, 0, len(diff))
	if context < 0 {
		for _, line := range diff {
			rows = append(rows, Row{Line: line})
		}
		return rows
	}
	near := nearChanges(diff, context)
	for index := 0; index < len(diff); {
		if near[index] {
			rows = append(rows, Row{Line: diff[index]})
			index++
			continue
		}
		start := index
		for index < len(diff) && !near[index] {
			index++
		}
		if opened[start] {
			for _, line := range diff[start:index] {
				rows = append(rows, Row{Line: line})
			}
			continue
		}
		rows = append(rows, Row{Folded: index - start, FoldStart: start})
	}
	return rows
}

// nearChanges marks the lines that are changes or within context lines of one.
func nearChanges(diff []git.DiffLine, context int) []bool {
	near := make([]bool, len(diff))
	lastChange := -1 << 30
	for index, line := range diff {
		if line.Kind != git.Context {
			lastChange = index
		}
		if index-lastChange <= context {
			near[index] = true
		}
	}
	lastChange = 1 << 30
	for index := len(diff) - 1; index >= 0; index-- {
		if diff[index].Kind != git.Context {
			lastChange = index
		}
		if lastChange-index <= context {
			near[index] = true
		}
	}
	return near
}

// HunkStarts are the rows where a run of changes starts, to jump from one to the next.
func HunkStarts(rows []Row) []int {
	var starts []int
	previousChanged := false
	for index, row := range rows {
		changed := !row.IsFold() && row.Line.Kind != git.Context
		if changed && !previousChanged {
			starts = append(starts, index)
		}
		previousChanged = changed
	}
	return starts
}

// Counts tells how many lines a diff adds and removes.
func Counts(diff []git.DiffLine) (added, removed int) {
	for _, line := range diff {
		switch line.Kind {
		case git.Insertion:
			added++
		case git.Removal:
			removed++
		}
	}
	return added, removed
}
