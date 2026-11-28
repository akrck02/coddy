package review

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// MarksNotKept is returned when the files marked as viewed cannot be written to their file;
	// the marks stay while the reader runs.
	MarksNotKept = failure.Define("review.marks-not-kept", "the files marked as viewed could not be kept on disk")
)
