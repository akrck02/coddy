package programs

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// NotFound is a program that is not installed: not on the PATH, nor where installers put them.
	NotFound = failure.Define("programs.not-found", "the program is not installed")
	// InstallFailed is an install command that could not run, or ended with an error.
	InstallFailed = failure.Define("programs.install-failed", "the program could not be installed")
)
