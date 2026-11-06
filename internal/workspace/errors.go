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
)
