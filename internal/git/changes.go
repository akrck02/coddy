package git

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Status is what happened to a file in a development.
type Status int

const (
	Unchanged Status = iota
	Added
	Modified
	Deleted
	Renamed
	Copied
	TypeChanged
)

// Letter is the one-letter mark git gives a status.
func (status Status) Letter() string {
	switch status {
	case Added:
		return "A"
	case Modified:
		return "M"
	case Deleted:
		return "D"
	case Renamed:
		return "R"
	case Copied:
		return "C"
	case TypeChanged:
		return "T"
	}
	return ""
}

// Change is a file a development added, changed, deleted or renamed.
type Change struct {
	Status  Status
	Path    string // where the file is now (where it was, for a deleted file)
	OldPath string // where it was at the base: differs from Path for a rename or a copy
	// Untracked is a new file git does not know about yet: it is only on disk.
	Untracked bool
}

// Comparison says what a development's changes are measured between.
type Comparison struct {
	MergeBase string
	// WorkingTree compares the base with the files on disk, so changes not committed yet count;
	// otherwise the base is compared with the last commit (HEAD).
	WorkingTree bool
}

// Changes lists the files a development changed since it left the base.
func Changes(repository string, comparison Comparison) ([]Change, error) {
	arguments := []string{"diff", "--name-status", "-z", "-M", "--no-ext-diff", "--no-textconv", comparison.MergeBase}
	if !comparison.WorkingTree {
		arguments = append(arguments, "HEAD")
	}
	output, err := run(repository, append(arguments, "--")...)
	if err != nil {
		return nil, err
	}
	changes := parseNameStatus(splitNull(output))
	if !comparison.WorkingTree {
		return changes, nil
	}
	untracked, err := run(repository, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return changes, err
	}
	for _, path := range splitNull(untracked) {
		changes = append(changes, Change{Status: Added, Path: path, OldPath: path, Untracked: true})
	}
	return changes, nil
}

// parseNameStatus reads `git diff --name-status -z`: a status, then one path, or two for a rename
// or a copy (where it was, where it is).
func parseNameStatus(fields []string) []Change {
	var changes []Change
	for index := 0; index < len(fields); {
		code := fields[index]
		index++
		if code == "" || index >= len(fields) {
			break
		}
		change := Change{Status: statusOf(code[0])}
		if change.Status == Renamed || change.Status == Copied {
			if index+1 >= len(fields) {
				break
			}
			change.OldPath, change.Path = fields[index], fields[index+1]
			index += 2
		} else {
			change.Path = fields[index]
			change.OldPath = change.Path
			index++
		}
		changes = append(changes, change)
	}
	return changes
}

func statusOf(letter byte) Status {
	switch letter {
	case 'A':
		return Added
	case 'D':
		return Deleted
	case 'R':
		return Renamed
	case 'C':
		return Copied
	case 'T':
		return TypeChanged
	}
	return Modified
}

// LineKind is what a line of a diff is.
type LineKind int

const (
	Context LineKind = iota // the same on both sides
	Insertion
	Removal
)

// DiffLine is one line of a file's diff, with its number on each side (0 where it is not on that
// side).
type DiffLine struct {
	Kind      LineKind
	OldNumber int
	NewNumber int
}

// FileDiff is the whole of a file's diff: every line of both sides, in order.
type FileDiff struct {
	Lines  []DiffLine
	Binary bool
}

// everyLine is a context large enough to show a whole file in one hunk, so the diff has every line
// of both sides and a reader can fold the unchanged ones itself.
const everyLine = "--unified=1000000"

// Diff compares one changed file between the base and the working tree or HEAD, keeping every line.
func Diff(repository string, comparison Comparison, change Change) (FileDiff, error) {
	arguments := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "-M", everyLine, comparison.MergeBase}
	if !comparison.WorkingTree {
		arguments = append(arguments, "HEAD")
	}
	arguments = append(arguments, "--", change.Path)
	if change.OldPath != change.Path {
		arguments = append(arguments, change.OldPath)
	}
	output, err := run(repository, arguments...)
	if err != nil {
		return FileDiff{}, err
	}
	return parseUnifiedDiff(output)
}

// parseUnifiedDiff reads git's unified diff of one file into numbered lines.
func parseUnifiedDiff(output []byte) (FileDiff, error) {
	var diff FileDiff
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	inHunk := false
	oldNumber, newNumber := 0, 0
	for scanner.Scan() {
		line := scanner.Text()
		if !inHunk {
			if strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch") {
				diff.Binary = true
				return diff, nil
			}
		}
		if strings.HasPrefix(line, "@@") {
			oldStart, newStart, err := parseHunkHeader(line)
			if err != nil {
				return diff, err
			}
			oldNumber, newNumber = oldStart, newStart
			inHunk = true
			continue
		}
		if !inHunk || line == "" {
			continue
		}
		switch line[0] {
		case ' ':
			diff.Lines = append(diff.Lines, DiffLine{Kind: Context, OldNumber: oldNumber, NewNumber: newNumber})
			oldNumber++
			newNumber++
		case '+':
			diff.Lines = append(diff.Lines, DiffLine{Kind: Insertion, NewNumber: newNumber})
			newNumber++
		case '-':
			diff.Lines = append(diff.Lines, DiffLine{Kind: Removal, OldNumber: oldNumber})
			oldNumber++
		}
	}
	return diff, scanner.Err()
}

// parseHunkHeader reads where a hunk starts on each side from "@@ -12,7 +12,9 @@".
func parseHunkHeader(header string) (oldStart, newStart int, err error) {
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return 0, 0, fmt.Errorf("unreadable hunk header %q", header)
	}
	oldStart, err = hunkStart(fields[1], "-")
	if err != nil {
		return 0, 0, err
	}
	newStart, err = hunkStart(fields[2], "+")
	return oldStart, newStart, err
}

func hunkStart(field, sign string) (int, error) {
	field = strings.TrimPrefix(field, sign)
	if comma := strings.IndexByte(field, ','); comma >= 0 {
		field = field[:comma]
	}
	start, err := strconv.Atoi(field)
	if err != nil {
		return 0, fmt.Errorf("unreadable hunk start %q", field)
	}
	if start == 0 {
		start = 1 // an empty side starts at 0; its first line, if any, is line 1
	}
	return start, nil
}
