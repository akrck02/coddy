package workspace

import "github.com/akrck02/code-review/internal/git"

// Span is a range of columns in a line, from Start up to End.
type Span struct{ Start, End int }

// largestShareChanged is how much of a line may change for its changed part to be marked: past it
// the whole line is new, and marking nearly all of it would only add noise.
const largestShareChanged = 0.7

// InlineChanges finds, in each removed line that is followed by the line that replaced it, the
// part that changed: what is left between what both lines start and end with. Removed and added
// lines are paired in order within each run of changes, as diff tools do. text gives a line's text
// as shown (tabs turned into spaces). The result is by row.
func InlineChanges(rows []Row, text func(line git.DiffLine) []rune) map[int]Span {
	spans := map[int]Span{}
	for start := 0; start < len(rows); {
		removals, insertions := changeRun(rows, start)
		if len(removals) == 0 && len(insertions) == 0 {
			start++
			continue
		}
		for pair := 0; pair < len(removals) && pair < len(insertions); pair++ {
			removed, added := removals[pair], insertions[pair]
			oldSpan, newSpan, worth := changedMiddle(text(rows[removed].Line), text(rows[added].Line))
			if worth {
				spans[removed], spans[added] = oldSpan, newSpan
			}
		}
		start += len(removals) + len(insertions)
	}
	return spans
}

// changeRun is the run of changed rows from start: its removed rows, then its added rows.
func changeRun(rows []Row, start int) (removals, insertions []int) {
	index := start
	for ; index < len(rows) && !rows[index].IsFold() && rows[index].Line.Kind == git.Removal; index++ {
		removals = append(removals, index)
	}
	for ; index < len(rows) && !rows[index].IsFold() && rows[index].Line.Kind == git.Insertion; index++ {
		insertions = append(insertions, index)
	}
	return removals, insertions
}

// changedMiddle finds what two lines do not share at their start and end.
func changedMiddle(old, new []rune) (oldSpan, newSpan Span, worth bool) {
	prefix := 0
	for prefix < len(old) && prefix < len(new) && old[prefix] == new[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(new)-prefix && old[len(old)-1-suffix] == new[len(new)-1-suffix] {
		suffix++
	}
	oldSpan = Span{prefix, len(old) - suffix}
	newSpan = Span{prefix, len(new) - suffix}
	shared := prefix + suffix
	if shared == 0 || (oldSpan.Start == oldSpan.End && newSpan.Start == newSpan.End) {
		return oldSpan, newSpan, false
	}
	longest := max(len(old), len(new))
	return oldSpan, newSpan, float64(longest-shared)/float64(longest) <= largestShareChanged
}

// Mark is how a line of the file is marked in the gutter, as Zed marks it.
type Mark int

const (
	NoMark       Mark = iota
	AddedMark         // a new line
	ModifiedMark      // a line that replaced others
	RemovedAbove      // lines were removed just above this one
	RemovedBelow      // lines were removed just below this one: at the end of the file
)

// LineMarks marks the lines of the file as it is now (by number, from 1) from its diff: added
// lines, lines that replaced removed ones, and where lines were removed.
func LineMarks(diff []git.DiffLine) map[int]Mark {
	marks := map[int]Mark{}
	lastNew := 0
	for index := 0; index < len(diff); {
		if diff[index].Kind == git.Context {
			lastNew = diff[index].NewNumber
			index++
			continue
		}
		removed := 0
		for ; index < len(diff) && diff[index].Kind == git.Removal; index++ {
			removed++
		}
		var added []int
		for ; index < len(diff) && diff[index].Kind == git.Insertion; index++ {
			added = append(added, diff[index].NewNumber)
		}
		for _, number := range added {
			marks[number] = AddedMark
			if removed > 0 {
				marks[number] = ModifiedMark
			}
		}
		if len(added) > 0 {
			lastNew = added[len(added)-1]
			continue
		}
		// only removals: mark the line after them, or the last line at the end of the file
		if index < len(diff) {
			if marks[diff[index].NewNumber] == NoMark {
				marks[diff[index].NewNumber] = RemovedAbove
			}
		} else if lastNew > 0 && marks[lastNew] == NoMark {
			marks[lastNew] = RemovedBelow
		}
	}
	return marks
}
