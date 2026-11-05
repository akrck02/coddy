// Package git reads git repositories through the git command, and only reads them: every command
// it runs is on a short list of commands that never change a repository, and runs without the
// optional locks (the index refresh that `git status` and `git diff` would otherwise write back).
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readOnlyCommands are the only git commands this package runs. None of them writes to a
// repository in the forms used here: no fetch, no checkout, no commit, no hash-object -w.
var readOnlyCommands = map[string]bool{
	"cat-file":     true,
	"diff":         true,
	"grep":         true,
	"ls-files":     true,
	"ls-tree":      true,
	"merge-base":   true,
	"rev-list":     true,
	"rev-parse":    true,
	"symbolic-ref": true,
}

// ErrNotReadOnly is returned when code asks for a git command that is not known to be read-only.
var ErrNotReadOnly = errors.New("git command is not on the read-only list")

// safetyOptions keep git from writing or from starting other programs while it reads: no optional
// locks (so the index is not refreshed and written back), no file system monitor daemon, no
// external diff or text conversion programs, no pager and no colours.
var safetyOptions = []string{
	"--no-optional-locks",
	"--no-pager",
	"-c", "core.fsmonitor=false",
	"-c", "core.quotepath=off",
	"-c", "color.ui=false",
	"-c", "diff.external=",
}

// run runs a read-only git command in a repository and returns what it printed.
func run(repository string, arguments ...string) ([]byte, error) {
	if len(arguments) == 0 || !readOnlyCommands[arguments[0]] {
		return nil, ErrNotReadOnly
	}
	command := exec.Command("git", append(append([]string{}, safetyOptions...), arguments...)...)
	command.Dir = repository
	command.Env = append(os.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"LC_ALL=C",
	)
	var output, errorOutput bytes.Buffer
	command.Stdout = &output
	command.Stderr = &errorOutput
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(errorOutput.String())
		if message == "" {
			message = err.Error()
		}
		return output.Bytes(), fmt.Errorf("git %s: %s", arguments[0], message)
	}
	return output.Bytes(), nil
}

// runText is run with the output as a string, without its trailing new line.
func runText(repository string, arguments ...string) (string, error) {
	output, err := run(repository, arguments...)
	return strings.TrimSpace(string(output)), err
}
