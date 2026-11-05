package window

import "github.com/akrck02/go-renderer/failure"

// The errors the window gives.
var (
	NoMode = failure.Define("window.no-mode", "the reader was made without a mode to show")
)
