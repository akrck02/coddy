package shortcut

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// NotAChord is a text that does not name keys Coddy knows, as "Command+Shift+F" does; the
	// details hold the text.
	NotAChord = failure.Define("shortcut.not-a-chord", "the text does not name a combination of keys")
	// NotAProfile is a profile file that is not JSON in a profile's form; the details hold the file.
	NotAProfile = failure.Define("shortcut.not-a-profile", "the file is not a profile of keys")
	// NotSaved is a profile that could not be written, or removed; the details hold the file.
	NotSaved = failure.Define("shortcut.not-saved", "the profile of keys could not be saved")
	// BadName is a profile's name that is empty, Coddy's own, or not fit for a file's name.
	BadName = failure.Define("shortcut.bad-name", "a profile needs a name of its own, without slashes")
)
