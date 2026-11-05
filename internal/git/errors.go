package git

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// CommandNotReadOnly is returned when code asks for a git command that is not known to be
	// read-only: the guarantee that the reader never changes a repository.
	CommandNotReadOnly = failure.Define("git.command-not-read-only", "the git command is not on the read-only list")
	// CommandFailed is git's own failure; the details hold the command and what git said.
	CommandFailed = failure.Define("git.command-failed", "git could not do what was asked")
	// UnreadableDiff is a diff whose hunk headers git wrote in a form this package does not know.
	UnreadableDiff = failure.Define("git.unreadable-diff", "the diff is not in a form this reader knows")
)
