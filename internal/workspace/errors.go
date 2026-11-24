package workspace

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// ChangedOnDisk is a file someone changed on disk after it was read: saving would lose that.
	ChangedOnDisk = failure.Define("workspace.changed-on-disk", "the file changed on disk since it was read")
	// NotSaved is a file that could not be written; the details hold why.
	NotSaved = failure.Define("workspace.not-saved", "the file could not be saved")
	// OutsideTheProject is a path that leads out of its project, as "../other" does.
	OutsideTheProject = failure.Define("workspace.outside-the-project", "the path leads outside its project")
	// BaseNotFound is a base asked for that the repository does not have: the default branch is
	// used instead.
	BaseNotFound = failure.Define("workspace.base-not-found", "the base asked for is not in the repository; the default branch is used")
	// NoBase is a repository with none of the usual base branches (main, master, develop).
	NoBase = failure.Define("workspace.no-base", "no base branch found (main, master, develop)")
	// TooLarge is a file too large to show.
	TooLarge = failure.Define("workspace.too-large", "the file is too large to show")
	// RecordNotKept is the record of the files language servers wrote, which could not be written
	// in the reader's own folder; the details hold why.
	RecordNotKept = failure.Define("workspace.record-not-kept", "what the language servers wrote could not be kept")
)
