package icons

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// NotAThemeFile is a file of icon themes that is not JSON in Zed's form, or is not there to
	// read; the details hold the file.
	NotAThemeFile = failure.Define("icons.not-a-theme-file", "the file is not an icon theme file of Zed's")
	// NoThemes is an icon theme file that lists no icon theme under "themes".
	NoThemes = failure.Define("icons.no-themes", `the file lists no icon theme under "themes"`)
	// NotDrawn is an icon's SVG that cannot be read; the file is drawn with another icon, and the
	// details hold the SVG's path.
	NotDrawn = failure.Define("icons.not-drawn", "an icon's picture cannot be read")
)
