package theme

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// NotAThemeFile is a file of the themes folder that is not JSON in Zed's form, or is not there
	// to read; the details hold the file.
	NotAThemeFile = failure.Define("theme.not-a-theme-file", "the file is not a theme file of Zed's")
	// NoThemes is a theme file that lists no theme under "themes".
	NoThemes = failure.Define("theme.no-themes", `the file lists no theme under "themes"`)
	// MissingColours is a theme without the colours a look is made from; the details hold its name.
	MissingColours = failure.Define("theme.missing-colours", `the theme needs at least "text" and "editor.background"`)
)
