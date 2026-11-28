package codebase

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// ChangedOnDisk is a file someone changed on disk after it was read: saving would lose that.
	ChangedOnDisk = failure.Define("codebase.changed-on-disk", "the file changed on disk since it was read")
	// NotSaved is a file that could not be written; the details hold why.
	NotSaved = failure.Define("codebase.not-saved", "the file could not be saved")
	// OutsideTheProject is a path that leads out of its project, as "../other" does.
	OutsideTheProject = failure.Define("codebase.outside-the-project", "the path leads outside its project")
	// BaseNotFound is a base asked for that the repository does not have: the default branch is
	// used instead.
	BaseNotFound = failure.Define("codebase.base-not-found", "the base asked for is not in the repository; the default branch is used")
	// NoBase is a repository with none of the usual base branches (main, master, develop).
	NoBase = failure.Define("codebase.no-base", "no base branch found (main, master, develop)")
	// DanglingLink is a file that is a link to nothing: saving would write over the link.
	DanglingLink = failure.Define("codebase.dangling-link", "the file is a link to a file that does not exist")
	// TooLarge is a file too large to show.
	TooLarge = failure.Define("codebase.too-large", "the file is too large to show")
	// RecordNotKept is the record of the files language servers wrote, which could not be written
	// in the reader's own folder; the details hold why.
	RecordNotKept = failure.Define("codebase.record-not-kept", "what the language servers wrote could not be kept")
)
