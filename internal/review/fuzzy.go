package review

import (
	"sort"
	"strings"
	"unicode"
)

// Candidate is something the fuzzy finder can find: a file of a project.
type Candidate struct {
	Project int    // which project
	Path    string // the file's path in it
	Label   string // what is matched and shown: "project/path"
	Changed bool
}

// Match is a candidate that matches what was typed, with how well.
type Match struct {
	Candidate Candidate
	Score     int
	Positions []int // the matched characters of the label, by byte
}

// FuzzyFind finds the candidates whose label holds the query's characters in order, best first,
// at most limit of them. Changed files come first among equally good matches; an empty query lists
// the changed files.
func FuzzyFind(candidates []Candidate, query string, limit int) []Match {
	query = strings.TrimSpace(query)
	var matches []Match
	for _, candidate := range candidates {
		if query == "" {
			if candidate.Changed {
				matches = append(matches, Match{Candidate: candidate})
			}
			continue
		}
		score, positions, found := fuzzyScore(candidate.Label, query)
		if !found {
			continue
		}
		if candidate.Changed {
			score += 8
		}
		matches = append(matches, Match{Candidate: candidate, Score: score, Positions: positions})
	}
	sort.SliceStable(matches, func(first, second int) bool {
		if matches[first].Score != matches[second].Score {
			return matches[first].Score > matches[second].Score
		}
		return len(matches[first].Candidate.Label) < len(matches[second].Candidate.Label)
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

// fuzzyScore matches the query's characters in order, ignoring case and spaces. Characters that
// follow each other, that start a word or a path part, and that fall in the file's name score more,
// so "ordsvc" finds OrderService.java before a path that merely contains those letters.
func fuzzyScore(label, query string) (int, []int, bool) {
	lowerLabel := strings.ToLower(label)
	lowerQuery := strings.ToLower(strings.ReplaceAll(query, " ", ""))
	nameStart := strings.LastIndexByte(label, '/') + 1
	positions := make([]int, 0, len(lowerQuery))
	score := 0
	labelIndex := 0
	previousMatch := -2
	for _, wanted := range lowerQuery {
		found := -1
		for index := labelIndex; index < len(lowerLabel); index++ {
			if rune(lowerLabel[index]) == wanted {
				found = index
				break
			}
		}
		if found < 0 {
			return 0, nil, false
		}
		score++
		if found == previousMatch+1 {
			score += 5
		}
		if startsWord(label, found) {
			score += 4
		}
		if found >= nameStart {
			score += 2
		}
		positions = append(positions, found)
		previousMatch = found
		labelIndex = found + 1
	}
	if strings.Contains(strings.ToLower(label[nameStart:]), lowerQuery) {
		score += 20
	}
	return score, positions, true
}

// startsWord tells whether a character starts a word: after a separator, or a capital after a
// small letter (the S of OrderService).
func startsWord(label string, index int) bool {
	if index == 0 {
		return true
	}
	previous, current := rune(label[index-1]), rune(label[index])
	if strings.ContainsRune("/._- ", previous) {
		return true
	}
	return unicode.IsUpper(current) && unicode.IsLower(previous)
}
