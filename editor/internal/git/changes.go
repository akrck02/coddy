package git

import (
	"bufio"
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/akrck02/go-renderer/failure"
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
	// Head, when not empty, is the commit compared with the base instead of the disk or the last
	// commit: the head of a pull request, which is not checked out.
	Head string
}

// LastCommit is the revision of the last commit checked out.
const LastCommit = "HEAD"

// Last is the commit the base is compared with when the disk is not: Head, or the last commit.
func (comparison Comparison) Last() string {
	if comparison.Head != "" {
		return comparison.Head
	}
	return LastCommit
}

// OnDisk tells whether the base is compared with the files on disk.
func (comparison Comparison) OnDisk() bool { return comparison.WorkingTree && comparison.Head == "" }

// Changes lists the files a development changed since it left the base; git stops once stop is
// cancelled.
func Changes(stop context.Context, repository string, comparison Comparison) ([]Change, error) {
	arguments := []string{"diff", "--name-status", "-z", "-M", "--no-ext-diff", "--no-textconv", comparison.MergeBase}
	if !comparison.OnDisk() {
		arguments = append(arguments, comparison.Last())
	}
	output, err := runStoppable(stop, repository, append(arguments, "--")...)
	if err != nil {
		return nil, err
	}
	changes := parseNameStatus(splitNull(output))
	if !comparison.OnDisk() {
		return changes, nil
	}
	untracked, err := runStoppable(stop, repository, "ls-files", "-z", "--others", "--exclude-standard")
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
		change := Change{Status: StatusOf(code[0])}
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

// StatusOf is the status a letter of git's stands for ("A", "M", "D", "R"…); any other is Modified.
func StatusOf(letter byte) Status {
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

// Diff compares one changed file between the base and the working tree or the last commit (or
// Head), keeping every line.
func Diff(repository string, comparison Comparison, change Change) (FileDiff, error) {
	arguments := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "-M", everyLine, comparison.MergeBase}
	if !comparison.OnDisk() {
		arguments = append(arguments, comparison.Last())
	}
	arguments = append(arguments, "--", change.Path)
	if change.OldPath != change.Path {
		arguments = append(arguments, change.OldPath)
	}
	output, err := run(repository, arguments...)
	if err != nil {
		return FileDiff{}, err
	}
	return ParseUnifiedDiff(output)
}

// hunkReader numbers the lines of a diff's hunks as it reads them.
type hunkReader struct {
	diff                 FileDiff
	inHunk               bool
	oldNumber, newNumber int
}

// ParseUnifiedDiff reads a unified diff of one file, as git writes it, into numbered lines.
func ParseUnifiedDiff(output []byte) (FileDiff, error) {
	var reader hunkReader
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !reader.inHunk && (strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch")) {
			reader.diff.Binary = true
			return reader.diff, nil
		}
		if err := reader.read(line); err != nil {
			return reader.diff, err
		}
	}
	return reader.diff, scanner.Err()
}

// read takes one line of the diff: a hunk's header starts counting again, and a line inside a hunk
// is numbered on the sides it is on.
func (reader *hunkReader) read(line string) error {
	if strings.HasPrefix(line, "@@") {
		oldStart, newStart, err := parseHunkHeader(line)
		if err != nil {
			return err
		}
		reader.oldNumber, reader.newNumber = oldStart, newStart
		reader.inHunk = true
		return nil
	}
	if !reader.inHunk || line == "" {
		return nil
	}
	switch line[0] {
	case ' ':
		reader.diff.Lines = append(reader.diff.Lines, DiffLine{Kind: Context, OldNumber: reader.oldNumber, NewNumber: reader.newNumber})
		reader.oldNumber++
		reader.newNumber++
	case '+':
		reader.diff.Lines = append(reader.diff.Lines, DiffLine{Kind: Insertion, NewNumber: reader.newNumber})
		reader.newNumber++
	case '-':
		reader.diff.Lines = append(reader.diff.Lines, DiffLine{Kind: Removal, OldNumber: reader.oldNumber})
		reader.oldNumber++
	}
	return nil
}

// parseHunkHeader reads where a hunk starts on each side from "@@ -12,7 +12,9 @@".
func parseHunkHeader(header string) (oldStart, newStart int, err error) {
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return 0, 0, failure.Wrap(UnreadableDiff, "hunk header "+strconv.Quote(header), nil)
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
		return 0, failure.Wrap(UnreadableDiff, "hunk start "+strconv.Quote(field), err)
	}
	if start == 0 {
		start = 1 // an empty side starts at 0; its first line, if any, is line 1
	}
	return start, nil
}

// LineCounts is how many lines a change adds and removes; a binary file has none to count.
type LineCounts struct {
	Added, Removed int
	Binary         bool
}

// CountLines counts the lines each changed file adds and removes, by its path now. New files git
// does not know yet are not counted here: they are counted from their text. Git stops once stop is
// cancelled.
func CountLines(stop context.Context, repository string, comparison Comparison) (map[string]LineCounts, error) {
	arguments := []string{"diff", "--numstat", "-z", "-M", "--no-ext-diff", "--no-textconv", comparison.MergeBase}
	if !comparison.OnDisk() {
		arguments = append(arguments, comparison.Last())
	}
	output, err := runStoppable(stop, repository, append(arguments, "--")...)
	if err != nil {
		return nil, err
	}
	return parseNumStat(splitNull(output)), nil
}

// parseNumStat reads `git diff --numstat -z`: "added\tremoved\tpath", or for a rename
// "added\tremoved\t" followed by where it was and where it is, as fields of their own.
func parseNumStat(fields []string) map[string]LineCounts {
	counts := map[string]LineCounts{}
	for index := 0; index < len(fields); index++ {
		parts := strings.SplitN(fields[index], "\t", 3)
		if len(parts) < 3 {
			continue
		}
		path := parts[2]
		if path == "" { // a rename: the old path, then the new one
			if index+2 >= len(fields) {
				break
			}
			path = fields[index+2]
			index += 2
		}
		if parts[0] == "-" {
			counts[path] = LineCounts{Binary: true}
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		removed, _ := strconv.Atoi(parts[1])
		counts[path] = LineCounts{Added: added, Removed: removed}
	}
	return counts
}

// BlobsAt lists the content id of every file in a commit, by path: two files with the same id
// have the same text. Git stops once stop is cancelled.
func BlobsAt(stop context.Context, repository, revision string) (map[string]string, error) {
	output, err := runStoppable(stop, repository, "ls-tree", "-r", "-z", "--full-tree", "--end-of-options", revision)
	if err != nil {
		return nil, err
	}
	blobs := map[string]string{}
	for _, entry := range splitNull(output) {
		// "<mode> blob <id>\t<path>"
		meta, path, found := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if found && len(fields) == 3 {
			blobs[path] = fields[2]
		}
	}
	return blobs, nil
}
