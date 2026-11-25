package git

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
)

// Hit is a line of a file that holds the text searched for.
type Hit struct {
	Path   string
	Line   int    // from 1
	Column int    // where the first match starts in the line, in bytes from 0
	Text   string // the line
}

// GrepOptions say how a repository's files are searched.
type GrepOptions struct {
	// WorkingTree searches the files on disk, the new ones git does not know yet included;
	// otherwise the last commit is searched.
	WorkingTree bool
	// PerFile keeps at most this many lines of each file; 0 keeps them all.
	PerFile int
	// Word matches the text as a whole word only; Files, when set, are the only files searched,
	// as git pathspecs ("*.java").
	Word  bool
	Files []string
}

// Grep finds the lines of a repository's text files that hold a text, as written: ignoring case
// unless the text has a capital letter in it. It only reads, and stops when stop is cancelled.
// Finding nothing gives no hits and no error.
func Grep(stop context.Context, repository, text string, options GrepOptions) ([]Hit, error) {
	arguments := []string{"grep", "-n", "-z", "--column", "-I", "--no-color", "-F", "--no-textconv"}
	if !hasUpper(text) {
		arguments = append(arguments, "-i")
	}
	if options.PerFile > 0 {
		arguments = append(arguments, "--max-count", strconv.Itoa(options.PerFile))
	}
	if options.WorkingTree {
		arguments = append(arguments, "--untracked")
	}
	if options.Word {
		arguments = append(arguments, "-w")
	}
	arguments = append(arguments, "-e", text)
	if !options.WorkingTree {
		arguments = append(arguments, "HEAD")
	}
	output, err := runStoppable(stop, repository, append(append(arguments, "--"), options.Files...)...)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == nothingMatched {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseGrep(output, !options.WorkingTree), nil
}

// nothingMatched is the exit code git grep ends with when no line matches.
const nothingMatched = 1

// parseGrep reads `git grep -n -z --column`: "path\0line\0column\0text" per line, the path led by
// "HEAD:" when a commit was searched.
func parseGrep(output []byte, inCommit bool) []Hit {
	var hits []Hit
	for _, record := range bytes.Split(output, []byte{'\n'}) {
		fields := strings.SplitN(string(record), "\x00", 4)
		if len(fields) < 4 {
			continue
		}
		line, lineErr := strconv.Atoi(fields[1])
		column, columnErr := strconv.Atoi(fields[2])
		if lineErr != nil || columnErr != nil {
			continue
		}
		path := fields[0]
		if inCommit {
			path = strings.TrimPrefix(path, "HEAD:")
		}
		hits = append(hits, Hit{Path: path, Line: line, Column: max(column-1, 0), Text: fields[3]})
	}
	return hits
}

func hasUpper(text string) bool {
	for _, letter := range text {
		if unicode.IsUpper(letter) {
			return true
		}
	}
	return false
}
