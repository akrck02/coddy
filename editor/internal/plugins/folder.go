package plugins

import "os"

// MakeFolder makes the folder plugins are installed in, Coddy's own, when it is not there yet:
// before the first plugin is installed, the settings may still open it.
func MakeFolder(folder string) error { return os.MkdirAll(folder, 0o755) }
